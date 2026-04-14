package service

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ArmanAA/rain-staking/internal/domain"
	"github.com/ArmanAA/rain-staking/internal/port"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

// --- In-memory fakes for testing ---

type fakeStakeRepo struct {
	stakes map[string]*domain.Stake
	byKey  map[string]*domain.Stake
}

func newFakeStakeRepo() *fakeStakeRepo {
	return &fakeStakeRepo{
		stakes: make(map[string]*domain.Stake),
		byKey:  make(map[string]*domain.Stake),
	}
}

func (r *fakeStakeRepo) Create(_ context.Context, s *domain.Stake) error {
	r.stakes[s.ID] = s
	if s.IdempotencyKey != "" {
		r.byKey[s.IdempotencyKey] = s
	}
	return nil
}

func (r *fakeStakeRepo) GetByID(_ context.Context, id string) (*domain.Stake, error) {
	s, ok := r.stakes[id]
	if !ok {
		return nil, domain.ErrStakeNotFound
	}
	return s, nil
}

func (r *fakeStakeRepo) GetByIdempotencyKey(_ context.Context, key string) (*domain.Stake, error) {
	return r.byKey[key], nil
}

func (r *fakeStakeRepo) ListByCustomerID(_ context.Context, customerID string, state *domain.StakeState, limit, offset int) ([]*domain.Stake, error) {
	var result []*domain.Stake
	for _, s := range r.stakes {
		if s.CustomerID == customerID {
			if state == nil || s.State == *state {
				result = append(result, s)
			}
		}
	}
	return result, nil
}

func (r *fakeStakeRepo) Update(_ context.Context, s *domain.Stake) error {
	r.stakes[s.ID] = s
	return nil
}

func (r *fakeStakeRepo) ListByState(_ context.Context, state domain.StakeState, limit int) ([]*domain.Stake, error) {
	var result []*domain.Stake
	for _, s := range r.stakes {
		if s.State == state {
			result = append(result, s)
		}
	}
	return result, nil
}

type fakeBalanceRepo struct {
	balances map[string]*domain.Balance // key: customerID+asset
}

func newFakeBalanceRepo() *fakeBalanceRepo {
	return &fakeBalanceRepo{balances: make(map[string]*domain.Balance)}
}

func (r *fakeBalanceRepo) GetByCustomerAndAsset(_ context.Context, customerID, asset string) (*domain.Balance, error) {
	b, ok := r.balances[customerID+asset]
	if !ok {
		return nil, domain.ErrBalanceNotFound
	}
	return b, nil
}

