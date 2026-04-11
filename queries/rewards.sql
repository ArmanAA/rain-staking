-- name: CreateReward :exec
INSERT INTO rewards (id, stake_id, customer_id, asset, amount, cumulative_amount, reward_date, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListRewardsByStakeID :many
SELECT id, stake_id, customer_id, asset, amount, cumulative_amount, reward_date, created_at
FROM rewards
WHERE stake_id = $1
ORDER BY reward_date DESC
LIMIT $2 OFFSET $3;

-- name: GetRewardSummaryByStakeID :one
SELECT
    COALESCE(SUM(amount), 0)::TEXT AS total_rewards,
    COUNT(*)::INT AS reward_count,
    MAX(reward_date)::TEXT AS last_reward_at
FROM rewards
WHERE stake_id = $1;
