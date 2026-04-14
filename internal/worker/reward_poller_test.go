package worker

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ArmanAA/rain-staking/internal/domain"
	"github.com/ArmanAA/rain-staking/internal/port"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

// --- In-memory fakes ---

type fakeStakeRepo struct {
	stakes map[string]*domain.Stake
}

func newFakeStakeRepo() *fakeStakeRepo {
	return &fakeStakeRepo{stakes: make(map[string]*domain.Stake)}
}

func (r *fakeStakeRepo) Create(_ context.Context, s *domain.Stake) error {
	r.stakes[s.ID] = s
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
	return nil, nil
}

func (r *fakeStakeRepo) ListByCustomerID(_ context.Context, customerID string, state *domain.StakeState, limit, offset int) ([]*domain.Stake, error) {
	return nil, nil
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
	balances map[string]*domain.Balance
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
	return nil, nil
}

func (r *fakeBalanceRepo) Upsert(_ context.Context, b *domain.Balance) error {
	r.balances[b.CustomerID+b.Asset] = b
	return nil
}

func (r *fakeBalanceRepo) Update(_ context.Context, b *domain.Balance) error {
	r.balances[b.CustomerID+b.Asset] = b
	return nil
}

type fakeRewardRepo struct {
	rewards map[string][]*domain.Reward // keyed by stake_id
}

func newFakeRewardRepo() *fakeRewardRepo {
	return &fakeRewardRepo{rewards: make(map[string][]*domain.Reward)}
}

func (r *fakeRewardRepo) Create(_ context.Context, reward *domain.Reward) error {
	// Check for duplicate (stake_id + reward_date)
	for _, existing := range r.rewards[reward.StakeID] {
		if existing.RewardDate.Equal(reward.RewardDate) {
			return domain.ErrDuplicateIdempotency
		}
	}
	r.rewards[reward.StakeID] = append(r.rewards[reward.StakeID], reward)
	return nil
}

func (r *fakeRewardRepo) ListByStakeID(_ context.Context, stakeID string, limit, offset int) ([]*domain.Reward, error) {
	return r.rewards[stakeID], nil
}

func (r *fakeRewardRepo) GetTotalByStakeID(_ context.Context, stakeID string) (*port.RewardSummary, error) {
	rewards := r.rewards[stakeID]
	total := decimal.Zero
	for _, rw := range rewards {
		total = total.Add(rw.Amount)
	}
	return &port.RewardSummary{
		TotalRewards: total.String(),
		RewardCount:  int32(len(rewards)),
	}, nil
}

type fakeProvider struct {
	stakeStatuses map[string]port.ProviderStakeStatus
	rewards       map[string][]port.RewardEntry
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{
		stakeStatuses: make(map[string]port.ProviderStakeStatus),
		rewards:       make(map[string][]port.RewardEntry),
	}
}

func (p *fakeProvider) Stake(_ context.Context, req port.StakeRequest) (*port.StakeResponse, error) {
	return nil, nil
}

func (p *fakeProvider) Unstake(_ context.Context, providerRef string) error { return nil }

func (p *fakeProvider) GetStakeStatus(_ context.Context, providerRef string) (*port.StakeStatusResponse, error) {
	status, ok := p.stakeStatuses[providerRef]
	if !ok {
		status = port.ProviderStakeStatusActive
	}
	return &port.StakeStatusResponse{Status: status, Validator: "0xvalidator"}, nil
}

func (p *fakeProvider) GetRewards(_ context.Context, providerRef string) ([]port.RewardEntry, error) {
	return p.rewards[providerRef], nil
}

type fakeEventPublisher struct {
	events []*domain.AuditEvent
}

func (p *fakeEventPublisher) Publish(_ context.Context, e *domain.AuditEvent) error {
	p.events = append(p.events, e)
	return nil
}

// --- Tests ---