func (r *fakeBalanceRepo) ListByCustomerID(_ context.Context, customerID string) ([]*domain.Balance, error) {
	var result []*domain.Balance
	for _, b := range r.balances {
		if b.CustomerID == customerID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (r *fakeBalanceRepo) Upsert(_ context.Context, b *domain.Balance) error {
	r.balances[b.CustomerID+b.Asset] = b
	return nil
}

func (r *fakeBalanceRepo) Update(_ context.Context, b *domain.Balance) error {
	r.balances[b.CustomerID+b.Asset] = b
	return nil
}

type fakeProvider struct{}

func (p *fakeProvider) Stake(_ context.Context, req port.StakeRequest) (*port.StakeResponse, error) {
	return &port.StakeResponse{
		ProviderRef: "provider-ref-123",
		Validator:   "0xvalidator",
	}, nil
}

func (p *fakeProvider) Unstake(_ context.Context, providerRef string) error { return nil }

func (p *fakeProvider) GetStakeStatus(_ context.Context, providerRef string) (*port.StakeStatusResponse, error) {
	return &port.StakeStatusResponse{Status: port.ProviderStakeStatusActive, Validator: "0xvalidator"}, nil
}

func (p *fakeProvider) GetRewards(_ context.Context, providerRef string) ([]port.RewardEntry, error) {
	return nil, nil
}

type fakeEventPublisher struct {
	events []*domain.AuditEvent
}

func (p *fakeEventPublisher) Publish(_ context.Context, e *domain.AuditEvent) error {
	p.events = append(p.events, e)
	return nil
}

// --- Tests ---

func TestCreateStake(t *testing.T) {
	t.Run("successful stake creation", func(t *testing.T) {
		stakeRepo := newFakeStakeRepo()
		balanceRepo := newFakeBalanceRepo()
		publisher := &fakeEventPublisher{}

		// Set up customer with 100 ETH available
		balance := domain.NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(100.0)
		balanceRepo.Upsert(context.Background(), balance)

		svc := NewStakingService(stakeRepo, balanceRepo, &fakeProvider{}, publisher, testLogger)

		stake, err := svc.CreateStake(context.Background(), CreateStakeRequest{
			CustomerID:     "cust-1",
			Asset:          "ETH",
			Amount:         "32",
			IdempotencyKey: "idem-1",
		})

		require.NoError(t, err)
		assert.Equal(t, domain.StakeStateDelegating, stake.State)
		assert.Equal(t, "provider-ref-123", stake.ProviderRef)
		assert.Equal(t, "0xvalidator", stake.Validator)

		// Verify balance was updated
		updatedBalance, _ := balanceRepo.GetByCustomerAndAsset(context.Background(), "cust-1", "ETH")
		assert.True(t, updatedBalance.Available.Equal(decimal.NewFromFloat(68.0)))
		assert.True(t, updatedBalance.Pending.Equal(decimal.NewFromFloat(32.0)))

		// Verify audit event was published
		assert.Len(t, publisher.events, 1)
		assert.Equal(t, "stake.created", publisher.events[0].EventType)
	})

	t.Run("insufficient balance", func(t *testing.T) {
		stakeRepo := newFakeStakeRepo()
		balanceRepo := newFakeBalanceRepo()

		balance := domain.NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(10.0)
		balanceRepo.Upsert(context.Background(), balance)

		svc := NewStakingService(stakeRepo, balanceRepo, &fakeProvider{}, &fakeEventPublisher{}, testLogger)

		_, err := svc.CreateStake(context.Background(), CreateStakeRequest{
			CustomerID:     "cust-1",
			Asset:          "ETH",
			Amount:         "32",
			IdempotencyKey: "idem-2",
		})

		assert.ErrorIs(t, err, domain.ErrInsufficientBalance)
	})

	t.Run("idempotent duplicate returns existing stake", func(t *testing.T) {
		stakeRepo := newFakeStakeRepo()
		balanceRepo := newFakeBalanceRepo()

		balance := domain.NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(100.0)
		balanceRepo.Upsert(context.Background(), balance)

		svc := NewStakingService(stakeRepo, balanceRepo, &fakeProvider{}, &fakeEventPublisher{}, testLogger)

		// First call
		stake1, err := svc.CreateStake(context.Background(), CreateStakeRequest{
			CustomerID:     "cust-1",
			Asset:          "ETH",
			Amount:         "32",
			IdempotencyKey: "idem-same",
		})
		require.NoError(t, err)

		// Second call with same key — should return same stake
		stake2, err := svc.CreateStake(context.Background(), CreateStakeRequest{
			CustomerID:     "cust-1",
			Asset:          "ETH",
			Amount:         "32",
			IdempotencyKey: "idem-same",
		})
		require.NoError(t, err)
		assert.Equal(t, stake1.ID, stake2.ID)
	})

	t.Run("invalid amount", func(t *testing.T) {
		svc := NewStakingService(newFakeStakeRepo(), newFakeBalanceRepo(), &fakeProvider{}, &fakeEventPublisher{}, testLogger)

		_, err := svc.CreateStake(context.Background(), CreateStakeRequest{
			CustomerID:     "cust-1",
			Asset:          "ETH",
			Amount:         "-5",
			IdempotencyKey: "idem-3",
		})

		assert.ErrorIs(t, err, domain.ErrInvalidAmount)
	})
}

func TestUnstake(t *testing.T) {
	t.Run("successful unstake", func(t *testing.T) {
		stakeRepo := newFakeStakeRepo()
		balanceRepo := newFakeBalanceRepo()
		publisher := &fakeEventPublisher{}

		// Set up an active stake
		stake, _ := domain.NewStake("cust-1", "ETH", decimal.NewFromFloat(32.0), "idem-1")
		stake.Delegate("0xvalidator", "provider-ref-123")
		stake.Activate()
		stakeRepo.Create(context.Background(), stake)

		balance := domain.NewBalance("cust-1", "ETH")
		balance.Staked = decimal.NewFromFloat(32.0)
		balanceRepo.Upsert(context.Background(), balance)

		svc := NewStakingService(stakeRepo, balanceRepo, &fakeProvider{}, publisher, testLogger)

		result, err := svc.Unstake(context.Background(), stake.ID, "idem-unstake-1")

		require.NoError(t, err)
		assert.Equal(t, domain.StakeStateUnstaking, result.State)
		assert.Len(t, publisher.events, 1)
		assert.Equal(t, "stake.unstake_requested", publisher.events[0].EventType)
	})

	t.Run("cannot unstake non-active stake", func(t *testing.T) {
		stakeRepo := newFakeStakeRepo()

		stake, _ := domain.NewStake("cust-1", "ETH", decimal.NewFromFloat(32.0), "idem-1")
		stakeRepo.Create(context.Background(), stake) // Still PENDING

		svc := NewStakingService(stakeRepo, newFakeBalanceRepo(), &fakeProvider{}, &fakeEventPublisher{}, testLogger)

		_, err := svc.Unstake(context.Background(), stake.ID, "idem-unstake-2")

		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})
}
