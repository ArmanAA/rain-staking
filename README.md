# Rain Staking Service

A microservice that enables customers to stake Ethereum through BitGo's custodial staking API, track balances, and monitor rewards over time. Designed as a standalone service within Rain's existing microservice platform.

## Scope

**In scope:**
- Customer balance tracking (available / staked / pending)
- Staking and unstaking via BitGo's Staking API (Holesky testnet)
- Stake lifecycle state machine (PENDING -> DELEGATING -> ACTIVE -> UNSTAKING -> WITHDRAWN)
- Reward tracking and history over time via background polling
- RESTful HTTP API and gRPC API from a single protobuf definition
- Idempotent mutation endpoints for safe retries
- Audit trail of all state changes

**Out of scope (assumed to exist in Rain's platform):**
- Crypto deposits and withdrawals
- Frontend UI
- KYC/compliance checks
- Validator node operation (handled by BitGo)

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        API Layer                             │
│   gRPC :9090              HTTP/REST :8080 (grpc-gateway)     │
│         └────────────────────┬───────────────────────────┘   │
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

Dependencies flow inward. The domain layer has zero external dependencies.

## Tech Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Language | Go 1.22+ | Required |
| API | gRPC + grpc-gateway | Type-safe internal comms + REST for frontends |
| Database | PostgreSQL (pgx + sqlc) | Best Go performance, compile-time type safety |
| Migrations | golang-migrate | Industry standard, reversible |
| Protobuf | buf | Modern toolchain with lint and breaking change detection |
| Logging | slog (stdlib) | Zero deps, structured, production-grade |
| Testing | testify | Readable assertions, table-driven tests |
| CI | GitHub Actions | Lint, test, build on every push |

## Getting Started

### Prerequisites
- Docker and Docker Compose
- Go 1.22+ (for local development)

### Quick Start (Docker)
```bash
# Start everything: Postgres + migrations + app
docker compose up -d

# Generate a JWT token for testing
make token
# Copy the output token

# Verify it's running (replace <token> with the output above)
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/v1/customers/550e8400-e29b-41d4-a716-446655440000/balances
```

### Local Development
```bash
# Start only Postgres
docker compose up -d postgres

# Run migrations
export DATABASE_URL="postgres://rain:rain@localhost:5432/rain_staking?sslmode=disable"
make migrate-up

# Run with mock provider
STAKING_PROVIDER=mock DATABASE_URL="$DATABASE_URL" make run

# Run tests
make test
```

### Using BitGo Testnet
```bash
export STAKING_PROVIDER=bitgo
export BITGO_BASE_URL=https://app.bitgo-test.com
export BITGO_ACCESS_TOKEN=your-token
export BITGO_WALLET_ID=your-wallet-id
export BITGO_COIN=hteth
```

## Authentication & Security

All API endpoints require a JWT Bearer token. The service implements a layered security model:

### Generating a Token

```bash
# Default: generates token for test customer with dev secret
make token

# Custom customer ID
go run cmd/tokengen/main.go --customer-id <uuid>

# Custom secret and expiry
go run cmd/tokengen/main.go --secret my-secret --expiry 72h
```

### Using the Token

Include the token as a Bearer token in the `Authorization` header:

```bash
TOKEN=$(make token)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/customers/.../balances
```

### Security Architecture

| Layer | What | How |
|-------|------|-----|
| **Authentication** | Verify caller identity | JWT validation via gRPC interceptor. Extracts `customer_id` from token claims and stores in request context. |
| **Authorization** | Verify resource ownership | Handler-level checks ensure callers can only access their own stakes, balances, and rewards. |
| **Input Validation** | Reject malformed requests | gRPC interceptor validates UUID formats, required fields, decimal amounts, and page size bounds before handlers run. |
| **Audit Trail** | Record all mutations | Append-only `audit_events` table logs every state change with actor, timestamp, and event data. |
| **Data Integrity** | Prevent corruption | Optimistic locking (version field), CHECK constraints on balances, UNIQUE idempotency keys. |

### Security Design Decisions

- **NotFound over PermissionDenied:** When a user requests a resource they don't own, the API returns `404 Not Found` instead of `403 Forbidden`. This prevents attackers from enumerating valid resource IDs by probing with different tokens.
- **Interceptor chain ordering:** Recovery (catches panics) -> Logging (records all attempts, including failed auth) -> Authentication (rejects unauthenticated) -> Validation (rejects malformed). This ensures failed auth attempts are always logged.
- **Gateway routing through gRPC:** The HTTP gateway dials back to the gRPC server over loopback (`RegisterStakingServiceHandlerFromEndpoint`), ensuring all HTTP requests pass through the full interceptor chain including auth. The alternative (`RegisterStakingServiceHandlerServer`) bypasses interceptors entirely.
- **Auth in interceptor, authz in handler:** Authentication is cross-cutting (every request needs it), so it belongs in an interceptor. Authorization requires business context (looking up stake ownership), so it belongs in the handler where that data is available.

## API Examples

All examples below assume `TOKEN` is set:
```bash
export TOKEN=$(make token)
```

### Create a balance (seed a customer)
```bash
curl -X POST http://localhost:8080/v1/customers/550e8400-e29b-41d4-a716-446655440000/balances \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"asset":"ETH","available":"100.0"}'
```

### Stake 32 ETH
```bash
curl -X POST http://localhost:8080/v1/stakes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "550e8400-e29b-41d4-a716-446655440000",
    "asset": "ETH",
    "amount": "32",
    "idempotency_key": "stake-001"
  }'
```

### Check balance
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/customers/550e8400-e29b-41d4-a716-446655440000/balances/ETH
```

### View reward history
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/stakes/{stake_id}/rewards/history
```

### gRPC (with grpcurl)
```bash
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{"customer_id":"550e8400-e29b-41d4-a716-446655440000"}' \
  localhost:9090 staking.v1.StakingService/ListStakes
```

## Postman Collection

Import `postman/rain-staking.postman_collection.json` into Postman for a pre-built collection with all endpoints and example requests.

## Project Structure

```
├── cmd/
│   ├── stakingd/              # Application entry point, DI wiring
│   └── tokengen/              # JWT token generator for testing
├── internal/
│   ├── domain/                # Pure business logic, no external deps
│   │   ├── stake.go           # Stake entity with state machine
│   │   ├── balance.go         # Balance entity with hold/release ops
│   │   ├── reward.go          # Reward value object
│   │   ├── event.go           # Audit event
│   │   └── errors.go          # Domain error types
│   ├── auth/                  # JWT authentication utilities
│   ├── port/                  # Interfaces (contracts between layers)
│   │   ├── repository.go      # Data access interfaces
│   │   ├── staking_provider.go # Third-party provider abstraction
│   │   └── event_publisher.go # Event publishing interface
│   ├── service/               # Application orchestration layer
│   │   ├── staking_service.go # Stake/unstake orchestration
│   │   ├── balance_service.go # Balance queries
│   │   └── reward_service.go  # Reward queries
│   ├── adapter/
│   │   ├── grpc/              # gRPC handlers + interceptors
│   │   ├── postgres/          # Repository implementations
│   │   ├── bitgo/             # BitGo staking provider
│   │   └── mock/              # Mock provider for local dev
│   └── worker/
│       └── reward_poller.go   # Background reward sync + state reconciliation
├── proto/staking/v1/          # Protobuf service definition
├── gen/                       # Generated code (proto + sqlc)
├── migrations/                # SQL schema migrations
├── queries/                   # sqlc query definitions
├── postman/                   # Postman collection
├── .github/workflows/         # CI pipeline
├── Dockerfile                 # Multi-stage build
└── docker-compose.yml         # One-command local setup
```

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Hexagonal architecture | Domain logic stays pure and testable, infrastructure is swappable |
| State machine in domain entity | Invalid state transitions are impossible at the type level |
| Optimistic locking (version field) | Prevents concurrent balance corruption without expensive DB locks |
| Idempotency keys on mutations | Safe retries for financial operations |
| Strategy pattern for providers | Swap BitGo/Mock/Figment with zero domain changes |
| Append-only audit log | Complete traceability for compliance |
| sqlc over ORM | Compile-time type safety, no runtime magic, reviewable SQL |
| slog over third-party loggers | Zero dependencies, stdlib support, sufficient for this workload |
| JWT auth with interceptor | Cross-cutting authentication separated from business logic |
| Authorization in handlers | Resource ownership checks require business context (stake lookups) |
| NotFound for ownership violations | Prevents resource enumeration via 403 vs 404 probing |

## Scaling & Production Considerations

For a production deployment at scale, the following would be added:

- **Event streaming (Kafka/SQS):** Replace the log-based EventPublisher with a Kafka producer. The `EventPublisher` interface already supports this — swap the adapter, zero domain changes.
- **Leader election for reward poller:** Use `pg_advisory_lock` or a distributed lock to ensure only one instance polls at a time when running multiple replicas.
- **Read replicas:** Route read-heavy queries (balance lookups, reward history) to PostgreSQL read replicas.
- **Connection pooling:** Use PgBouncer in front of PostgreSQL for connection management at scale.
- **Observability:** Add OpenTelemetry tracing, Prometheus metrics, and integrate with Datadog/Grafana.
- **Rate limiting:** Add per-customer rate limiting on mutation endpoints.
- **Circuit breaker:** Wrap BitGo client calls with a circuit breaker to handle provider outages gracefully.

## Testing

```bash
# Unit tests (fast, no external deps)
make test

# With coverage report
make test-coverage
```

Tests cover:
- Domain state machine transitions (all valid + invalid paths)
- Balance operations (hold, confirm, release, edge cases)
- Staking service orchestration with mocked dependencies
- Idempotency behavior
