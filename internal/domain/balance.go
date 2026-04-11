package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Balance tracks a customer's asset balance across three states:
// Available (liquid), Staked (locked in active stakes), and Pending (in transit).
type Balance struct {
	ID         string
	CustomerID string
	Asset      string
	Available  decimal.Decimal
	Staked     decimal.Decimal
	Pending    decimal.Decimal
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewBalance creates a new balance with zero values.
func NewBalance(customerID, asset string) *Balance {
	now := time.Now().UTC()
	return &Balance{
		ID:         uuid.New().String(),
		CustomerID: customerID,
		Asset:      asset,
		Available:  decimal.Zero,
		Staked:     decimal.Zero,
		Pending:    decimal.Zero,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// Hold moves funds from Available to Pending when a stake is initiated.
func (b *Balance) Hold(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}
	if b.Available.LessThan(amount) {
		return ErrInsufficientBalance
	}
	b.Available = b.Available.Sub(amount)
	b.Pending = b.Pending.Add(amount)
	b.UpdatedAt = time.Now().UTC()
	return nil
}

// ConfirmStake moves funds from Pending to Staked when a stake is confirmed on-chain.
func (b *Balance) ConfirmStake(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}
	if b.Pending.LessThan(amount) {
		return ErrInsufficientBalance
	}
	b.Pending = b.Pending.Sub(amount)
	b.Staked = b.Staked.Add(amount)
	b.UpdatedAt = time.Now().UTC()
	return nil
}

// ReleaseHold returns funds from Pending back to Available when a stake fails.
func (b *Balance) ReleaseHold(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}
	if b.Pending.LessThan(amount) {
		return ErrInsufficientBalance
	}
	b.Pending = b.Pending.Sub(amount)
	b.Available = b.Available.Add(amount)
	b.UpdatedAt = time.Now().UTC()
	return nil
}

// CompleteUnstake moves funds from Staked back to Available when unstaking is complete.
func (b *Balance) CompleteUnstake(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}
	if b.Staked.LessThan(amount) {
		return ErrInsufficientBalance
	}
	b.Staked = b.Staked.Sub(amount)
	b.Available = b.Available.Add(amount)
	b.UpdatedAt = time.Now().UTC()
	return nil
}

// AddReward adds staking rewards to the Available balance.
func (b *Balance) AddReward(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}
	b.Available = b.Available.Add(amount)
	b.UpdatedAt = time.Now().UTC()
	return nil
}
