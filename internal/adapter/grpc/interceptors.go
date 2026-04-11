package grpc

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
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

// RecoveryInterceptor catches panics and converts them to internal errors.
func RecoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "panic recovered",
					slog.String("method", info.FullMethod),
					slog.Any("panic", r),
				)
				err = status.Errorf(14, "internal server error") // codes.Unavailable
			}
		}()
		return handler(ctx, req)
	}
}
