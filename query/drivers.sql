-- name: CreateDriver :one
INSERT INTO drivers (owner_id, full_name)
VALUES ($1, $2)
RETURNING *;

-- name: GetDriverByID :one
SELECT * FROM drivers
WHERE id = $1 AND owner_id = $2;

-- name: GetDriverByTelegramID :one
SELECT * FROM drivers
WHERE telegram_id = $1;

-- name: ListDriversByOwner :many
SELECT * FROM drivers
WHERE owner_id = $1
ORDER BY created_at DESC;

-- name: UpdateDriverActive :one
UPDATE drivers
SET active = $1
WHERE id = $2 AND owner_id = $3
RETURNING *;

-- name: UpdateDriverLinkToken :one
UPDATE drivers
SET link_token = $1,
    link_token_expires_at = $2,
    link_token_used = false
WHERE id = $3 AND owner_id = $4
RETURNING *;

-- name: GetDriverByLinkToken :one
SELECT * FROM drivers
WHERE link_token = $1
  AND link_token_used = false
  AND link_token_expires_at > now();

-- name: MarkDriverLinkTokenUsed :one
UPDATE drivers
SET link_token_used = true,
    telegram_id = $1
WHERE id = $2
RETURNING *;
