package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArmanAA/rain-staking/gen/sqlc"
	"github.com/ArmanAA/rain-staking/internal/domain"
	"github.com/ArmanAA/rain-staking/internal/port"
)

// RewardRepo implements port.RewardRepository using PostgreSQL.
type RewardRepo struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

func NewRewardRepo(pool *pgxpool.Pool) *RewardRepo {
	return &RewardRepo{
		pool:    pool,
		queries: sqlcgen.New(pool),
	}
}

func (r *RewardRepo) Create(ctx context.Context, reward *domain.Reward) error {
	return r.queries.CreateReward(ctx, sqlcgen.CreateRewardParams{
		ID:               toUUID(reward.ID),
		StakeID:          toUUID(reward.StakeID),
		CustomerID:       toUUID(reward.CustomerID),
		Asset:            reward.Asset,
		Amount:           toNumeric(reward.Amount),
		CumulativeAmount: toNumeric(reward.CumulativeAmount),
		RewardDate:       toDate(reward.RewardDate),
		CreatedAt:        toTimestamptz(reward.CreatedAt),
	})
}

func (r *RewardRepo) ListByStakeID(ctx context.Context, stakeID string, limit, offset int) ([]*domain.Reward, error) {
	rows, err := r.queries.ListRewardsByStakeID(ctx, sqlcgen.ListRewardsByStakeIDParams{
		StakeID: toUUID(stakeID),
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, err
	}
	rewards := make([]*domain.Reward, len(rows))
	for i, row := range rows {
		rewards[i] = &domain.Reward{
			ID:               fromUUID(row.ID),
			StakeID:          fromUUID(row.StakeID),
			CustomerID:       fromUUID(row.CustomerID),
			Asset:            row.Asset,
			Amount:           fromNumeric(row.Amount),
			CumulativeAmount: fromNumeric(row.CumulativeAmount),
			RewardDate:       fromDate(row.RewardDate),
			CreatedAt:        fromTimestamptz(row.CreatedAt),
		}
	}
	return rewards, nil
}

func (r *RewardRepo) GetTotalByStakeID(ctx context.Context, stakeID string) (*port.RewardSummary, error) {
	row, err := r.queries.GetRewardSummaryByStakeID(ctx, toUUID(stakeID))
	if err != nil {
		return nil, err
	}
	var lastRewardAt *string
	if row.LastRewardAt != "" {
		lastRewardAt = &row.LastRewardAt
	}
	return &port.RewardSummary{
		TotalRewards: row.TotalRewards,
		RewardCount:  row.RewardCount,
		LastRewardAt: lastRewardAt,
	}, nil
}
