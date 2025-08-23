-- name: TryInsertIdempotencyKey :exec
INSERT INTO idempotency_keys (
    key,
    user_id,
    endpoint,
    request_hash,
    status,
    expires_at
) VALUES (
    $1, $2, $3, $4, 'processing', $5
)
ON CONFLICT (key, user_id) DO NOTHING;

-- name: GetIdempotencyKey :one
SELECT 
    key,
    user_id,
    endpoint,
    request_hash,
    response_body_hash,
    status,
    result_reservation_id,
    expires_at,
    created_at,
    updated_at
FROM idempotency_keys 
WHERE key = $1 AND user_id = $2;

-- name: UpdateIdempotencyKeyCompleted :exec
UPDATE idempotency_keys 
SET 
    status = 'completed',
    response_body_hash = $3,
    result_reservation_id = $4,
    updated_at = NOW()
WHERE key = $1 AND user_id = $2;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys 
WHERE expires_at < NOW();