-- name: CreateReview :one
INSERT INTO reviews (
    id,
    user_id,
    resource_id,
    reservation_id,
    rating,
    comment
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING id;

-- name: ApplyResourceRatingStatsOnCreate :exec
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
VALUES (
  sqlc.arg(resource_id)::uuid,
  1,
  (sqlc.arg(rating)::int)::numeric,
  (CASE WHEN sqlc.arg(rating)::int = 1 THEN 1 ELSE 0 END),
  (CASE WHEN sqlc.arg(rating)::int = 2 THEN 1 ELSE 0 END),
  (CASE WHEN sqlc.arg(rating)::int = 3 THEN 1 ELSE 0 END),
  (CASE WHEN sqlc.arg(rating)::int = 4 THEN 1 ELSE 0 END),
  (CASE WHEN sqlc.arg(rating)::int = 5 THEN 1 ELSE 0 END),
  NOW()
)
ON CONFLICT (resource_id) DO UPDATE SET
  total_reviews = resource_rating_stats.total_reviews + 1,
  average_rating = ROUND(((resource_rating_stats.average_rating * resource_rating_stats.total_reviews) + (sqlc.arg(rating)::int)::numeric) / (resource_rating_stats.total_reviews + 1), 2),
  rating_1_count = resource_rating_stats.rating_1_count + (CASE WHEN sqlc.arg(rating)::int = 1 THEN 1 ELSE 0 END),
  rating_2_count = resource_rating_stats.rating_2_count + (CASE WHEN sqlc.arg(rating)::int = 2 THEN 1 ELSE 0 END),
  rating_3_count = resource_rating_stats.rating_3_count + (CASE WHEN sqlc.arg(rating)::int = 3 THEN 1 ELSE 0 END),
  rating_4_count = resource_rating_stats.rating_4_count + (CASE WHEN sqlc.arg(rating)::int = 4 THEN 1 ELSE 0 END),
  rating_5_count = resource_rating_stats.rating_5_count + (CASE WHEN sqlc.arg(rating)::int = 5 THEN 1 ELSE 0 END),
  updated_at = NOW();

-- name: ApplyResourceRatingStatsOnUpdate :exec
UPDATE resource_rating_stats
SET
  average_rating = CASE
    WHEN total_reviews = 0 THEN 0.00
    ELSE ROUND(((average_rating * total_reviews) - (sqlc.arg(old_rating)::int)::numeric + (sqlc.arg(new_rating)::int)::numeric) / total_reviews, 2)
  END,
  rating_1_count = rating_1_count + (CASE WHEN sqlc.arg(new_rating)::int = 1 THEN 1 ELSE 0 END) - (CASE WHEN sqlc.arg(old_rating)::int = 1 THEN 1 ELSE 0 END),
  rating_2_count = rating_2_count + (CASE WHEN sqlc.arg(new_rating)::int = 2 THEN 1 ELSE 0 END) - (CASE WHEN sqlc.arg(old_rating)::int = 2 THEN 1 ELSE 0 END),
  rating_3_count = rating_3_count + (CASE WHEN sqlc.arg(new_rating)::int = 3 THEN 1 ELSE 0 END) - (CASE WHEN sqlc.arg(old_rating)::int = 3 THEN 1 ELSE 0 END),
  rating_4_count = rating_4_count + (CASE WHEN sqlc.arg(new_rating)::int = 4 THEN 1 ELSE 0 END) - (CASE WHEN sqlc.arg(old_rating)::int = 4 THEN 1 ELSE 0 END),
  rating_5_count = rating_5_count + (CASE WHEN sqlc.arg(new_rating)::int = 5 THEN 1 ELSE 0 END) - (CASE WHEN sqlc.arg(old_rating)::int = 5 THEN 1 ELSE 0 END),
  updated_at = NOW()
WHERE resource_id = sqlc.arg(resource_id)::uuid;

-- name: ApplyResourceRatingStatsOnDelete :exec
UPDATE resource_rating_stats
SET
  total_reviews = GREATEST(total_reviews - 1, 0),
  average_rating = CASE
    WHEN total_reviews - 1 <= 0 THEN 0.00
    ELSE ROUND(((average_rating * total_reviews) - (sqlc.arg(rating)::int)::numeric) / (total_reviews - 1), 2)
  END,
  rating_1_count = GREATEST(rating_1_count - (CASE WHEN sqlc.arg(rating)::int = 1 THEN 1 ELSE 0 END), 0),
  rating_2_count = GREATEST(rating_2_count - (CASE WHEN sqlc.arg(rating)::int = 2 THEN 1 ELSE 0 END), 0),
  rating_3_count = GREATEST(rating_3_count - (CASE WHEN sqlc.arg(rating)::int = 3 THEN 1 ELSE 0 END), 0),
  rating_4_count = GREATEST(rating_4_count - (CASE WHEN sqlc.arg(rating)::int = 4 THEN 1 ELSE 0 END), 0),
  rating_5_count = GREATEST(rating_5_count - (CASE WHEN sqlc.arg(rating)::int = 5 THEN 1 ELSE 0 END), 0),
  updated_at = NOW()
WHERE resource_id = sqlc.arg(resource_id)::uuid;

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
