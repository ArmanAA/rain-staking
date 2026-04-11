-- name: UpsertBalance :exec
INSERT INTO balances (id, customer_id, asset, available, staked, pending, version, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (customer_id, asset)
DO UPDATE SET available = EXCLUDED.available, staked = EXCLUDED.staked, pending = EXCLUDED.pending,
             version = EXCLUDED.version, updated_at = EXCLUDED.updated_at;

-- name: GetBalanceByCustomerAndAsset :one
SELECT id, customer_id, asset, available, staked, pending, version, created_at, updated_at
FROM balances
WHERE customer_id = $1 AND asset = $2;

-- name: ListBalancesByCustomerID :many
SELECT id, customer_id, asset, available, staked, pending, version, created_at, updated_at
FROM balances
WHERE customer_id = $1
ORDER BY asset ASC;

-- name: UpdateBalance :execrows
UPDATE balances
SET available = $1, staked = $2, pending = $3, version = $4, updated_at = $5
WHERE id = $6 AND version = $7;
