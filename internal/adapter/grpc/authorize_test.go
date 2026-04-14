package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ArmanAA/rain-staking/internal/auth"
)

func TestAuthorizeCustomer_MatchingID(t *testing.T) {
	ctx := auth.NewContextWithCustomerID(context.Background(), "cust-123")
	err := authorizeCustomer(ctx, "cust-123")
	assert.NoError(t, err)
}

func TestAuthorizeCustomer_MismatchedID(t *testing.T) {
	ctx := auth.NewContextWithCustomerID(context.Background(), "cust-123")
	err := authorizeCustomer(ctx, "cust-999")

	require.Error(t, err)
	// Should return NotFound, not PermissionDenied (prevents resource enumeration)
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Contains(t, err.Error(), "resource not found")
}

func TestAuthorizeCustomer_NoAuthInContext(t *testing.T) {
	err := authorizeCustomer(context.Background(), "cust-123")

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
