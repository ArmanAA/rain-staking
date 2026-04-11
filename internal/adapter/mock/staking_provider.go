package mock

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/ArmanAA/rain-staking/internal/port"
)

// StakingProvider implements port.StakingProvider with in-memory state.
// Simulates a staking provider for local development and testing.
type StakingProvider struct {
	mu     sync.RWMutex
	stakes map[string]*mockStake
	logger *slog.Logger
}

type mockStake struct {
	providerRef    string
	validator      string
	status         port.ProviderStakeStatus
	amount         decimal.Decimal
	totalRewards   decimal.Decimal
	createdAt      time.Time
}

func NewStakingProvider(logger *slog.Logger) *StakingProvider {
	return &StakingProvider{
		stakes: make(map[string]*mockStake),
		logger: logger,
	}
}

func (p *StakingProvider) Stake(ctx context.Context, req port.StakeRequest) (*port.StakeResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	providerRef := fmt.Sprintf("mock-%s", uuid.New().String()[:8])
	validator := fmt.Sprintf("0x%s", uuid.New().String()[:40])

	p.stakes[providerRef] = &mockStake{
		providerRef:  providerRef,
		validator:    validator,
		status:       port.ProviderStakeStatusActive, // Mock: instantly active
		amount:       req.Amount,
		totalRewards: decimal.Zero,
		createdAt:    time.Now().UTC(),
	}

	p.logger.InfoContext(ctx, "mock: stake created",
		slog.String("provider_ref", providerRef),
		slog.String("amount", req.Amount.String()),
	)

	return &port.StakeResponse{
		ProviderRef: providerRef,
		Validator:   validator,
	}, nil
}

func (p *StakingProvider) Unstake(ctx context.Context, providerRef string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	stake, ok := p.stakes[providerRef]
	if !ok {
		return fmt.Errorf("mock: stake not found: %s", providerRef)
	}

	stake.status = port.ProviderStakeStatusWithdrawn

	p.logger.InfoContext(ctx, "mock: stake unstaked", slog.String("provider_ref", providerRef))
	return nil
}

func (p *StakingProvider) GetStakeStatus(ctx context.Context, providerRef string) (*port.StakeStatusResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stake, ok := p.stakes[providerRef]
	if !ok {
		return nil, fmt.Errorf("mock: stake not found: %s", providerRef)
	}

	return &port.StakeStatusResponse{
		Status:    stake.status,
		Validator: stake.validator,
	}, nil
}

func (p *StakingProvider) GetRewards(ctx context.Context, providerRef string) ([]port.RewardEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stake, ok := p.stakes[providerRef]
	if !ok {
		return nil, fmt.Errorf("mock: stake not found: %s", providerRef)
	}

	if stake.status != port.ProviderStakeStatusActive {
		return nil, nil
	}

	// Simulate daily reward: ~0.01% of staked amount per poll
	reward := stake.amount.Mul(decimal.NewFromFloat(0.0001))
	stake.totalRewards = stake.totalRewards.Add(reward)

	return []port.RewardEntry{
		{
			Amount:     reward,
			RewardDate: time.Now().UTC().Format("2006-01-02"),
		},
	}, nil
}
