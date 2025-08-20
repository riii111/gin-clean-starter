-- name: CreateNotificationJob :exec
INSERT INTO notification_jobs (
    kind,
    payload,
    run_at,
    status
) VALUES (
    $1, $2, $3, $4
);

-- name: GetPendingNotificationJobs :many
SELECT 
    id,
    kind,
    payload,
    run_at,
    attempts,
    status,
    last_error,
    created_at,
    updated_at
FROM notification_jobs 
WHERE status = 'queued' AND run_at <= NOW()
ORDER BY run_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: UpdateNotificationJobStatus :exec
UPDATE notification_jobs 
SET 
    status = $2,
    attempts = attempts + 1,
    last_error = $3,
    updated_at = NOW()
WHERE id = $1;