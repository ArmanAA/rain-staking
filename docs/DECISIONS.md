# Architectural Decisions

This document explains the key technical decisions made for the Rain Staking Service and the reasoning behind each.

---

## 1. Hexagonal Architecture (Ports and Adapters)

**Decision:** Structure the service using hexagonal architecture with distinct domain, port, service, and adapter layers.

**Why:** In a financial system like staking, business logic must remain testable and independent of infrastructure. Hexagonal architecture enforces a strict dependency rule — all dependencies point inward toward the domain. This means we can swap PostgreSQL for another store, replace BitGo with Figment, or move from gRPC to pure HTTP without touching a single line of business logic. For a microservice that lives inside a larger platform, this isolation is critical for long-term maintainability.

**Trade-off:** Slightly more upfront boilerplate compared to a flat package structure. Worth it for a service handling financial operations where correctness and testability are non-negotiable.

---

## 2. gRPC with HTTP Gateway

**Decision:** Serve both gRPC (port 9090) and HTTP/REST (port 8080) from the same service using grpc-gateway.

**Why:** Rain already has a microservice platform. gRPC is the natural choice for internal service-to-service communication — it provides strong typing via protobuf, efficient binary serialization, and built-in code generation. However, frontend teams and external integrators expect REST APIs. grpc-gateway solves both needs from a single protobuf definition, eliminating the risk of API contract drift between the two.

**Trade-off:** Adds a build-time dependency on protobuf tooling (buf). However, this also gives us auto-generated OpenAPI specs, which eliminates the need for manual API documentation.

---

## 3. Domain-Driven State Machine for Stake Lifecycle

**Decision:** Model the stake lifecycle as an explicit state machine embedded in the domain entity, with transitions enforced at the domain level.

**Why:** A stake moves through well-defined states: PENDING -> DELEGATING -> ACTIVE -> UNSTAKING -> WITHDRAWN, with FAILED as a terminal state reachable from any non-terminal state. Encoding these transitions in the domain entity (rather than in service logic or database constraints) ensures that no code path can put a stake into an invalid state. This is especially important when multiple actors can trigger transitions — the customer initiates, BitGo confirms, and the reward poller updates.

**Trade-off:** Chose to implement transitions as methods on the entity rather than using a state machine library. The lifecycle is simple enough that a library would add unnecessary abstraction, and keeping it in plain Go makes the logic immediately readable during code review.

---

## 4. PostgreSQL with pgx and sqlc

**Decision:** Use pgx as the PostgreSQL driver and sqlc for compile-time type-safe query generation. No ORM.

**Why:**
- **pgx** is the highest-performance Go driver for PostgreSQL. In a financial service where every query touches money, we want the lowest overhead and most control over connection pooling and transactions.
- **sqlc** generates Go code from SQL queries at compile time, giving us type safety without the runtime overhead or magic of an ORM. Queries are plain SQL — reviewable, optimizable, and debuggable with standard database tools.
- **No ORM** because ORMs obscure what's actually hitting the database. In a staking service, we need to reason precisely about transactions, locking, and query performance.

**Trade-off:** Writing raw SQL requires more upfront effort than an ORM for simple CRUD. sqlc mitigates this by generating all the boilerplate while keeping queries explicit.

---

## 5. Optimistic Locking on Balances and Stakes

**Decision:** Use version-based optimistic locking for all balance and stake mutations.

**Why:** In a concurrent system, two requests could simultaneously try to stake from the same balance. Without locking, one could succeed silently with stale data, leading to an overdrawn balance. Optimistic locking (WHERE version = $expected) detects conflicts at write time and forces a retry, ensuring balance integrity without the performance cost of pessimistic locks (SELECT FOR UPDATE) that hold rows locked during external API calls to BitGo.

**Trade-off:** Optimistic locking can cause retries under high contention. For a staking service where a single customer is unlikely to submit multiple concurrent stake requests, contention is low and optimistic locking is the right fit.

---

## 6. Idempotency Keys on Mutations

**Decision:** All state-changing operations (CreateStake, Unstake) require an idempotency key.

**Why:** Network failures, client retries, and load balancer timeouts are inevitable. In a financial system, a duplicate stake request must not double-stake a customer's funds. Idempotency keys ensure that retrying the same request returns the original response without re-executing the operation. This is standard practice in payment systems (Stripe, Square) and is critical for any service handling real money.

**Trade-off:** Requires storing idempotency keys and checking them on every mutation. The storage cost is trivial compared to the correctness guarantee.

---

## 7. Strategy Pattern for Staking Providers

**Decision:** Abstract the staking provider behind a `StakingProvider` interface with BitGo and Mock implementations.

