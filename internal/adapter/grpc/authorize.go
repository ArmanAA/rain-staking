package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ArmanAA/rain-staking/internal/auth"
)

// authorizeCustomer verifies the authenticated caller owns the requested resource.
// Returns codes.NotFound (not PermissionDenied) on mismatch to prevent resource enumeration.
func authorizeCustomer(ctx context.Context, resourceCustomerID string) error {
	callerID, ok := auth.CustomerIDFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing authentication")
	}
	if callerID != resourceCustomerID {
		return status.Error(codes.NotFound, "resource not found")
	}
	return nil
}
