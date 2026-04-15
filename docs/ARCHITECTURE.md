# Architecture Diagram

## Functional Requirements

- **Stake:** Users can stake ETH via BitGo.
- **Unstake:** Users can unstake and withdraw funds.
- **Track Balances:** The system tracks Available, Staked, and Pending balances.
- **Reconcile State:** A background worker automatically syncs stake statuses with the provider.
- **Process Rewards:** The system automatically fetches and records daily staking rewards.

**Out of scope:**
- Swap staked assets for other cryptocurrencies directly.
- Set up "Auto-Staking" for all incoming deposits.

## Non-Functional Requirements

- **Consistency >> Availability (Mutations):** For financial actions (staking, unstaking, balance updates), the system strictly prioritizes absolute consistency.
- **Availability >> Consistency (Reads & Async Ops):** For read operations (checking balances, viewing rewards) and third-party integrations, we prioritize availability.
- **Scalability (Stateless API & Background Workers):** Optimized for high-throughput fintech needs.
- **Security (Defense-in-Depth):** Even though this is a downstream microservice where authentication is assumed to be handled by the platform gateway, we practice defense-in-depth.
- **Idempotency:** Every state-changing network call requires an idempotency key, guaranteeing that background retries or frontend glitches will never duplicate a financial transaction.

**Out of scope:**
- "Social Media" Scale: We handle robust fintech throughput, but we aren't optimized for 100 million concurrent requests.
- Regional Compliance: GDPR and local data residency laws.
- Multi-Region CI/CD: Fully automated multi-region deployment pipelines.

---

## System Diagram

```mermaid
graph TD
    %% ===== External Clients =====
    Client["Client / Frontend"]
    LB["Load Balancer"]

    Client -->|"HTTP / gRPC requests"| LB

    %% ===== API Layer =====
    subgraph API_LAYER["API Layer (Adapters - Driving Side)"]
        direction TB
        HTTP["HTTP/REST Gateway\n:8080"]
        GRPC["gRPC Server\n:9090"]
        HTTP -->|"Loopback\ntranslation"| GRPC
    end

    LB -->|"Route traffic"| HTTP
    LB -->|"Route traffic"| GRPC

    %% ===== Interceptor Chain =====
    subgraph INTERCEPTORS["Security Shield (Defense-in-Depth)"]
        direction LR
        Recovery["Recovery\n(Panic Handler)"]
        Logging["Logging\n(Request ID + Duration)"]
        Auth["JWT Auth\n(HS256 Token Validation)"]
        Validation["Validation\n(UUID, Decimals, Bounds)"]
        Recovery --> Logging --> Auth --> Validation
    end

    GRPC -->|"All requests\npass through"| Recovery

    %% ===== Core Domain =====
    subgraph CORE["Core Domain & Services (Hexagonal Center)"]
        direction TB
        StakingSvc["Staking Service\n(Orchestration)"]
        BalanceSvc["Balance Service\n(Queries + Creation)"]
        RewardSvc["Reward Service\n(Queries + Summaries)"]

        subgraph DOMAIN["Domain Layer (Pure Go - Zero Dependencies)"]
            direction LR
            StakeEntity["Stake\n(State Machine)"]
            BalanceEntity["Balance\n(Available/Staked/Pending)"]
            RewardEntity["Reward\n(Value Object)"]
            AuditEntity["Audit Event\n(Immutable)"]
        end

        StakingSvc --> StakeEntity
        StakingSvc --> BalanceEntity
        BalanceSvc --> BalanceEntity
        RewardSvc --> RewardEntity
    end

    Validation -->|"Stake / Unstake"| StakingSvc
    Validation -->|"Get / List Balances"| BalanceSvc
    Validation -->|"Reward Summary / History"| RewardSvc

    %% ===== Background Worker =====
    subgraph WORKER["Background Worker (Async)"]
        direction TB
        Poller["Reward Poller"]
        Cron(["Every N minutes\n(configurable interval)"])
        Cron -->|"triggers"| Poller
    end

    Poller -->|"1. Reconcile\nDELEGATING -> ACTIVE\nUNSTAKING -> WITHDRAWN"| StakeEntity
    Poller -->|"2. Fetch & record\ndaily rewards"| RewardEntity
    Poller -->|"3. Update\nbalance buckets"| BalanceEntity

    %% ===== Database =====
    subgraph DB["PostgreSQL (Adapters - Driven Side)"]
        direction TB
        Postgres[("PostgreSQL")]
        Tables["Tables:\n1. stakes (state machine + optimistic lock)\n2. balances (available/staked/pending + optimistic lock)\n3. rewards (append-only, unique per stake+date)\n4. audit_events (append-only, never UPDATE/DELETE)"]
        Postgres --- Tables
    end

    StakingSvc -->|"Read / Write\n(Optimistic Locking)"| Postgres
    BalanceSvc -->|"Read / Write\n(Optimistic Locking)"| Postgres
    RewardSvc -->|"Read"| Postgres
    Poller -->|"Update states\nAdd rewards\nAudit events"| Postgres

    %% ===== External Provider =====
    BitGo["BitGo\n(Custodial Staking API)\nHolesky Testnet"]

    StakingSvc -->|"Stake / Unstake\nrequests"| BitGo
    Poller -->|"Poll statuses\nFetch daily rewards"| BitGo

    %% ===== Styling =====
    classDef external fill:#f9d6d6,stroke:#d63031,stroke-width:2px,color:#2d3436
    classDef api fill:#dfe6e9,stroke:#636e72,stroke-width:2px,color:#2d3436
    classDef security fill:#ffeaa7,stroke:#fdcb6e,stroke-width:2px,color:#2d3436
    classDef core fill:#d4f5d4,stroke:#00b894,stroke-width:2px,color:#2d3436
    classDef domain fill:#c8f7c5,stroke:#27ae60,stroke-width:2px,color:#2d3436
    classDef worker fill:#e0d4f5,stroke:#6c5ce7,stroke-width:2px,color:#2d3436
    classDef database fill:#dfe6e9,stroke:#2d3436,stroke-width:2px,color:#2d3436
    classDef provider fill:#f9d6d6,stroke:#d63031,stroke-width:2px,color:#2d3436

    class Client,LB external
    class HTTP,GRPC api
    class Recovery,Logging,Auth,Validation security
    class StakingSvc,BalanceSvc,RewardSvc core
    class StakeEntity,BalanceEntity,RewardEntity,AuditEntity domain
    class Poller,Cron worker
    class Postgres,Tables database
    class BitGo provider
```
