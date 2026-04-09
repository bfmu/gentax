-- name: CreateOwner :one
INSERT INTO owners (name, email)
VALUES ($1, $2)
RETURNING *;

-- name: GetOwnerByID :one
SELECT * FROM owners
WHERE id = $1;

-- name: GetOwnerByEmail :one
SELECT * FROM owners
WHERE email = $1;
