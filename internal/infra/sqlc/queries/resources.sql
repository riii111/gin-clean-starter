-- name: GetResourceByID :one
SELECT 
    id,
    name,
    lead_time_min,
    created_at,
    updated_at
FROM resources 
WHERE id = $1;

-- name: GetAllResources :many
SELECT 
    id,
    name,
    lead_time_min,
    created_at,
    updated_at
FROM resources 
ORDER BY name;

-- name: SearchResourcesByName :many
SELECT 
    id,
    name,
    lead_time_min,
    created_at,
    updated_at
FROM resources 
WHERE name ILIKE '%' || $1 || '%'
ORDER BY name;