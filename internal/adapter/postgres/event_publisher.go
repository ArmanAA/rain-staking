package postgres

import (
	"context"
	"log/slog"

	"github.com/ArmanAA/rain-staking/internal/domain"
)

// LogEventPublisher publishes domain events by logging them and persisting to the audit table.
// In production, this would be replaced with a Kafka or SQS/SNS publisher
// while keeping the same EventPublisher interface.
type LogEventPublisher struct {
	auditRepo *AuditEventRepo
	logger    *slog.Logger
}

func NewLogEventPublisher(auditRepo *AuditEventRepo, logger *slog.Logger) *LogEventPublisher {
	return &LogEventPublisher{
		auditRepo: auditRepo,
		logger:    logger,
	}
}

func (p *LogEventPublisher) Publish(ctx context.Context, event *domain.AuditEvent) error {
	p.logger.InfoContext(ctx, "domain event",
		slog.String("event_type", event.EventType),
		slog.String("aggregate_type", event.AggregateType),
		slog.String("aggregate_id", event.AggregateID),
		slog.String("actor_id", event.ActorID),
	)
	return p.auditRepo.Create(ctx, event)
}
