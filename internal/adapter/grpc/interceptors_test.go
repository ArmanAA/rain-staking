package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ArmanAA/rain-staking/internal/auth"
)

const testSecret = "test-secret"

// noopHandler is a gRPC handler that always succeeds.
func noopHandler(ctx context.Context, req any) (any, error) {
	return "ok", nil
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	token, err := auth.GenerateToken("cust-123", testSecret, time.Hour)
	require.NoError(t, err)

	interceptor := AuthInterceptor(testSecret)

	md := metadata.New(map[string]string{"authorization": "Bearer " + token})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		// Verify customer ID was stored in context
		id, ok := auth.CustomerIDFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "cust-123", id)
		return "ok", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestAuthInterceptor_MissingToken(t *testing.T) {
	interceptor := AuthInterceptor(testSecret)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(nil))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "missing authorization header")
}

func TestAuthInterceptor_InvalidFormat(t *testing.T) {
	interceptor := AuthInterceptor(testSecret)

	md := metadata.New(map[string]string{"authorization": "Basic abc123"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "invalid authorization format")
}

func TestAuthInterceptor_ExpiredToken(t *testing.T) {
	token, err := auth.GenerateToken("cust-123", testSecret, -time.Hour)
	require.NoError(t, err)

	interceptor := AuthInterceptor(testSecret)

	md := metadata.New(map[string]string{"authorization": "Bearer " + token})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestAuthInterceptor_WrongSecret(t *testing.T) {
	token, err := auth.GenerateToken("cust-123", "different-secret", time.Hour)
	require.NoError(t, err)

	interceptor := AuthInterceptor(testSecret)

	md := metadata.New(map[string]string{"authorization": "Bearer " + token})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestAuthInterceptor_MissingMetadata(t *testing.T) {
	interceptor := AuthInterceptor(testSecret)

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
