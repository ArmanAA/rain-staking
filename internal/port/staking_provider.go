package port

import (
	"context"

	"github.com/shopspring/decimal"
)

// StakingProvider abstracts the third-party staking infrastructure (BitGo, Figment, etc.).
// Implementations handle all provider-specific API calls and response mapping.
type StakingProvider interface {
	// Stake submits a staking request to the provider and returns a provider reference ID
	// and the assigned validator address.
	Stake(ctx context.Context, req StakeRequest) (*StakeResponse, error)

	// Unstake submits an unstaking request to the provider.
	Unstake(ctx context.Context, providerRef string) error

	// GetStakeStatus checks the current status of a staking position with the provider.
	GetStakeStatus(ctx context.Context, providerRef string) (*StakeStatusResponse, error)

	// GetRewards fetches any new rewards for a staking position since the last check.
	GetRewards(ctx context.Context, providerRef string) ([]RewardEntry, error)
}

// StakeRequest contains the information needed to submit a stake to a provider.
type StakeRequest struct {
	Amount    decimal.Decimal
	Asset     string
	ClientRef string // Our internal stake ID, used for provider-side idempotency
}

// StakeResponse contains the provider's response after accepting a stake request.
type StakeResponse struct {
	ProviderRef string // Provider-assigned reference ID (e.g., BitGo request ID)
	Validator   string // Assigned validator address
}

// ProviderStakeStatus represents the provider's view of a stake's status.
type ProviderStakeStatus string

const (
	ProviderStakeStatusPending   ProviderStakeStatus = "PENDING"
	ProviderStakeStatusActive    ProviderStakeStatus = "ACTIVE"
	ProviderStakeStatusUnstaking ProviderStakeStatus = "UNSTAKING"
	ProviderStakeStatusWithdrawn ProviderStakeStatus = "WITHDRAWN"
	ProviderStakeStatusFailed    ProviderStakeStatus = "FAILED"
)

// StakeStatusResponse contains the provider's current view of a stake.
type StakeStatusResponse struct {
	Status    ProviderStakeStatus
	Validator string
}

// RewardEntry represents a single reward data point from the provider.
type RewardEntry struct {
	Amount     decimal.Decimal
	RewardDate string // ISO date "2026-04-11"
}
