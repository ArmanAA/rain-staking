package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ArmanAA/rain-staking/internal/domain"
	"github.com/ArmanAA/rain-staking/internal/port"
)

// RewardPoller periodically syncs staking rewards from the provider
// and reconciles stake states (DELEGATING → ACTIVE, UNSTAKING → WITHDRAWN).
type RewardPoller struct {
	stakeRepo   port.StakeRepository
	balanceRepo port.BalanceRepository
	rewardRepo  port.RewardRepository
	provider    port.StakingProvider
	publisher   port.EventPublisher
	interval    time.Duration
	logger      *slog.Logger
}

func NewRewardPoller(
	stakeRepo port.StakeRepository,
	balanceRepo port.BalanceRepository,
	rewardRepo port.RewardRepository,
	provider port.StakingProvider,
	publisher port.EventPublisher,
	interval time.Duration,
	logger *slog.Logger,
) *RewardPoller {
	return &RewardPoller{
		stakeRepo:   stakeRepo,
		balanceRepo: balanceRepo,
		rewardRepo:  rewardRepo,
		provider:    provider,
		publisher:   publisher,
		interval:    interval,
		logger:      logger,
	}
}

// Start begins the polling loop. It blocks until ctx is cancelled.
func (p *RewardPoller) Start(ctx context.Context) {
	p.logger.Info("reward poller started", slog.Duration("interval", p.interval))

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run immediately on start, then on interval
	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("reward poller stopped")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *RewardPoller) poll(ctx context.Context) {
	p.reconcileDelegating(ctx)
	p.reconcileUnstaking(ctx)
	p.fetchRewards(ctx)
}

// reconcileDelegating checks DELEGATING stakes and transitions them to ACTIVE if confirmed.
func (p *RewardPoller) reconcileDelegating(ctx context.Context) {
	stakes, err := p.stakeRepo.ListByState(ctx, domain.StakeStateDelegating, 100)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to list delegating stakes", slog.String("error", err.Error()))
		return
	}

	for _, stake := range stakes {
		status, err := p.provider.GetStakeStatus(ctx, stake.ProviderRef)
		if err != nil {
			p.logger.WarnContext(ctx, "failed to get stake status",
				slog.String("stake_id", stake.ID), slog.String("error", err.Error()))
			continue
		}

		switch status.Status {
		case port.ProviderStakeStatusActive:
			if err := stake.Activate(); err != nil {
				p.logger.ErrorContext(ctx, "failed to activate stake", slog.String("stake_id", stake.ID))
				continue
			}
			stake.Version++
			if err := p.stakeRepo.Update(ctx, stake); err != nil {
				p.logger.ErrorContext(ctx, "failed to update stake", slog.String("stake_id", stake.ID))
				continue
			}

			// Move balance from pending to staked
			balance, err := p.balanceRepo.GetByCustomerAndAsset(ctx, stake.CustomerID, stake.Asset)
			if err != nil {
				p.logger.ErrorContext(ctx, "failed to get balance", slog.String("stake_id", stake.ID))
				continue
			}
			if err := balance.ConfirmStake(stake.Amount); err != nil {
				p.logger.ErrorContext(ctx, "failed to confirm stake on balance", slog.String("stake_id", stake.ID))
				continue
			}
			balance.Version++
			if err := p.balanceRepo.Update(ctx, balance); err != nil {
				p.logger.ErrorContext(ctx, "failed to update balance after stake activation",
					slog.String("stake_id", stake.ID), slog.String("error", err.Error()))
				continue
			}

			_ = p.publisher.Publish(ctx, domain.NewAuditEvent(
				"stake", stake.ID, "stake.activated", "system",
				map[string]any{"validator": status.Validator},
			))

			p.logger.InfoContext(ctx, "stake activated", slog.String("stake_id", stake.ID))

		case port.ProviderStakeStatusFailed:
			_ = stake.Fail("provider reported failure")
			stake.Version++
			if err := p.stakeRepo.Update(ctx, stake); err != nil {
				p.logger.ErrorContext(ctx, "failed to update stake to failed state",
					slog.String("stake_id", stake.ID), slog.String("error", err.Error()))
				continue
			}

			// Release hold on balance
			balance, err := p.balanceRepo.GetByCustomerAndAsset(ctx, stake.CustomerID, stake.Asset)
			if err == nil {
				_ = balance.ReleaseHold(stake.Amount)
				balance.Version++
				if err := p.balanceRepo.Update(ctx, balance); err != nil {
					p.logger.ErrorContext(ctx, "failed to release balance hold after stake failure",
						slog.String("stake_id", stake.ID), slog.String("error", err.Error()))
				}
			}

			_ = p.publisher.Publish(ctx, domain.NewAuditEvent(
				"stake", stake.ID, "stake.failed", "system", nil,
			))
		}
	}
}