**Why:** Rain's requirements mention BitGo, Figment, BlockDaemon, and custom solutions as potential providers. Abstracting behind an interface means:
- **Local development** works without BitGo credentials (mock provider).
- **Testing** can use deterministic mock behavior.
- **Future providers** can be added by implementing the interface — zero changes to domain or service logic.
- **Provider migration** (e.g., BitGo -> Figment) becomes a configuration change, not a rewrite.

**Trade-off:** The interface must be general enough to accommodate different providers without being so abstract that it loses useful provider-specific features. The current interface is designed around the common denominator of staking operations.

---

## 8. Append-Only Audit Log

**Decision:** Record all state changes as immutable events in an audit_events table.

**Why:** Financial regulators and compliance teams expect a complete, tamper-proof history of every action taken on customer funds. An append-only event log provides:
- Full traceability of who did what and when.
- The ability to reconstruct the state of any entity at any point in time.
- A foundation for event-driven features (notifications, analytics) if needed later.

**Trade-off:** Storage grows over time. For a staking service, the event volume is low (stakes are infrequent, high-value operations), making this a non-concern.

---

## 9. Structured Logging with slog

**Decision:** Use Go's standard library `slog` package for structured logging.

**Why:** slog (introduced in Go 1.21) provides structured, leveled logging with zero external dependencies. In a microservice that will be deployed into Rain's existing platform, using stdlib reduces supply chain risk and ensures compatibility with whatever log aggregation system is in place (ELK, Datadog, CloudWatch). JSON output format integrates with all major observability stacks.

**Trade-off:** Slightly less feature-rich than zerolog or zap, but the performance difference is negligible for a staking service's throughput profile, and having no external dependency is a meaningful advantage.

---

## 10. golang-migrate for Schema Migrations

**Decision:** Use golang-migrate with sequential, versioned SQL migration files.

**Why:** Schema migrations in a financial system must be explicit, reviewable, and reversible. golang-migrate provides:
- Plain SQL up/down migrations (no DSL to learn).
- Version tracking in the database itself.
- CLI for manual migrations and library integration for programmatic use.
- Broad industry adoption, meaning Rain's team is likely already familiar with it.

**Trade-off:** Requires manual migration authoring. Declarative tools like Atlas auto-generate migrations but can produce unsafe changes (like dropping columns) if not carefully reviewed. For financial data, explicit control is preferred.

---

## 11. JWT Authentication & Authorization Model

**Decision:** Authenticate via JWT in a gRPC interceptor; authorize resource access at the handler level, returning NotFound instead of PermissionDenied on ownership violations.

**Why:**

**Auth in interceptor, authz in handler.** Authentication is a cross-cutting concern — every request needs it, and it doesn't require any business context. An interceptor is the right place. Authorization, however, requires business context. For example, to check whether a caller can access a stake, we first need to look up the stake to find its owner. That lookup happens in the handler, so the authorization check naturally belongs there too.

**NotFound over PermissionDenied.** When a user requests a resource they don't own, the API returns `NotFound` (gRPC code 5) instead of `PermissionDenied` (code 7). A 403/PermissionDenied leaks information — an attacker can enumerate valid resource IDs by checking which ones return 403 vs 404. Returning NotFound for both "doesn't exist" and "exists but not yours" makes them indistinguishable.

**JWT as a stand-in.** In production at Rain, authentication would be handled by an upstream identity service (OAuth2/OIDC), and the staking service would validate tokens issued by that service. For this project, a local JWT implementation demonstrates the same authentication/authorization architecture without requiring external infrastructure. The interceptor's interface doesn't change — swap in a different token validator and the rest of the system is unaffected.

**Trade-off:** JWT with a shared secret (HS256) is simpler than asymmetric keys (RS256/ES256) but means the secret must be distributed to every service that needs to validate tokens. In production, RS256 with a JWKS endpoint would be preferred. The architecture supports this — only the `ValidateToken` function changes.

---

## 12. HTTP Gateway Interceptor Chain

**Decision:** Use `RegisterStakingServiceHandlerFromEndpoint` (which routes HTTP requests through the gRPC server via loopback) instead of `RegisterStakingServiceHandlerServer` (which calls handler methods directly).

**Why:** `RegisterStakingServiceHandlerServer` is faster because it avoids the loopback hop, but it bypasses the gRPC interceptor chain entirely. This means HTTP requests would reach handlers **without authentication, validation, or logging**. This is a security vulnerability — the HTTP gateway is the public-facing API, and skipping auth on it would defeat the entire security model.

`RegisterStakingServiceHandlerFromEndpoint` ensures that all HTTP requests are translated into gRPC calls and pass through the full interceptor chain: Recovery -> Logging -> Auth -> Validation -> Handler. The loopback cost is negligible (localhost TCP, sub-millisecond) compared to the security guarantee.

**Trade-off:** Slightly higher latency per HTTP request due to the loopback hop. In practice, the added latency is under a millisecond — invisible compared to database queries and provider API calls. The security guarantee is worth far more than the performance cost.
