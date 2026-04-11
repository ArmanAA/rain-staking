package service

import (
	"context"

	"github.com/ArmanAA/rain-staking/internal/domain"
	"github.com/ArmanAA/rain-staking/internal/port"
)

// RewardService handles reward queries.
type RewardService struct {
	rewardRepo port.RewardRepository
}

func NewRewardService(rewardRepo port.RewardRepository) *RewardService {
	return &RewardService{rewardRepo: rewardRepo}
}

// GetSummary returns aggregated reward data for a stake.
func (s *RewardService) GetSummary(ctx context.Context, stakeID string) (*port.RewardSummary, error) {
	return s.rewardRepo.GetTotalByStakeID(ctx, stakeID)
}

// ListHistory returns paginated reward history for a stake.
func (s *RewardService) ListHistory(ctx context.Context, stakeID string, limit, offset int) ([]*domain.Reward, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.rewardRepo.ListByStakeID(ctx, stakeID, limit, offset)
}