// reconcileUnstaking checks UNSTAKING stakes and transitions them to WITHDRAWN if complete.
func (p *RewardPoller) reconcileUnstaking(ctx context.Context) {
	stakes, err := p.stakeRepo.ListByState(ctx, domain.StakeStateUnstaking, 100)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to list unstaking stakes", slog.String("error", err.Error()))
		return
	}

	for _, stake := range stakes {
		status, err := p.provider.GetStakeStatus(ctx, stake.ProviderRef)
		if err != nil {
			p.logger.WarnContext(ctx, "failed to get unstake status",
				slog.String("stake_id", stake.ID), slog.String("error", err.Error()))
			continue
		}

		if status.Status == port.ProviderStakeStatusWithdrawn {
			if err := stake.Withdraw(); err != nil {
				continue
			}
			stake.Version++
			if err := p.stakeRepo.Update(ctx, stake); err != nil {
				continue
			}

			// Move balance from staked to available
			balance, err := p.balanceRepo.GetByCustomerAndAsset(ctx, stake.CustomerID, stake.Asset)
			if err != nil {
				continue
			}
			if err := balance.CompleteUnstake(stake.Amount); err != nil {
				continue
			}
			balance.Version++
			if err := p.balanceRepo.Update(ctx, balance); err != nil {
				p.logger.ErrorContext(ctx, "failed to update balance after withdrawal",
					slog.String("stake_id", stake.ID), slog.String("error", err.Error()))
				continue
			}

			_ = p.publisher.Publish(ctx, domain.NewAuditEvent(
				"stake", stake.ID, "stake.withdrawn", "system", nil,
			))

			p.logger.InfoContext(ctx, "stake withdrawn", slog.String("stake_id", stake.ID))
		}
	}
}

// fetchRewards polls for new rewards on all active stakes.
func (p *RewardPoller) fetchRewards(ctx context.Context) {
	stakes, err := p.stakeRepo.ListByState(ctx, domain.StakeStateActive, 100)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to list active stakes", slog.String("error", err.Error()))
		return
	}

	for _, stake := range stakes {
		entries, err := p.provider.GetRewards(ctx, stake.ProviderRef)
		if err != nil {
			p.logger.WarnContext(ctx, "failed to fetch rewards",
				slog.String("stake_id", stake.ID), slog.String("error", err.Error()))
			continue
		}

		for _, entry := range entries {
			rewardDate, err := time.Parse("2006-01-02", entry.RewardDate)
			if err != nil {
				continue
			}

			// Get current cumulative for this stake
			summary, err := p.rewardRepo.GetTotalByStakeID(ctx, stake.ID)
			cumulative := decimal.Zero
			if err == nil {
				cumulative, _ = decimal.NewFromString(summary.TotalRewards)
			}
			cumulative = cumulative.Add(entry.Amount)

			reward, err := domain.NewReward(
				stake.ID, stake.CustomerID, stake.Asset,
				entry.Amount, cumulative, rewardDate,
			)
			if err != nil {
				continue
			}

			if err := p.rewardRepo.Create(ctx, reward); err != nil {
				// Duplicate reward_date — idempotent, skip
				continue
			}

			// Add reward to customer's available balance
			balance, err := p.balanceRepo.GetByCustomerAndAsset(ctx, stake.CustomerID, stake.Asset)
			if err != nil {
				continue
			}
			_ = balance.AddReward(entry.Amount)
			balance.Version++
			if err := p.balanceRepo.Update(ctx, balance); err != nil {
				p.logger.ErrorContext(ctx, "failed to update balance with reward",
					slog.String("stake_id", stake.ID), slog.String("error", err.Error()))
				continue
			}

			p.logger.InfoContext(ctx, "reward recorded",
				slog.String("stake_id", stake.ID),
				slog.String("amount", entry.Amount.String()),
			)
		}
	}
}
