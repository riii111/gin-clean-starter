-- name: FindUserByEmail :one
SELECT
    id,
    email,
    password_hash,
    role,
    company_id,
    last_login,
    is_active,
    created_at,
    updated_at
FROM users
WHERE email = $1;

-- name: FindUserByID :one
SELECT id, email, role, company_id, last_login, is_active, created_at, updated_at
FROM users 
WHERE id = $1;

-- name: UpdateUserLastLogin :exec
UPDATE users 
SET last_login = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role, company_id, is_active)
VALUES ($1, $2, $3, $4, true)
RETURNING id, email, role, company_id, last_login, is_active, created_at, updated_at;

