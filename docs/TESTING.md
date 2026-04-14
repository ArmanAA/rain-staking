# Testing Strategy

## Philosophy

1. **Test business logic thoroughly.** The domain and service layers contain the logic that protects customer funds — state transitions, balance operations, orchestration. These get the most coverage.
2. **Use fakes, not mocks.** In-memory implementations of repository and provider interfaces allow tests to exercise real business logic without infrastructure dependencies. Fakes are simpler to write, easier to reason about, and less brittle than mock-based tests that assert on call sequences.
3. **Hexagonal architecture enables isolation.** Because the domain has zero external dependencies and services depend only on port interfaces, every layer can be tested in isolation with pure Go — no database, no network, no containers.

---

## Test Inventory

**8 test files, 41 test functions.**

| File | Tests | What It Covers |
|------|-------|----------------|
| `internal/domain/stake_test.go` | 5 | State machine transitions: all valid paths (Delegate, Activate, RequestUnstake, Withdraw, Fail) and invalid transitions |
| `internal/domain/balance_test.go` | 6 | Balance operations: Hold, ConfirmStake, ReleaseHold, CompleteUnstake, AddReward, edge cases (insufficient funds, negative amounts) |
| `internal/auth/jwt_test.go` | 6 | Token generation, validation, expiry, wrong secret, tampered tokens, missing claims |
| `internal/service/staking_service_test.go` | 2 | End-to-end staking orchestration: CreateStake (balance check + provider call + persistence), Unstake (state transition + provider call) |
| `internal/adapter/grpc/interceptors_test.go` | 6 | Auth interceptor: valid token, missing token, invalid format, expired token, wrong secret, missing metadata |
| `internal/adapter/grpc/validation_test.go` | 7 | Request validation: UUID format, required fields, decimal amounts, page size bounds, all 9 request types |
| `internal/adapter/grpc/authorize_test.go` | 3 | Authorization: matching customer (pass), mismatched customer (NotFound), missing auth context (Unauthenticated) |
| `internal/worker/reward_poller_test.go` | 6 | DELEGATING->ACTIVE reconciliation, UNSTAKING->WITHDRAWN reconciliation, DELEGATING->FAILED with balance release, reward fetching and balance updates, idempotent reward deduplication, balance updates on reward |

---

## Patterns

### Table-Driven Tests

Most test files use table-driven tests with subtests — the idiomatic Go pattern for testing multiple scenarios with the same setup:

```go
tests := []struct {
    name    string
    // inputs and expected outputs
}{...}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test logic
    })
}
```

### testify Conventions

- **`require`** for setup assertions — if setup fails, the test stops immediately (no point continuing with bad preconditions).
- **`assert`** for verification — checks all assertions and reports all failures, giving a complete picture of what's wrong.

### In-Memory Fakes

Repository and provider fakes live alongside their test files (in the test itself or in the `mock` package). They implement the port interfaces with simple in-memory maps, providing:
- Deterministic behavior (no network, no race conditions with external systems)
- Fast execution (no database roundtrips)
- Full control over error injection (return errors on demand to test error paths)

---

## Coverage by Layer

### Domain Layer (comprehensive)

The domain layer is the most thoroughly tested because it contains the core business rules:
- **Stake state machine:** Every valid transition is tested. Invalid transitions (e.g., PENDING -> WITHDRAWN, ACTIVE -> DELEGATING) are tested to ensure they return errors.
- **Balance operations:** Hold, confirm, release, unstake completion, reward addition. Edge cases like insufficient funds, zero amounts, and negative amounts are covered.

### Service Layer (orchestration paths)

Service tests verify that the orchestration logic works end-to-end:
- CreateStake: validates amount, checks balance, calls provider, persists stake, updates balance, publishes event.
- Unstake: validates state, calls provider, transitions state, publishes event.
- Uses in-memory fakes for all repositories and the staking provider.

### gRPC Adapter Layer (interceptors + authorization)

- **Auth interceptor:** All 6 tests cover token validation — valid tokens, missing tokens, invalid format, expired tokens, wrong secret, and missing metadata. Recovery and logging interceptors are exercised implicitly but not unit-tested directly.
- **Validation:** All 9 request types are validated. Tests cover valid requests, missing required fields, invalid UUIDs, and invalid decimal amounts.
- **Authorization:** Tests the three cases — matching owner (pass), mismatched owner (NotFound), and missing auth context (Unauthenticated).

### Worker Layer (reconciliation + rewards)

The reward poller tests verify all three polling responsibilities:
- DELEGATING stakes transition to ACTIVE when the provider confirms, with balance moving from pending to staked.
- DELEGATING stakes transition to FAILED when the provider reports failure, with balance hold released.
- UNSTAKING stakes transition to WITHDRAWN when unbonding completes, with balance moving from staked to available.
- Rewards are fetched, recorded (with cumulative tracking), and added to available balance.

---

## What's Not Tested (and Why)

| Area | Why Not | What Would Be Needed |
|------|---------|---------------------|
| **Postgres repositories** | These are thin data-access adapters with sqlc-generated queries. Testing them requires a real PostgreSQL instance. | Integration tests with testcontainers-go spinning up a real Postgres. |
| **BitGo HTTP client** | Requires either a real BitGo testnet account or an HTTP mock server. | HTTP test server with recorded BitGo responses. |
| **gRPC handler methods** | Handlers are thin adapters that delegate to services. The logic they contain (proto mapping, error translation) is simple and deterministic. | Handler-level tests with a real gRPC server and test client. Not high-value given that services and interceptors are already tested. |
| **main.go wiring** | Dependency injection wiring is verified by the binary building and running. | End-to-end integration test that starts the server and makes real API calls. |

### Production Additions

For a production deployment, the following tests would be added:
- **Integration tests** with testcontainers-go for Postgres repositories (query correctness, constraint enforcement, optimistic locking behavior).
- **BitGo client tests** with an HTTP mock server replaying recorded responses.
- **End-to-end tests** that start the full server and exercise the API through gRPC and HTTP.
- **Load tests** for the reward poller under high stake counts.

---

## How to Run

```bash
# Unit tests (fast, no external deps)
make test
# Equivalent to: go test ./... -short -count=1

# With race detector (as CI runs)
go test ./... -short -count=1 -race

# Coverage report (generates coverage.html)
make test-coverage
```

### CI Pipeline

The GitHub Actions CI pipeline runs on every push and pull request to `main`:

1. **Lint** — `golangci-lint run ./...` with errcheck, govet, staticcheck, unused, ineffassign, gosimple
2. **Test** — `go test ./... -short -count=1 -race` (race detector enabled, caching disabled)
3. **Build** — compiles the binary and builds the Docker image

The `-count=1` flag disables test caching to ensure tests always run fresh. The `-race` flag enables Go's race detector to catch data races in concurrent code (particularly important for the reward poller).
