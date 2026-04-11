package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArmanAA/rain-staking/gen/sqlc"
	"github.com/ArmanAA/rain-staking/internal/domain"
)

// AuditEventRepo implements port.AuditEventRepository using PostgreSQL.
type AuditEventRepo struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

func NewAuditEventRepo(pool *pgxpool.Pool) *AuditEventRepo {
	return &AuditEventRepo{
		pool:    pool,
		queries: sqlcgen.New(pool),
	}
}

func (r *AuditEventRepo) Create(ctx context.Context, event *domain.AuditEvent) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	return r.queries.CreateAuditEvent(ctx, sqlcgen.CreateAuditEventParams{
		ID:            toUUID(event.ID),
		AggregateType: event.AggregateType,
		AggregateID:   toUUID(event.AggregateID),
		EventType:     event.EventType,
		ActorID:       toUUID(event.ActorID),
		Data:          data,
		CreatedAt:     toTimestamptz(event.CreatedAt),
	})
}
