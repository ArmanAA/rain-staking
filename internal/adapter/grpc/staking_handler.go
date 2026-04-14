package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/ArmanAA/rain-staking/gen/staking/v1"
	"github.com/ArmanAA/rain-staking/internal/domain"
	"github.com/ArmanAA/rain-staking/internal/service"
)

// StakingHandler implements the gRPC StakingService. It is a thin adapter
// that maps protobuf types to service calls and translates domain errors to gRPC status codes.
type StakingHandler struct {
	pb.UnimplementedStakingServiceServer
	stakingSvc *service.StakingService
	balanceSvc *service.BalanceService
	rewardSvc  *service.RewardService
}

func NewStakingHandler(
	stakingSvc *service.StakingService,
	balanceSvc *service.BalanceService,
	rewardSvc *service.RewardService,
) *StakingHandler {
	return &StakingHandler{
		stakingSvc: stakingSvc,
		balanceSvc: balanceSvc,
		rewardSvc:  rewardSvc,
	}
}

func (h *StakingHandler) CreateStake(ctx context.Context, req *pb.CreateStakeRequest) (*pb.CreateStakeResponse, error) {
	if err := authorizeCustomer(ctx, req.CustomerId); err != nil {
		return nil, err
	}

	stake, err := h.stakingSvc.CreateStake(ctx, service.CreateStakeRequest{
		CustomerID:     req.CustomerId,
		Asset:          req.Asset,
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	return &pb.CreateStakeResponse{Stake: stakeToProto(stake)}, nil
}

func (h *StakingHandler) GetStake(ctx context.Context, req *pb.GetStakeRequest) (*pb.GetStakeResponse, error) {
	stake, err := h.stakingSvc.GetStake(ctx, req.StakeId)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	if err := authorizeCustomer(ctx, stake.CustomerID); err != nil {
		return nil, err
	}
	return &pb.GetStakeResponse{Stake: stakeToProto(stake)}, nil
}

func (h *StakingHandler) ListStakes(ctx context.Context, req *pb.ListStakesRequest) (*pb.ListStakesResponse, error) {
	if err := authorizeCustomer(ctx, req.CustomerId); err != nil {
		return nil, err
	}

	var stateFilter *domain.StakeState
	if req.State != "" {
		state := domain.StakeState(req.State)
		stateFilter = &state
	}

	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 20
	}

	stakes, err := h.stakingSvc.ListStakes(ctx, req.CustomerId, stateFilter, limit, 0)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	pbStakes := make([]*pb.Stake, len(stakes))
	for i, s := range stakes {
		pbStakes[i] = stakeToProto(s)
	}

	return &pb.ListStakesResponse{Stakes: pbStakes}, nil
}

func (h *StakingHandler) Unstake(ctx context.Context, req *pb.UnstakeRequest) (*pb.UnstakeResponse, error) {
	// Verify ownership before unstaking
	existing, err := h.stakingSvc.GetStake(ctx, req.StakeId)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	if err := authorizeCustomer(ctx, existing.CustomerID); err != nil {
		return nil, err
	}

	stake, err := h.stakingSvc.Unstake(ctx, req.StakeId, req.IdempotencyKey)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	return &pb.UnstakeResponse{Stake: stakeToProto(stake)}, nil
}

func (h *StakingHandler) CreateBalance(ctx context.Context, req *pb.CreateBalanceRequest) (*pb.CreateBalanceResponse, error) {
	if err := authorizeCustomer(ctx, req.CustomerId); err != nil {
		return nil, err
	}

	amount, err := decimal.NewFromString(req.Available)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid amount")
	}

	balance, err := h.balanceSvc.CreateOrUpdateBalance(ctx, req.CustomerId, req.Asset, amount)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	return &pb.CreateBalanceResponse{Balance: balanceToProto(balance)}, nil
}

func (h *StakingHandler) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	if err := authorizeCustomer(ctx, req.CustomerId); err != nil {
		return nil, err
	}

	balance, err := h.balanceSvc.GetBalance(ctx, req.CustomerId, req.Asset)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	return &pb.GetBalanceResponse{Balance: balanceToProto(balance)}, nil
}

func (h *StakingHandler) ListBalances(ctx context.Context, req *pb.ListBalancesRequest) (*pb.ListBalancesResponse, error) {
	if err := authorizeCustomer(ctx, req.CustomerId); err != nil {
		return nil, err
	}

	balances, err := h.balanceSvc.ListBalances(ctx, req.CustomerId)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	pbBalances := make([]*pb.Balance, len(balances))
	for i, b := range balances {
		pbBalances[i] = balanceToProto(b)
	}

	return &pb.ListBalancesResponse{Balances: pbBalances}, nil
}

