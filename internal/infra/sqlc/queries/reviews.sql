-- name: CreateReview :one
INSERT INTO reviews (
    user_id,
    resource_id,
    reservation_id,
    rating,
    comment
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING id;

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
SELECT id, user_id, resource_id, reservation_id, rating, comment, created_at, updated_at FROM reviews WHERE id = $1;

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
  AND (sqlc.narg(min_rating)::int IS NULL OR r.rating >= sqlc.narg(min_rating)::int)
  AND (sqlc.narg(max_rating)::int IS NULL OR r.rating <= sqlc.narg(max_rating)::int)
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
  AND (sqlc.narg(min_rating)::int IS NULL OR r.rating >= sqlc.narg(min_rating)::int)
  AND (sqlc.narg(max_rating)::int IS NULL OR r.rating <= sqlc.narg(max_rating)::int)
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

-- name: RecalcResourceRatingStats :exec
INSERT INTO resource_rating_stats (
  resource_id,
  total_reviews,
  average_rating,
  rating_1_count,
  rating_2_count,
  rating_3_count,
  rating_4_count,
  rating_5_count,
  updated_at
)
SELECT
  $1 AS resource_id,
  COALESCE(COUNT(*), 0) AS total_reviews,
  COALESCE(ROUND(AVG(rating)::numeric, 2), 0.00) AS average_rating,
  COALESCE(SUM(CASE WHEN rating = 1 THEN 1 ELSE 0 END), 0) AS rating_1_count,
  COALESCE(SUM(CASE WHEN rating = 2 THEN 1 ELSE 0 END), 0) AS rating_2_count,
  COALESCE(SUM(CASE WHEN rating = 3 THEN 1 ELSE 0 END), 0) AS rating_3_count,
  COALESCE(SUM(CASE WHEN rating = 4 THEN 1 ELSE 0 END), 0) AS rating_4_count,
  COALESCE(SUM(CASE WHEN rating = 5 THEN 1 ELSE 0 END), 0) AS rating_5_count,
  NOW() AS updated_at
FROM reviews
WHERE resource_id = $1
ON CONFLICT (resource_id) DO UPDATE SET
  total_reviews = EXCLUDED.total_reviews,
  average_rating = EXCLUDED.average_rating,
  rating_1_count = EXCLUDED.rating_1_count,
  rating_2_count = EXCLUDED.rating_2_count,
  rating_3_count = EXCLUDED.rating_3_count,
  rating_4_count = EXCLUDED.rating_4_count,
  rating_5_count = EXCLUDED.rating_5_count,
  updated_at = NOW();
