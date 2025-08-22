-- name: CreateReservation :one
INSERT INTO reservations (
    resource_id,
    user_id,
    slot,
    status,
    price_cents,
    coupon_id,
    note
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetReservationByID :one
SELECT 
    r.id,
    r.resource_id,
    r.user_id,
    r.slot,
    r.status,
    r.price_cents,
    r.coupon_id,
    r.note,
    r.created_at,
    r.updated_at,
    res.name AS resource_name,
    u.email AS user_email,
    c.code AS coupon_code
FROM reservations AS r
INNER JOIN resources AS res ON r.resource_id = res.id
INNER JOIN users AS u ON r.user_id = u.id
LEFT JOIN coupons AS c ON r.coupon_id = c.id
WHERE r.id = $1;

-- name: UpdateReservationStatus :exec
UPDATE reservations 
SET 
    status = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateReservationSlot :exec
UPDATE reservations 
SET 
    slot = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: GetReservationsByUserID :many
SELECT 
    r.id,
    r.resource_id,
    r.slot,
    r.status,
    r.price_cents,
    r.created_at,
    res.name AS resource_name
FROM reservations AS r
INNER JOIN resources AS res ON r.resource_id = res.id
WHERE r.user_id = $1
ORDER BY r.created_at DESC;

-- name: GetReservationsByUserIDPaginated :many
SELECT 
    r.id,
    r.resource_id,
    r.slot,
    r.status,
    r.price_cents,
    r.created_at,
    res.name AS resource_name
FROM reservations AS r
INNER JOIN resources AS res ON r.resource_id = res.id
WHERE r.user_id = $1
ORDER BY r.created_at DESC
LIMIT $2 OFFSET $3;