-- Customer balances: one row per customer per asset.
-- Tracks available (liquid), staked (locked), and pending (in-transit) amounts.
CREATE TABLE balances (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL,
    asset       TEXT NOT NULL,
    available   NUMERIC(36, 18) NOT NULL DEFAULT 0,
    staked      NUMERIC(36, 18) NOT NULL DEFAULT 0,
    pending     NUMERIC(36, 18) NOT NULL DEFAULT 0,
    version     BIGINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(customer_id, asset),
    CONSTRAINT positive_balances CHECK (available >= 0 AND staked >= 0 AND pending >= 0)
);

-- Staking positions with lifecycle state machine.
CREATE TABLE stakes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id      UUID NOT NULL,
    asset            TEXT NOT NULL,
    amount           NUMERIC(36, 18) NOT NULL,
    state            TEXT NOT NULL DEFAULT 'PENDING',
    provider_ref     TEXT,
    validator        TEXT,
    idempotency_key  TEXT UNIQUE,
    failure_reason   TEXT,
    version          BIGINT NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_state CHECK (
        state IN ('PENDING', 'DELEGATING', 'ACTIVE', 'UNSTAKING', 'WITHDRAWN', 'FAILED')
    ),
    CONSTRAINT positive_amount CHECK (amount > 0)
);

CREATE INDEX idx_stakes_customer ON stakes(customer_id);
CREATE INDEX idx_stakes_state ON stakes(state);

-- Reward history: append-only ledger of staking rewards.
CREATE TABLE rewards (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stake_id          UUID NOT NULL REFERENCES stakes(id),
    customer_id       UUID NOT NULL,
    asset             TEXT NOT NULL,
    amount            NUMERIC(36, 18) NOT NULL,
    cumulative_amount NUMERIC(36, 18) NOT NULL,
    reward_date       DATE NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(stake_id, reward_date)
);

CREATE INDEX idx_rewards_customer ON rewards(customer_id);
CREATE INDEX idx_rewards_stake ON rewards(stake_id);

-- Audit log: append-only, immutable record of all state changes.
CREATE TABLE audit_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT NOT NULL,
    aggregate_id   UUID NOT NULL,
    event_type     TEXT NOT NULL,
    actor_id       UUID,
    data           JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_aggregate ON audit_events(aggregate_type, aggregate_id);
