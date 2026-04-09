-- name: CreateDriverTaxiAssignment :one
INSERT INTO driver_taxi_assignments (driver_id, taxi_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetActiveAssignmentByDriver :one
SELECT * FROM driver_taxi_assignments
WHERE driver_id = $1 AND unassigned_at IS NULL;

-- name: UnassignDriver :one
UPDATE driver_taxi_assignments
SET unassigned_at = now()
WHERE driver_id = $1 AND unassigned_at IS NULL
RETURNING *;
