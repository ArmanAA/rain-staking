# Rain Staking Microservice - Design Document

## Context

Rain is a cryptocurrency exchange building a staking feature for customers. This service allows customers to stake Ethereum via BitGo's custodial staking API, track balances, and monitor rewards over time. In a custodial model, BitGo holds the private keys on behalf of Rain's customers — Rain submits staking requests via API and BitGo handles validator selection and delegation.

**Key decisions:**
- **Provider:** BitGo (Rain's custodian partner) with Ethereum on Holesky testnet
- **API:** gRPC + HTTP gateway via grpc-gateway (single protobuf definition serves both)
- **Architecture:** Hexagonal / Ports-and-Adapters

---

## Requirements

### Functional

- Customer can **stake ETH** via BitGo's custodial staking API
- Customer can **unstake** and withdraw funds after the unbonding period
- System **tracks balances** per customer per asset with three buckets: available, staked, pending
- Background poller **reconciles stake state** with the provider (confirms delegation, detects withdrawal completion, handles failures)
- Background poller **fetches and records daily rewards** for active stakes
- All mutations are **idempotent** — safe to retry on network failure
- All state changes produce an **append-only audit trail**
- **9 API endpoints** served over both gRPC (:9090) and HTTP/REST (:8080)

### Non-Functional

- **Consistency** — optimistic locking (version field) prevents concurrent balance corruption without holding database locks during external API calls
- **Auditability** — immutable event log for compliance; every state change is recorded with actor, timestamp, and event data
- **Provider-agnostic** — `StakingProvider` interface allows swapping BitGo for Figment, BlockDaemon, or any other provider with zero domain changes
- **Testability** — domain layer has zero external dependencies; all infrastructure is behind interfaces; tests use in-memory fakes, not mocks
- **Security** — JWT authentication via interceptor, resource-level authorization in handlers, input validation interceptor, interceptor chain ensures HTTP gateway requests pass through auth

---

## End-to-End User Flow

### Staking

```
Customer                    API Gateway              Service                Provider (BitGo)
   │                            │                       │                       │
   │── CreateStake ────────────>│                       │                       │
   │                            │── Auth (JWT) ────────>│                       │
   │                            │── Validate fields ───>│                       │
   │                            │── Authorize owner ───>│                       │
   │                            │                       │── Check balance       │
   │                            │                       │── Hold funds          │
   │                            │                       │   (available→pending) │
   │                            │                       │── Stake ─────────────>│
   │                            │                       │<── providerRef ───────│
   │                            │                       │── Save stake (PENDING)│
   │                            │                       │── Publish audit event │
   │<── stake (PENDING) ───────│                       │                       │
   │                            │                       │                       │
   │         ... time passes (background poller runs) ...                      │
   │                            │                       │                       │
   │                   Poller ──│── GetStakeStatus ────────────────────────────>│
   │                            │<── status: ACTIVE ───────────────────────────│
   │                            │── Transition DELEGATING → ACTIVE             │
   │                            │── Move balance (pending → staked)             │
   │                            │                       │                       │
   │                   Poller ──│── GetRewards ────────────────────────────────>│
   │                            │<── reward entries ───────────────────────────│
   │                            │── Record reward (idempotent)                 │
   │                            │── Add reward to available balance            │
```

### Unstaking

```
Customer                    API Gateway              Service                Provider (BitGo)
   │                            │                       │                       │
   │── Unstake ────────────────>│                       │                       │
   │                            │── Auth + Validate ───>│                       │
   │                            │── Authorize owner ───>│                       │
   │                            │                       │── Unstake ───────────>│
   │                            │                       │── Transition ACTIVE → UNSTAKING
   │<── stake (UNSTAKING) ─────│                       │                       │
   │                            │                       │                       │
   │         ... unbonding period (background poller runs) ...                 │
   │                            │                       │                       │
   │                   Poller ──│── GetStakeStatus ────────────────────────────>│
   │                            │<── status: WITHDRAWN ────────────────────────│
   │                            │── Transition UNSTAKING → WITHDRAWN            │
   │                            │── Move balance (staked → available)           │
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        API Layer                             │
│   gRPC :9090              HTTP/REST :8080 (grpc-gateway)     │
│         └────────────────────┬───────────────────────────┘   │
│                              v                               │
│  ┌───────────────────────────────────────────────────────┐   │
│  │                  Interceptor Chain                     │   │
│  │   Recovery → Logging → Auth → Validation → Handler    │   │
│  └───────────────────────────┬───────────────────────────┘   │
│                              v                               │
│              ┌───────────────────────────────┐               │
│              │      Application Services     │               │
│              │  Staking · Balance · Reward    │               │
│              └───────────────┬───────────────┘               │
│                              v                               │
│              ┌───────────────────────────────┐               │
│              │     Domain Layer (pure Go)     │               │
│              │  Stake (state machine)         │               │
│              │  Balance · Reward · AuditEvent │               │
│              └───────────────┬───────────────┘               │
│                              v                               │
│              ┌───────────────────────────────┐               │
│              │           Ports               │               │
│              │  StakingProvider · Repos ·     │               │
│              │  EventPublisher               │               │
│              └───────────────┬───────────────┘               │
│                              v                               │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│   │  BitGo   │  │ Postgres │  │  Mock    │  │  Audit   │   │
│   │ Adapter  │  │  Repos   │  │ Provider │  │  Logger  │   │
│   └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└──────────────────────────────────────────────────────────────┘
```

Dependencies flow **inward**: adapters -> ports -> domain. The domain layer has zero external dependencies.

The HTTP gateway routes requests through the gRPC server via loopback (`RegisterStakingServiceHandlerFromEndpoint`), ensuring all HTTP requests pass through the full interceptor chain. The alternative (`RegisterStakingServiceHandlerServer`) was avoided because it bypasses interceptors entirely.

---

## Domain Model

### Stake Lifecycle (State Machine)

```
         ┌──────────┐
         │ PENDING   │  Customer requests stake
         └────┬──────┘
              │ provider accepts
              ▼
         ┌──────────┐
         │DELEGATING │  Tx submitted to chain
         └────┬──────┘
              │ confirmed on-chain
              ▼
         ┌──────────┐
         │  ACTIVE   │  Earning rewards
         └────┬──────┘
              │ customer requests unstake
              ▼
         ┌──────────┐
         │UNSTAKING  │  Unbonding period
         └────┬──────┘
              │ unbonding complete
              ▼
         ┌──────────┐
         │WITHDRAWN  │  Funds returned (terminal)
         └──────────┘

    Any non-terminal state → FAILED (on unrecoverable error)
```

State transitions are enforced in the domain entity — invalid transitions return a domain error. This is critical because multiple actors trigger transitions: the customer initiates, BitGo confirms, and the background poller reconciles.

### Balance Tracking

Each customer has a per-asset balance with three buckets:
- **Available** — liquid funds, can be staked or withdrawn
- **Staked** — locked in active stakes, earning rewards
- **Pending** — in transit between available and staked (during PENDING/DELEGATING states)

### Data Model

Four tables — see [`migrations/000001_initial_schema.up.sql`](../migrations/000001_initial_schema.up.sql) for full schema:

| Table | Purpose | Key Constraints |
|-------|---------|----------------|
| `balances` | Per-customer per-asset balance tracking | `CHECK (available >= 0 AND staked >= 0 AND pending >= 0)`, optimistic locking via `version` |
| `stakes` | Staking positions with lifecycle state | `CHECK (state IN (...))`, `UNIQUE(idempotency_key)`, optimistic locking via `version` |
| `rewards` | Append-only reward history | `UNIQUE(stake_id, reward_date)` for idempotent ingestion |
| `audit_events` | Immutable audit log | Append-only, never UPDATE/DELETE |

---

## Security

### Interceptor Chain

```
Request → Recovery → Logging → Auth → Validation → Handler
```

1. **Recovery** — catches panics, returns `codes.Internal`
2. **Logging** — generates request ID, logs method/status/duration. Runs before auth so failed auth attempts are always recorded.
3. **Authentication** — validates JWT (HS256) from `authorization` metadata, extracts `customer_id` into context
4. **Validation** — checks UUID formats, required fields, decimal amounts, page size bounds (max 100)

### Authorization

Handled at the **handler level** (not interceptor) because it requires business context — e.g., looking up a stake to check ownership.

Returns `NotFound` on ownership violations (not `PermissionDenied`) to prevent resource enumeration.

---

## Key Design Patterns

| Pattern | Where | Why |
|---------|-------|-----|
| **Hexagonal Architecture** | Overall structure | Isolates business logic from infrastructure |
| **Strategy Pattern** | `StakingProvider` interface | Swap BitGo/Mock/Figment without touching domain |
| **State Machine** | `Stake` entity | Enforces valid lifecycle transitions at domain level |
| **Repository Pattern** | `port/repository.go` | Abstracts data access, enables testing |
| **Optimistic Locking** | Balance + Stake updates | Prevents lost updates without expensive DB locks |
| **Idempotency** | Stake/Unstake operations | Safe retries for financial operations |
| **Domain Events** | Audit log | Append-only trail for compliance (swappable for Kafka/SQS) |
| **Dependency Injection** | `main.go` wiring | All deps injected, no global state |

---

## Tech Stack

| Concern | Choice | Justification |
|---------|--------|---------------|
| Language | Go 1.25+ | Required by project |
| API | gRPC + grpc-gateway | Service mesh ready + REST for frontends |
| Database | PostgreSQL + pgx | Best perf for Go+Postgres; type-safe |
| Query gen | sqlc | Compile-time type-safe SQL, zero runtime cost |
| Migrations | golang-migrate | Industry standard, supports rollbacks |
| Logging | slog (stdlib) | No deps, structured, production-grade |
| Config | envconfig | Lightweight, 12-factor app compliant |
| Auth | golang-jwt/jwt/v5 | JWT-based authentication via gRPC interceptor |
| Testing | testify + in-memory fakes | Assertions + hexagonal arch enables pure unit tests |
| Proto mgmt | buf | Modern protobuf toolchain, lint + breaking change detection |
| CI | GitHub Actions | Lint, test, build on every push |

---

## Approach

The implementation followed a domain-first approach: define the domain model and state machine with comprehensive tests, then build outward through ports, services, and adapters. Each layer was tested at its boundary using in-memory fakes for dependencies, ensuring the domain remained pure and infrastructure swappable. The API surface — 9 RPCs defined in [`proto/staking/v1/staking.proto`](../proto/staking/v1/staking.proto) — was designed from the protobuf definition first, generating both gRPC and HTTP/REST handlers from the same source of truth.

See also: [Architectural Decisions](DECISIONS.md) for trade-off rationale, [Testing Strategy](TESTING.md) for coverage analysis.