func TestReconcileDelegating_ActivatesStake(t *testing.T) {
	stakeRepo := newFakeStakeRepo()
	balanceRepo := newFakeBalanceRepo()
	rewardRepo := newFakeRewardRepo()
	provider := newFakeProvider()
	publisher := &fakeEventPublisher{}

	// Create a DELEGATING stake
	stake, _ := domain.NewStake("cust-1", "ETH", decimal.NewFromFloat(32), "idem-1")
	_ = stake.Delegate("0xval", "prov-ref-1")
	_ = stakeRepo.Create(context.Background(), stake)

	// Create balance with 32 in pending
	balance := domain.NewBalance("cust-1", "ETH")
	balance.Pending = decimal.NewFromFloat(32)
	_ = balanceRepo.Upsert(context.Background(), balance)

	// Provider says stake is active
	provider.stakeStatuses["prov-ref-1"] = port.ProviderStakeStatusActive

	poller := NewRewardPoller(stakeRepo, balanceRepo, rewardRepo, provider, publisher, time.Minute, testLogger)
	poller.reconcileDelegating(context.Background())

	// Stake should be ACTIVE
	updated := stakeRepo.stakes[stake.ID]
	assert.Equal(t, domain.StakeStateActive, updated.State)

	// Balance: pending should move to staked
	bal, _ := balanceRepo.GetByCustomerAndAsset(context.Background(), "cust-1", "ETH")
	assert.True(t, bal.Pending.Equal(decimal.Zero))
	assert.True(t, bal.Staked.Equal(decimal.NewFromFloat(32)))

	// Audit event published
	assert.Len(t, publisher.events, 1)
	assert.Equal(t, "stake.activated", publisher.events[0].EventType)
}

func TestReconcileDelegating_FailedStake(t *testing.T) {
	stakeRepo := newFakeStakeRepo()
	balanceRepo := newFakeBalanceRepo()
	provider := newFakeProvider()
	publisher := &fakeEventPublisher{}

	stake, _ := domain.NewStake("cust-1", "ETH", decimal.NewFromFloat(32), "idem-1")
	_ = stake.Delegate("0xval", "prov-ref-1")
	_ = stakeRepo.Create(context.Background(), stake)

	balance := domain.NewBalance("cust-1", "ETH")
	balance.Pending = decimal.NewFromFloat(32)
	_ = balanceRepo.Upsert(context.Background(), balance)

	// Provider says stake failed
	provider.stakeStatuses["prov-ref-1"] = port.ProviderStakeStatusFailed

	poller := NewRewardPoller(stakeRepo, balanceRepo, newFakeRewardRepo(), provider, publisher, time.Minute, testLogger)
	poller.reconcileDelegating(context.Background())

	// Stake should be FAILED
	updated := stakeRepo.stakes[stake.ID]
	assert.Equal(t, domain.StakeStateFailed, updated.State)

	// Balance: pending should return to available
	bal, _ := balanceRepo.GetByCustomerAndAsset(context.Background(), "cust-1", "ETH")
	assert.True(t, bal.Pending.Equal(decimal.Zero))
	assert.True(t, bal.Available.Equal(decimal.NewFromFloat(32)))

	assert.Len(t, publisher.events, 1)
	assert.Equal(t, "stake.failed", publisher.events[0].EventType)
}

func TestReconcileUnstaking_WithdrawsStake(t *testing.T) {
	stakeRepo := newFakeStakeRepo()
	balanceRepo := newFakeBalanceRepo()
	provider := newFakeProvider()
	publisher := &fakeEventPublisher{}

	// Create an UNSTAKING stake
	stake, _ := domain.NewStake("cust-1", "ETH", decimal.NewFromFloat(32), "idem-1")
	_ = stake.Delegate("0xval", "prov-ref-1")
	_ = stake.Activate()
	_ = stake.Unstake()
	_ = stakeRepo.Create(context.Background(), stake)

	balance := domain.NewBalance("cust-1", "ETH")
	balance.Staked = decimal.NewFromFloat(32)
	_ = balanceRepo.Upsert(context.Background(), balance)

	// Provider says withdrawn
	provider.stakeStatuses["prov-ref-1"] = port.ProviderStakeStatusWithdrawn

	poller := NewRewardPoller(stakeRepo, balanceRepo, newFakeRewardRepo(), provider, publisher, time.Minute, testLogger)
	poller.reconcileUnstaking(context.Background())

	// Stake should be WITHDRAWN
	updated := stakeRepo.stakes[stake.ID]
	assert.Equal(t, domain.StakeStateWithdrawn, updated.State)

	// Balance: staked should return to available
	bal, _ := balanceRepo.GetByCustomerAndAsset(context.Background(), "cust-1", "ETH")
	assert.True(t, bal.Staked.Equal(decimal.Zero))
	assert.True(t, bal.Available.Equal(decimal.NewFromFloat(32)))

	assert.Len(t, publisher.events, 1)
	assert.Equal(t, "stake.withdrawn", publisher.events[0].EventType)
}

