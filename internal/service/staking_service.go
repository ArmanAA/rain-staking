package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shopspring/decimal"

	"github.com/ArmanAA/rain-staking/internal/domain"
	"github.com/ArmanAA/rain-staking/internal/port"
)

// CreateStakeRequest holds the input for creating a new stake.
type CreateStakeRequest struct {
	CustomerID     string
	Asset          string
	Amount         string // Decimal string
	IdempotencyKey string
}

// StakingService orchestrates staking operations across domain, provider, and persistence.
type StakingService struct {
	stakeRepo   port.StakeRepository
	balanceRepo port.BalanceRepository
	provider    port.StakingProvider
	publisher   port.EventPublisher
	logger      *slog.Logger
}

func NewStakingService(
	stakeRepo port.StakeRepository,
	balanceRepo port.BalanceRepository,
	provider port.StakingProvider,
	publisher port.EventPublisher,
	logger *slog.Logger,
) *StakingService {
	return &StakingService{
		stakeRepo:   stakeRepo,
		balanceRepo: balanceRepo,
		provider:    provider,
		publisher:   publisher,
		logger:      logger,
	}
}

// CreateStake initiates a new staking position: validates balance, calls provider, persists state.
func (s *StakingService) CreateStake(ctx context.Context, req CreateStakeRequest) (*domain.Stake, error) {
	// Idempotency check: if we've seen this key before, return the existing stake
	if req.IdempotencyKey != "" {
		existing, err := s.stakeRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("checking idempotency: %w", err)
		}
		if existing != nil {
			return existing, nil
		}
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	// Create the domain entity (validates amount > 0)
	stake, err := domain.NewStake(req.CustomerID, req.Asset, amount, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}

	// Check and hold balance
	balance, err := s.balanceRepo.GetByCustomerAndAsset(ctx, req.CustomerID, req.Asset)
	if err != nil {
		return nil, fmt.Errorf("fetching balance: %w", err)
	}

	if err := balance.Hold(amount); err != nil {
		return nil, err
	}

	balance.Version++
	if err := s.balanceRepo.Update(ctx, balance); err != nil {
		return nil, fmt.Errorf("updating balance: %w", err)
	}

	// Persist the stake in PENDING state
	if err := s.stakeRepo.Create(ctx, stake); err != nil {
		return nil, fmt.Errorf("creating stake: %w", err)
	}

	// Call the staking provider
	providerResp, err := s.provider.Stake(ctx, port.StakeRequest{
		Amount:    amount,
		Asset:     req.Asset,
		ClientRef: stake.ID,
	})
	if err != nil {
		// Provider failed — mark stake as failed and release the hold
		stake.Fail(err.Error())
		stake.Version++
		if updateErr := s.stakeRepo.Update(ctx, stake); updateErr != nil {
			s.logger.ErrorContext(ctx, "failed to mark stake as failed",
				slog.String("stake_id", stake.ID), slog.String("error", updateErr.Error()))
		}

		balance.ReleaseHold(amount)
		balance.Version++
		if updateErr := s.balanceRepo.Update(ctx, balance); updateErr != nil {
			s.logger.ErrorContext(ctx, "failed to release balance hold after provider failure",
				slog.String("stake_id", stake.ID), slog.String("error", updateErr.Error()))
		}

		return nil, fmt.Errorf("provider stake failed: %w", err)
	}

	// Provider accepted — transition to DELEGATING
	if err := stake.Delegate(providerResp.Validator, providerResp.ProviderRef); err != nil {
		return nil, fmt.Errorf("transitioning stake: %w", err)
	}

	stake.Version++
	if err := s.stakeRepo.Update(ctx, stake); err != nil {
		return nil, fmt.Errorf("updating stake: %w", err)
	}

	// Publish audit event
	s.publisher.Publish(ctx, domain.NewAuditEvent(
		"stake", stake.ID, "stake.created", req.CustomerID,
		map[string]any{
			"amount":       req.Amount,
			"asset":        req.Asset,
			"provider_ref": providerResp.ProviderRef,
		},
	))

	return stake, nil
}

// Unstake initiates the unstaking process for an active stake.
func (s *StakingService) Unstake(ctx context.Context, stakeID, idempotencyKey string) (*domain.Stake, error) {
	stake, err := s.stakeRepo.GetByID(ctx, stakeID)
	if err != nil {
		return nil, err
	}

	if err := stake.Unstake(); err != nil {
		return nil, err
	}

	// Call provider to initiate unstaking
	if err := s.provider.Unstake(ctx, stake.ProviderRef); err != nil {
		return nil, fmt.Errorf("provider unstake failed: %w", err)
	}

	stake.Version++
	if err := s.stakeRepo.Update(ctx, stake); err != nil {
		return nil, fmt.Errorf("updating stake: %w", err)
	}

	s.publisher.Publish(ctx, domain.NewAuditEvent(
		"stake", stake.ID, "stake.unstake_requested", stake.CustomerID,
		map[string]any{"stake_id": stakeID},
	))

	return stake, nil
}

// GetStake retrieves a stake by ID.
func (s *StakingService) GetStake(ctx context.Context, stakeID string) (*domain.Stake, error) {
	return s.stakeRepo.GetByID(ctx, stakeID)
}

// ListStakes retrieves all stakes for a customer, optionally filtered by state.
func (s *StakingService) ListStakes(ctx context.Context, customerID string, state *domain.StakeState, limit, offset int) ([]*domain.Stake, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.stakeRepo.ListByCustomerID(ctx, customerID, state, limit, offset)
}
