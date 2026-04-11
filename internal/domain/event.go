package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditEvent represents an immutable record of an action taken on a domain entity.
// These are append-only — never updated or deleted — forming a complete audit trail.
type AuditEvent struct {
	ID            string
	AggregateType string // "stake", "balance"
	AggregateID   string
	EventType     string // "stake.created", "stake.delegated", etc.
	ActorID       string
	Data          map[string]any
	CreatedAt     time.Time
}

// NewAuditEvent creates a new audit event.
func NewAuditEvent(aggregateType, aggregateID, eventType, actorID string, data map[string]any) *AuditEvent {
	return &AuditEvent{
		ID:            uuid.New().String(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		ActorID:       actorID,
		Data:          data,
		CreatedAt:     time.Now().UTC(),
	}
}
