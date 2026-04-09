-- name: CreateTaxi :one
INSERT INTO taxis (owner_id, plate, model, year)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTaxiByID :one
SELECT * FROM taxis
WHERE id = $1 AND owner_id = $2;

-- name: ListTaxisByOwner :many
SELECT * FROM taxis
WHERE owner_id = $1
ORDER BY created_at DESC;

-- name: UpdateTaxiActive :one
UPDATE taxis
SET active = $1
WHERE id = $2 AND owner_id = $3
RETURNING *;

-- name: GetTaxiByOwnerAndPlate :one
SELECT * FROM taxis
WHERE owner_id = $1 AND plate = $2;