func (h *StakingHandler) GetRewardsSummary(ctx context.Context, req *pb.GetRewardsSummaryRequest) (*pb.GetRewardsSummaryResponse, error) {
	// Verify ownership via stake lookup
	stake, err := h.stakingSvc.GetStake(ctx, req.StakeId)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	if err := authorizeCustomer(ctx, stake.CustomerID); err != nil {
		return nil, err
	}

	summary, err := h.rewardSvc.GetSummary(ctx, req.StakeId)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	pbSummary := &pb.RewardsSummary{
		StakeId:      req.StakeId,
		TotalRewards: summary.TotalRewards,
		RewardCount:  summary.RewardCount,
	}
	if summary.LastRewardAt != nil {
		t, err := time.Parse("2006-01-02", *summary.LastRewardAt)
		if err == nil {
			pbSummary.LastRewardAt = timestamppb.New(t)
		}
	}

	return &pb.GetRewardsSummaryResponse{Summary: pbSummary}, nil
}

func (h *StakingHandler) ListRewardHistory(ctx context.Context, req *pb.ListRewardHistoryRequest) (*pb.ListRewardHistoryResponse, error) {
	// Verify ownership via stake lookup
	stake, err := h.stakingSvc.GetStake(ctx, req.StakeId)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	if err := authorizeCustomer(ctx, stake.CustomerID); err != nil {
		return nil, err
	}

	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 20
	}

	rewards, err := h.rewardSvc.ListHistory(ctx, req.StakeId, limit, 0)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	pbRewards := make([]*pb.Reward, len(rewards))
	for i, r := range rewards {
		pbRewards[i] = &pb.Reward{
			Id:               r.ID,
			StakeId:          r.StakeID,
			Amount:           r.Amount.String(),
			CumulativeAmount: r.CumulativeAmount.String(),
			RewardDate:       r.RewardDate.Format("2006-01-02"),
			CreatedAt:        timestamppb.New(r.CreatedAt),
		}
	}

	return &pb.ListRewardHistoryResponse{Rewards: pbRewards}, nil
}

// --- Mapping helpers ---

func stakeToProto(s *domain.Stake) *pb.Stake {
	return &pb.Stake{
		Id:          s.ID,
		CustomerId:  s.CustomerID,
		Asset:       s.Asset,
		Amount:      s.Amount.String(),
		State:       stateToProto(s.State),
		ProviderRef: s.ProviderRef,
		Validator:   s.Validator,
		CreatedAt:   timestamppb.New(s.CreatedAt),
		UpdatedAt:   timestamppb.New(s.UpdatedAt),
	}
}

func stateToProto(s domain.StakeState) pb.StakeState {
	switch s {
	case domain.StakeStatePending:
		return pb.StakeState_STAKE_STATE_PENDING
	case domain.StakeStateDelegating:
		return pb.StakeState_STAKE_STATE_DELEGATING
	case domain.StakeStateActive:
		return pb.StakeState_STAKE_STATE_ACTIVE
	case domain.StakeStateUnstaking:
		return pb.StakeState_STAKE_STATE_UNSTAKING
	case domain.StakeStateWithdrawn:
		return pb.StakeState_STAKE_STATE_WITHDRAWN
	case domain.StakeStateFailed:
		return pb.StakeState_STAKE_STATE_FAILED
	default:
		return pb.StakeState_STAKE_STATE_UNSPECIFIED
	}
}

func balanceToProto(b *domain.Balance) *pb.Balance {
	return &pb.Balance{
		Id:         b.ID,
		CustomerId: b.CustomerID,
		Asset:      b.Asset,
		Available:  b.Available.String(),
		Staked:     b.Staked.String(),
		Pending:    b.Pending.String(),
		UpdatedAt:  timestamppb.New(b.UpdatedAt),
	}
}

// domainErrorToGRPC maps domain errors to appropriate gRPC status codes.
func domainErrorToGRPC(err error) error {
	switch {
	case errors.Is(err, domain.ErrStakeNotFound), errors.Is(err, domain.ErrBalanceNotFound), errors.Is(err, domain.ErrRewardNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInsufficientBalance):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrInvalidAmount):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidStateTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrOptimisticLock):
		return status.Error(codes.Aborted, err.Error())
	case errors.Is(err, domain.ErrDuplicateIdempotency):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
