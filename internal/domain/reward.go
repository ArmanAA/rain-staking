package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Reward represents a single staking reward accrual. Rewards are immutable
// once created — they form an append-only audit trail.
type Reward struct {
	ID               string
	StakeID          string
	CustomerID       string
	Asset            string
	Amount           decimal.Decimal
	CumulativeAmount decimal.Decimal
	RewardDate       time.Time
	CreatedAt        time.Time
}

// NewReward creates a new reward entry.
func NewReward(stakeID, customerID, asset string, amount, cumulativeAmount decimal.Decimal, rewardDate time.Time) (*Reward, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidAmount
	}

	return &Reward{
		ID:               uuid.New().String(),
		StakeID:          stakeID,
		CustomerID:       customerID,
		Asset:            asset,
		Amount:           amount,
		CumulativeAmount: cumulativeAmount,
		RewardDate:       rewardDate,
		CreatedAt:        time.Now().UTC(),
	}, nil
}
