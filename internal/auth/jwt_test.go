package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret"

func TestGenerateAndValidateToken(t *testing.T) {
	customerID := "550e8400-e29b-41d4-a716-446655440000"

	token, err := GenerateToken(customerID, testSecret, time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := ValidateToken(token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, customerID, claims.CustomerID)
	assert.Equal(t, customerID, claims.Subject)
	assert.Equal(t, "rain-staking", claims.Issuer)
}

func TestValidateToken_Expired(t *testing.T) {
	token, err := GenerateToken("cust-1", testSecret, -time.Hour)
	require.NoError(t, err)

	_, err = ValidateToken(token, testSecret)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestValidateToken_WrongSecret(t *testing.T) {
	token, err := GenerateToken("cust-1", testSecret, time.Hour)
	require.NoError(t, err)

	_, err = ValidateToken(token, "wrong-secret")
	assert.Error(t, err)
}

func TestValidateToken_MalformedToken(t *testing.T) {
	_, err := ValidateToken("not-a-jwt", testSecret)
	assert.Error(t, err)
}

func TestValidateToken_EmptyToken(t *testing.T) {
	_, err := ValidateToken("", testSecret)
	assert.Error(t, err)
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// No customer ID in fresh context
	_, ok := CustomerIDFromContext(ctx)
	assert.False(t, ok)

	// Store and retrieve
	customerID := "cust-123"
	ctx = NewContextWithCustomerID(ctx, customerID)
	got, ok := CustomerIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, customerID, got)
}
