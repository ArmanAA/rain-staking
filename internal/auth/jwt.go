package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const customerIDKey contextKey = "customer_id"

// Claims represents the JWT claims used for authentication.
type Claims struct {
	jwt.RegisteredClaims
	CustomerID string `json:"customer_id"`
}

// GenerateToken creates a signed JWT for the given customer.
func GenerateToken(customerID, secret string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    "rain-staking",
			Subject:   customerID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
		CustomerID: customerID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates a JWT, returning the claims on success.
func ValidateToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	if claims.CustomerID == "" {
		return nil, fmt.Errorf("missing customer_id claim")
	}
	return claims, nil
}

// NewContextWithCustomerID stores the authenticated customer ID in the context.
func NewContextWithCustomerID(ctx context.Context, customerID string) context.Context {
	return context.WithValue(ctx, customerIDKey, customerID)
}

// CustomerIDFromContext extracts the authenticated customer ID from the context.
func CustomerIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(customerIDKey).(string)
	return id, ok
}
