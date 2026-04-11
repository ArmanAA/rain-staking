package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// StakeState represents the lifecycle state of a staking position.
type StakeState string

const (
	StakeStatePending    StakeState = "PENDING"
	StakeStateDelegating StakeState = "DELEGATING"
	StakeStateActive     StakeState = "ACTIVE"
	StakeStateUnstaking  StakeState = "UNSTAKING"
	StakeStateWithdrawn  StakeState = "WITHDRAWN"
	StakeStateFailed     StakeState = "FAILED"
)

// Stake represents a customer's staking position with a strict lifecycle state machine.
type Stake struct {
	ID             string
	CustomerID     string
	Asset          string
	Amount         decimal.Decimal
	State          StakeState
	ProviderRef    string
	Validator      string
	IdempotencyKey string
	FailureReason  string
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewStake creates a new staking position in the PENDING state.
func NewStake(customerID, asset string, amount decimal.Decimal, idempotencyKey string) (*Stake, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidAmount
	}

	now := time.Now().UTC()
	return &Stake{
		ID:             uuid.New().String(),
		CustomerID:     customerID,
		Asset:          asset,
		Amount:         amount,
		State:          StakeStatePending,
		IdempotencyKey: idempotencyKey,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Delegate transitions the stake from PENDING to DELEGATING.
// Called when the staking provider accepts the request and submits the on-chain transaction.
func (s *Stake) Delegate(validator, providerRef string) error {
	if s.State != StakeStatePending {
		return ErrInvalidStateTransition
	}
	s.State = StakeStateDelegating
	s.Validator = validator
	s.ProviderRef = providerRef
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// Activate transitions the stake from DELEGATING to ACTIVE.
// Called when the on-chain staking transaction is confirmed.
func (s *Stake) Activate() error {
	if s.State != StakeStateDelegating {
		return ErrInvalidStateTransition
	}
	s.State = StakeStateActive
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// Unstake transitions the stake from ACTIVE to UNSTAKING.
// Called when the customer requests to unstake their position.
func (s *Stake) Unstake() error {
	if s.State != StakeStateActive {
		return ErrInvalidStateTransition
	}
	s.State = StakeStateUnstaking
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// Withdraw transitions the stake from UNSTAKING to WITHDRAWN.
// Called when the unbonding period is complete and funds are liquid.
func (s *Stake) Withdraw() error {
	if s.State != StakeStateUnstaking {
		return ErrInvalidStateTransition
	}
	s.State = StakeStateWithdrawn
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// Fail transitions the stake to FAILED from any non-terminal state.
func (s *Stake) Fail(reason string) error {
	if s.IsTerminal() {
		return ErrInvalidStateTransition
	}
	s.State = StakeStateFailed
	s.FailureReason = reason
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// IsTerminal returns true if the stake is in a final state (WITHDRAWN or FAILED).
func (s *Stake) IsTerminal() bool {
	return s.State == StakeStateWithdrawn || s.State == StakeStateFailed
}
