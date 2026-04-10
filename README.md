# Rain Staking Service

A microservice that enables customers to stake Ethereum through BitGo's custodial staking API, track balances, and monitor rewards over time.

## Architecture

- **Hexagonal / Ports-and-Adapters** architecture with clean domain separation
- **gRPC + HTTP/REST** gateway for flexible API consumption
- **PostgreSQL** for persistence with optimistic locking
- **BitGo** integration for on-chain Ethereum staking (Holesky testnet)

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.22+ |
| API | gRPC + grpc-gateway |
| Database | PostgreSQL (pgx + sqlc) |
| Migrations | golang-migrate |
| Protobuf | buf |
| Containerization | Docker + Docker Compose |

## Getting Started

```bash
# Start all services
docker compose up -d

# Run tests
make test

# Run with mock staking provider (no BitGo credentials needed)
STAKING_PROVIDER=mock make run
```

## Project Structure

```
├── cmd/stakingd/          # Application entry point
├── internal/
│   ├── domain/            # Business logic (pure Go, no deps)
│   ├── port/              # Interfaces (driven + driving)
│   ├── service/           # Application / use-case layer
│   ├── adapter/
│   │   ├── grpc/          # gRPC handlers
│   │   ├── postgres/      # Repository implementations
│   │   ├── bitgo/         # BitGo staking provider
│   │   └── mock/          # Mock staking provider
│   └── worker/            # Background jobs (reward polling)
├── proto/                 # Protobuf definitions
├── migrations/            # SQL migrations
└── queries/               # sqlc query definitions
```

## License

Private - Rain Interview Project
