-- name: CreateReview :one
INSERT INTO reviews (
    user_id,
    resource_id,
    reservation_id,
    rating,
    comment
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: UpdateReview :exec
UPDATE reviews
SET
    rating = $2,
    comment = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: DeleteReview :exec
DELETE FROM reviews WHERE id = $1;

-- name: GetReviewByID :one
SELECT * FROM reviews WHERE id = $1;

-- name: GetReviewViewByID :one
SELECT 
  r.id,
  r.user_id,
  u.email AS user_email,
  r.resource_id,
  res.name AS resource_name,
  r.reservation_id,
  r.rating,
  r.comment,
  r.created_at,
  r.updated_at
FROM reviews r
JOIN users u ON r.user_id = u.id
JOIN resources res ON r.resource_id = res.id
WHERE r.id = $1;

-- name: GetReviewsByResourceFirstPage :many
SELECT 
  r.id,
  u.email AS user_email,
  r.rating,
  r.comment,
  r.created_at
FROM reviews r
JOIN users u ON r.user_id = u.id
WHERE r.resource_id = $1
  AND (sqlc.narg(min_rating) IS NULL OR r.rating >= sqlc.narg(min_rating))
  AND (sqlc.narg(max_rating) IS NULL OR r.rating <= sqlc.narg(max_rating))
ORDER BY r.created_at DESC, r.id DESC
LIMIT $2;

-- name: GetReviewsByResourceKeyset :many
SELECT 
  r.id,
  u.email AS user_email,
  r.rating,
  r.comment,
  r.created_at
FROM reviews r
JOIN users u ON r.user_id = u.id
WHERE r.resource_id = $1
  AND (r.created_at < $2 OR (r.created_at = $2 AND r.id < $3))
  AND (sqlc.narg(min_rating) IS NULL OR r.rating >= sqlc.narg(min_rating))
  AND (sqlc.narg(max_rating) IS NULL OR r.rating <= sqlc.narg(max_rating))
ORDER BY r.created_at DESC, r.id DESC
LIMIT $4;

-- name: GetReviewsByUserFirstPage :many
SELECT 
  r.id,
  u.email AS user_email,
  r.rating,
  r.comment,
  r.created_at
FROM reviews r
JOIN users u ON r.user_id = u.id
WHERE r.user_id = $1
ORDER BY r.created_at DESC, r.id DESC
LIMIT $2;

-- name: GetReviewsByUserKeyset :many
SELECT 
  r.id,
  u.email AS user_email,
  r.rating,
  r.comment,
  r.created_at
FROM reviews r
JOIN users u ON r.user_id = u.id
WHERE r.user_id = $1
  AND (r.created_at < $2 OR (r.created_at = $2 AND r.id < $3))
ORDER BY r.created_at DESC, r.id DESC
LIMIT $4;

-- name: GetResourceRatingStats :one
SELECT 
  resource_id,
  total_reviews,
  average_rating,
  rating_1_count,
  rating_2_count,
  rating_3_count,
  rating_4_count,
  rating_5_count,
  updated_at
FROM resource_rating_stats
WHERE resource_id = $1;
