package port

import (
	"context"

	"github.com/ArmanAA/rain-staking/internal/domain"
)

// StakeRepository defines data access operations for staking positions.
type StakeRepository interface {
	Create(ctx context.Context, stake *domain.Stake) error
	GetByID(ctx context.Context, id string) (*domain.Stake, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Stake, error)
	ListByCustomerID(ctx context.Context, customerID string, state *domain.StakeState, limit, offset int) ([]*domain.Stake, error)
	Update(ctx context.Context, stake *domain.Stake) error // Uses optimistic locking via Version field
	ListByState(ctx context.Context, state domain.StakeState, limit int) ([]*domain.Stake, error)
}

// BalanceRepository defines data access operations for customer balances.
type BalanceRepository interface {
	GetByCustomerAndAsset(ctx context.Context, customerID, asset string) (*domain.Balance, error)
	ListByCustomerID(ctx context.Context, customerID string) ([]*domain.Balance, error)
	Upsert(ctx context.Context, balance *domain.Balance) error
	Update(ctx context.Context, balance *domain.Balance) error // Uses optimistic locking via Version field
}

// RewardRepository defines data access operations for staking rewards.
type RewardRepository interface {
	Create(ctx context.Context, reward *domain.Reward) error
	ListByStakeID(ctx context.Context, stakeID string, limit, offset int) ([]*domain.Reward, error)
	GetTotalByStakeID(ctx context.Context, stakeID string) (*RewardSummary, error)
}

// RewardSummary holds aggregated reward data for a stake.
type RewardSummary struct {
	TotalRewards string
	RewardCount  int32
	LastRewardAt *string
}

// AuditEventRepository defines data access for the append-only audit log.
type AuditEventRepository interface {
	Create(ctx context.Context, event *domain.AuditEvent) error
}
