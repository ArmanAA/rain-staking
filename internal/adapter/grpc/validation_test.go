package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/ArmanAA/rain-staking/gen/staking/v1"
)

func TestValidateRequest_CreateStake(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.CreateStakeRequest
		wantErr string
	}{
		{
			name: "valid request",
			req: &pb.CreateStakeRequest{
				CustomerId:     "550e8400-e29b-41d4-a716-446655440000",
				Asset:          "ETH",
				Amount:         "32",
				IdempotencyKey: "key-1",
			},
		},
		{
			name: "missing customer_id",
			req: &pb.CreateStakeRequest{
				Asset: "ETH", Amount: "32", IdempotencyKey: "key-1",
			},
			wantErr: "customer_id is required",
		},
		{
			name: "invalid customer_id format",
			req: &pb.CreateStakeRequest{
				CustomerId: "not-a-uuid", Asset: "ETH", Amount: "32", IdempotencyKey: "key-1",
			},
			wantErr: "customer_id must be a valid UUID",
		},
		{
			name: "missing asset",
			req: &pb.CreateStakeRequest{
				CustomerId: "550e8400-e29b-41d4-a716-446655440000", Amount: "32", IdempotencyKey: "key-1",
			},
			wantErr: "asset is required",
		},
		{
			name: "invalid amount",
			req: &pb.CreateStakeRequest{
				CustomerId: "550e8400-e29b-41d4-a716-446655440000", Asset: "ETH", Amount: "abc", IdempotencyKey: "key-1",
			},
			wantErr: "amount must be a valid decimal",
		},
		{
			name: "zero amount",
			req: &pb.CreateStakeRequest{
				CustomerId: "550e8400-e29b-41d4-a716-446655440000", Asset: "ETH", Amount: "0", IdempotencyKey: "key-1",
			},
			wantErr: "amount must be greater than zero",
		},
		{
			name: "negative amount",
			req: &pb.CreateStakeRequest{
				CustomerId: "550e8400-e29b-41d4-a716-446655440000", Asset: "ETH", Amount: "-5", IdempotencyKey: "key-1",
			},
			wantErr: "amount must be greater than zero",
		},
		{
			name: "missing idempotency_key",
			req: &pb.CreateStakeRequest{
				CustomerId: "550e8400-e29b-41d4-a716-446655440000", Asset: "ETH", Amount: "32",
			},
			wantErr: "idempotency_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequest(tt.req)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRequest_GetStake(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validateRequest(&pb.GetStakeRequest{StakeId: "550e8400-e29b-41d4-a716-446655440000"})
		assert.NoError(t, err)
	})

	t.Run("missing stake_id", func(t *testing.T) {
		err := validateRequest(&pb.GetStakeRequest{})
		assert.ErrorContains(t, err, "stake_id is required")
	})

	t.Run("invalid stake_id", func(t *testing.T) {
		err := validateRequest(&pb.GetStakeRequest{StakeId: "bad"})
		assert.ErrorContains(t, err, "stake_id must be a valid UUID")
	})
}

func TestValidateRequest_ListStakes(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validateRequest(&pb.ListStakesRequest{
			CustomerId: "550e8400-e29b-41d4-a716-446655440000",
		})
		assert.NoError(t, err)
	})

	t.Run("page_size too large", func(t *testing.T) {
		err := validateRequest(&pb.ListStakesRequest{
			CustomerId: "550e8400-e29b-41d4-a716-446655440000",
			PageSize:   200,
		})
		assert.ErrorContains(t, err, "page_size must not exceed 100")
	})
}

func TestValidateRequest_UnstakeRequest(t *testing.T) {
	t.Run("missing idempotency_key", func(t *testing.T) {
		err := validateRequest(&pb.UnstakeRequest{
			StakeId: "550e8400-e29b-41d4-a716-446655440000",
		})
		assert.ErrorContains(t, err, "idempotency_key is required")
	})
}

func TestValidateRequest_CreateBalance(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validateRequest(&pb.CreateBalanceRequest{
			CustomerId: "550e8400-e29b-41d4-a716-446655440000",
			Asset:      "ETH",
			Available:  "100",
		})
		assert.NoError(t, err)
	})

	t.Run("invalid available amount", func(t *testing.T) {
		err := validateRequest(&pb.CreateBalanceRequest{
			CustomerId: "550e8400-e29b-41d4-a716-446655440000",
			Asset:      "ETH",
			Available:  "not-a-number",
		})
		assert.ErrorContains(t, err, "available must be a valid decimal")
	})
}

func TestValidateRequest_GetBalance(t *testing.T) {
	t.Run("missing asset", func(t *testing.T) {
		err := validateRequest(&pb.GetBalanceRequest{
			CustomerId: "550e8400-e29b-41d4-a716-446655440000",
		})
		assert.ErrorContains(t, err, "asset is required")
	})
}

func TestValidateRequest_RewardRequests(t *testing.T) {
	t.Run("GetRewardsSummary valid", func(t *testing.T) {
		err := validateRequest(&pb.GetRewardsSummaryRequest{
			StakeId: "550e8400-e29b-41d4-a716-446655440000",
		})
		assert.NoError(t, err)
	})

	t.Run("ListRewardHistory page_size too large", func(t *testing.T) {
		err := validateRequest(&pb.ListRewardHistoryRequest{
			StakeId:  "550e8400-e29b-41d4-a716-446655440000",
			PageSize: 150,
		})
		assert.ErrorContains(t, err, "page_size must not exceed 100")
	})
}
