package grpc

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ArmanAA/rain-staking/internal/auth"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// LoggingInterceptor logs every gRPC request with duration, method, and status.
func LoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := uuid.New().String()[:8]
		ctx = context.WithValue(ctx, requestIDKey, requestID)
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)

		logger.InfoContext(ctx, "grpc request",
			slog.String("request_id", requestID),
			slog.String("method", info.FullMethod),
			slog.String("status", code.String()),
			slog.Duration("duration", duration),
		)

		if err != nil {
			logger.ErrorContext(ctx, "grpc error",
				slog.String("request_id", requestID),
				slog.String("method", info.FullMethod),
				slog.String("error", err.Error()),
			)
		}

		return resp, err
	}
}

// AuthInterceptor validates JWT tokens from the Authorization metadata and
// stores the authenticated customer ID in the request context.
func AuthInterceptor(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := values[0]
		if !strings.HasPrefix(token, "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
		}
		token = strings.TrimPrefix(token, "Bearer ")

		claims, err := auth.ValidateToken(token, jwtSecret)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}

		ctx = auth.NewContextWithCustomerID(ctx, claims.CustomerID)
		return handler(ctx, req)
	}
}

// RecoveryInterceptor catches panics and converts them to internal errors.
func RecoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "panic recovered",
					slog.String("method", info.FullMethod),
					slog.Any("panic", r),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}
