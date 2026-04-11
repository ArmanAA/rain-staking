-- name: CreateAuditEvent :exec
INSERT INTO audit_events (id, aggregate_type, aggregate_id, event_type, actor_id, data, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);
