package port

import (
	"context"

	"github.com/ArmanAA/rain-staking/internal/domain"
)

// EventPublisher defines the interface for publishing domain events.
// In production, this would be backed by Kafka, SQS/SNS, or similar.
// The current implementation logs events for observability.
type EventPublisher interface {
	Publish(ctx context.Context, event *domain.AuditEvent) error
}
