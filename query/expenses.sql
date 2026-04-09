-- name: CreateExpense :one
INSERT INTO expenses (
    owner_id, driver_id, taxi_id, category_id, receipt_id,
    amount, expense_date, notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetExpenseByID :one
SELECT * FROM expenses
WHERE id = $1 AND owner_id = $2;

-- name: ListExpensesByOwner :many
SELECT * FROM expenses
WHERE owner_id = $1
ORDER BY expense_date DESC, created_at DESC;

-- name: ListExpensesByOwnerAndDriver :many
SELECT * FROM expenses
WHERE owner_id = $1 AND driver_id = $2
ORDER BY expense_date DESC, created_at DESC;

-- name: ListExpensesByOwnerAndTaxi :many
SELECT * FROM expenses
WHERE owner_id = $1 AND taxi_id = $2
ORDER BY expense_date DESC, created_at DESC;

-- name: ListExpensesByOwnerAndCategory :many
SELECT * FROM expenses
WHERE owner_id = $1 AND category_id = $2
ORDER BY expense_date DESC, created_at DESC;

-- name: ListExpensesByOwnerAndStatus :many
SELECT * FROM expenses
WHERE owner_id = $1 AND status = $2
ORDER BY expense_date DESC, created_at DESC;

-- name: ListExpensesByOwnerAndDateRange :many
SELECT * FROM expenses
WHERE owner_id = $1
  AND expense_date >= $2
  AND expense_date <= $3
ORDER BY expense_date DESC, created_at DESC;

-- name: ApproveExpense :one
UPDATE expenses
SET status      = 'approved',
    reviewed_by = $1,
    reviewed_at = now(),
    updated_at  = now()
WHERE id = $2 AND owner_id = $3
RETURNING *;

-- name: RejectExpense :one
UPDATE expenses
SET status           = 'rejected',
    rejection_reason = $1,
    reviewed_by      = $2,
    reviewed_at      = now(),
    updated_at       = now()
WHERE id = $3 AND owner_id = $4
RETURNING *;

-- name: UpdateExpenseAmount :one
UPDATE expenses
SET amount     = $1,
    updated_at = now()
WHERE id = $2 AND owner_id = $3
RETURNING *;

-- name: ListExpensesByDriverAndOwner :many
SELECT * FROM expenses
WHERE driver_id = $1 AND owner_id = $2
ORDER BY expense_date DESC, created_at DESC;

-- name: ReportExpensesByTaxi :many
SELECT
    taxi_id,
    COUNT(*) AS expense_count,
    SUM(amount) AS total_amount
FROM expenses
WHERE owner_id = $1
  AND expense_date >= $2
  AND expense_date <= $3
GROUP BY taxi_id
ORDER BY total_amount DESC;

-- name: ReportExpensesByDriver :many
SELECT
    driver_id,
    COUNT(*) AS expense_count,
    SUM(amount) AS total_amount
FROM expenses
WHERE owner_id = $1
  AND expense_date >= $2
  AND expense_date <= $3
GROUP BY driver_id
ORDER BY total_amount DESC;

-- name: ReportExpensesByCategory :many
SELECT
    category_id,
    COUNT(*) AS expense_count,
    SUM(amount) AS total_amount
FROM expenses
WHERE owner_id = $1
  AND expense_date >= $2
  AND expense_date <= $3
GROUP BY category_id
ORDER BY total_amount DESC;
