package grpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/ArmanAA/rain-staking/gen/staking/v1"
)

// ValidationInterceptor validates incoming request fields before they reach the handler.
func ValidationInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := validateRequest(req); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return handler(ctx, req)
	}
}

func validateRequest(req any) error {
	switch r := req.(type) {
	case *pb.CreateStakeRequest:
		if err := validateUUID(r.CustomerId, "customer_id"); err != nil {
			return err
		}
		if err := validateRequired(r.Asset, "asset"); err != nil {
			return err
		}
		if err := validateDecimal(r.Amount, "amount"); err != nil {
			return err
		}
		if err := validateRequired(r.IdempotencyKey, "idempotency_key"); err != nil {
			return err
		}

	case *pb.GetStakeRequest:
		if err := validateUUID(r.StakeId, "stake_id"); err != nil {
			return err
		}

	case *pb.ListStakesRequest:
		if err := validateUUID(r.CustomerId, "customer_id"); err != nil {
			return err
		}
		if err := validatePageSize(r.PageSize); err != nil {
			return err
		}

	case *pb.UnstakeRequest:
		if err := validateUUID(r.StakeId, "stake_id"); err != nil {
			return err
		}
		if err := validateRequired(r.IdempotencyKey, "idempotency_key"); err != nil {
			return err
		}

	case *pb.CreateBalanceRequest:
		if err := validateUUID(r.CustomerId, "customer_id"); err != nil {
			return err
		}
		if err := validateRequired(r.Asset, "asset"); err != nil {
			return err
		}
		if err := validateDecimal(r.Available, "available"); err != nil {
			return err
		}

	case *pb.GetBalanceRequest:
		if err := validateUUID(r.CustomerId, "customer_id"); err != nil {
			return err
		}
		if err := validateRequired(r.Asset, "asset"); err != nil {
			return err
		}

	case *pb.ListBalancesRequest:
		if err := validateUUID(r.CustomerId, "customer_id"); err != nil {
			return err
		}

	case *pb.GetRewardsSummaryRequest:
		if err := validateUUID(r.StakeId, "stake_id"); err != nil {
			return err
		}

	case *pb.ListRewardHistoryRequest:
		if err := validateUUID(r.StakeId, "stake_id"); err != nil {
			return err
		}
		if err := validatePageSize(r.PageSize); err != nil {
			return err
		}
	}

	return nil
}

func validateUUID(value, field string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if _, err := uuid.Parse(value); err != nil {
		return fmt.Errorf("%s must be a valid UUID", field)
	}
	return nil
}

func validateRequired(value, field string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func validateDecimal(value, field string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	d, err := decimal.NewFromString(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid decimal number", field)
	}
	if d.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%s must be greater than zero", field)
	}
	return nil
}

func validatePageSize(size int32) error {
	if size > 100 {
		return fmt.Errorf("page_size must not exceed 100")
	}
	return nil
}