func TestFetchRewards_CreatesRewardAndUpdatesBalance(t *testing.T) {
	stakeRepo := newFakeStakeRepo()
	balanceRepo := newFakeBalanceRepo()
	rewardRepo := newFakeRewardRepo()
	provider := newFakeProvider()

	// Create an ACTIVE stake
	stake, _ := domain.NewStake("cust-1", "ETH", decimal.NewFromFloat(32), "idem-1")
	_ = stake.Delegate("0xval", "prov-ref-1")
	_ = stake.Activate()
	_ = stakeRepo.Create(context.Background(), stake)

	balance := domain.NewBalance("cust-1", "ETH")
	balance.Available = decimal.NewFromFloat(68)
	balance.Staked = decimal.NewFromFloat(32)
	_ = balanceRepo.Upsert(context.Background(), balance)

	// Provider returns a reward
	provider.rewards["prov-ref-1"] = []port.RewardEntry{
		{Amount: decimal.NewFromFloat(0.01), RewardDate: "2026-04-14"},
	}

	poller := NewRewardPoller(stakeRepo, balanceRepo, rewardRepo, provider, &fakeEventPublisher{}, time.Minute, testLogger)
	poller.fetchRewards(context.Background())

	// Reward should be created
	rewards := rewardRepo.rewards[stake.ID]
	require.Len(t, rewards, 1)
	assert.True(t, rewards[0].Amount.Equal(decimal.NewFromFloat(0.01)))

	// Balance available should increase by reward amount
	bal, _ := balanceRepo.GetByCustomerAndAsset(context.Background(), "cust-1", "ETH")
	assert.True(t, bal.Available.Equal(decimal.NewFromFloat(68.01)))
}

func TestFetchRewards_DuplicateRewardIsIdempotent(t *testing.T) {
	stakeRepo := newFakeStakeRepo()
	balanceRepo := newFakeBalanceRepo()
	rewardRepo := newFakeRewardRepo()
	provider := newFakeProvider()

	stake, _ := domain.NewStake("cust-1", "ETH", decimal.NewFromFloat(32), "idem-1")
	_ = stake.Delegate("0xval", "prov-ref-1")
	_ = stake.Activate()
	_ = stakeRepo.Create(context.Background(), stake)

	balance := domain.NewBalance("cust-1", "ETH")
	balance.Available = decimal.NewFromFloat(68)
	balance.Staked = decimal.NewFromFloat(32)
	_ = balanceRepo.Upsert(context.Background(), balance)

	provider.rewards["prov-ref-1"] = []port.RewardEntry{
		{Amount: decimal.NewFromFloat(0.01), RewardDate: "2026-04-14"},
	}

	poller := NewRewardPoller(stakeRepo, balanceRepo, rewardRepo, provider, &fakeEventPublisher{}, time.Minute, testLogger)

	// First poll — reward created
	poller.fetchRewards(context.Background())

	// Second poll — same date, should be idempotent (duplicate skipped)
	poller.fetchRewards(context.Background())

	// Still only 1 reward
	assert.Len(t, rewardRepo.rewards[stake.ID], 1)
}

func TestReconcileUnstaking_NotYetWithdrawn(t *testing.T) {
	stakeRepo := newFakeStakeRepo()
	provider := newFakeProvider()

	stake, _ := domain.NewStake("cust-1", "ETH", decimal.NewFromFloat(32), "idem-1")
	_ = stake.Delegate("0xval", "prov-ref-1")
	_ = stake.Activate()
	_ = stake.Unstake()
	_ = stakeRepo.Create(context.Background(), stake)

	// Provider still shows active (unbonding not complete)
	provider.stakeStatuses["prov-ref-1"] = port.ProviderStakeStatusActive

	poller := NewRewardPoller(stakeRepo, newFakeBalanceRepo(), newFakeRewardRepo(), provider, &fakeEventPublisher{}, time.Minute, testLogger)
	poller.reconcileUnstaking(context.Background())

	// Stake should still be UNSTAKING
	updated := stakeRepo.stakes[stake.ID]
	assert.Equal(t, domain.StakeStateUnstaking, updated.State)
}
