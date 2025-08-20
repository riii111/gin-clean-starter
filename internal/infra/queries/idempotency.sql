-- name: CreateIdempotencyKey :exec
INSERT INTO idempotency_keys (
    key,
    user_id,
    endpoint,
    request_hash,
    status,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: GetIdempotencyKey :one
SELECT 
    key,
    user_id,
    endpoint,
    request_hash,
    response_body_hash,
    status,
    expires_at,
    created_at,
    updated_at
FROM idempotency_keys 
WHERE key = $1 AND user_id = $2;

-- name: UpdateIdempotencyKeyStatus :exec
UPDATE idempotency_keys 
SET 
    status = $3,
    response_body_hash = $4,
    updated_at = NOW()
WHERE key = $1 AND user_id = $2;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys 
WHERE expires_at < NOW();