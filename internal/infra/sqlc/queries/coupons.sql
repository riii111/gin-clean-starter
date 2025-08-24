-- name: GetCouponByCode :one
SELECT 
    id,
    code,
    amount_off_cents,
    percent_off,
    valid_from,
    valid_to,
    created_at,
    updated_at
FROM coupons 
WHERE code = $1;

-- name: GetCouponByID :one
SELECT 
    id,
    code,
    amount_off_cents,
    percent_off,
    valid_from,
    valid_to,
    created_at,
    updated_at
FROM coupons 
WHERE id = $1;
