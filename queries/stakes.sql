-- name: CreateStake :exec
INSERT INTO stakes (id, customer_id, asset, amount, state, provider_ref, validator, idempotency_key, failure_reason, version, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: GetStakeByID :one
SELECT id, customer_id, asset, amount, state, provider_ref, validator, idempotency_key, failure_reason, version, created_at, updated_at
FROM stakes
WHERE id = $1;

-- name: GetStakeByIdempotencyKey :one
SELECT id, customer_id, asset, amount, state, provider_ref, validator, idempotency_key, failure_reason, version, created_at, updated_at
FROM stakes
WHERE idempotency_key = $1;

-- name: ListStakesByCustomerID :many
SELECT id, customer_id, asset, amount, state, provider_ref, validator, idempotency_key, failure_reason, version, created_at, updated_at
FROM stakes
WHERE customer_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListStakesByCustomerIDAndState :many
SELECT id, customer_id, asset, amount, state, provider_ref, validator, idempotency_key, failure_reason, version, created_at, updated_at
FROM stakes
WHERE customer_id = $1 AND state = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdateStake :execrows
UPDATE stakes
SET state = $1, provider_ref = $2, validator = $3, failure_reason = $4, version = $5, updated_at = $6
WHERE id = $7 AND version = $8;

-- name: ListStakesByState :many
SELECT id, customer_id, asset, amount, state, provider_ref, validator, idempotency_key, failure_reason, version, created_at, updated_at
FROM stakes
WHERE state = $1
ORDER BY created_at ASC
LIMIT $2;
