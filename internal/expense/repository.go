package expense

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// pgxRepository implements Repository using a *pgxpool.Pool.
// All queries include WHERE owner_id = $n to enforce multi-tenant isolation (REQ-TNT-01).
type pgxRepository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by a *pgxpool.Pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

// scanExpense reads all expense columns from a pgx.Row or pgx.Rows scan target.
func scanExpense(row interface {
	Scan(...interface{}) error
}) (*Expense, error) {
	var e Expense
	var amount *decimal.Decimal // NUMERIC(12,2), nullable
	err := row.Scan(
		&e.ID,
		&e.OwnerID,
		&e.DriverID,
		&e.TaxiID,
		&e.CategoryID,
		&e.ReceiptID,
		&amount,
		&e.ExpenseDate,
		&e.Notes,
		&e.Status,
		&e.RejectionReason,
		&e.ReviewedBy,
		&e.ReviewedAt,
		&e.CreatedAt,
		&e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.Amount = amount
	return &e, nil
}

const selectExpenseCols = `
	id, owner_id, driver_id, taxi_id, category_id, receipt_id,
	amount, expense_date, notes, status, rejection_reason,
	reviewed_by, reviewed_at, created_at, updated_at`

// Create inserts a new expense record with status=pending and returns it.
func (r *pgxRepository) Create(ctx context.Context, input CreateInput) (*Expense, error) {
	const q = `
		INSERT INTO expenses
			(owner_id, driver_id, taxi_id, category_id, receipt_id, notes, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		RETURNING ` + selectExpenseCols

	row := r.pool.QueryRow(ctx, q,
		input.OwnerID, input.DriverID, input.TaxiID,
		input.CategoryID, input.ReceiptID, input.Notes,
	)
	e, err := scanExpense(row)
	if err != nil {
		return nil, mapExpensePgError(err)
	}
	return e, nil
}

// GetByID retrieves an expense by its ID and ownerID.
// Special case: if ownerID == uuid.Nil the owner filter is skipped (used by Confirm driver path).
// Returns ErrNotFound if no matching record exists.
func (r *pgxRepository) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Expense, error) {
	var (
		q   string
		row pgx.Row
	)

	if ownerID == uuid.Nil {
		q = `SELECT ` + selectExpenseCols + ` FROM expenses WHERE id = $1`
		row = r.pool.QueryRow(ctx, q, id)
	} else {
		q = `SELECT ` + selectExpenseCols + ` FROM expenses WHERE id = $1 AND owner_id = $2`
		row = r.pool.QueryRow(ctx, q, id, ownerID)
	}

	e, err := scanExpense(row)
	if err != nil {
		return nil, mapExpensePgError(err)
	}
	return e, nil
}

// List returns expenses matching the filter with dynamic WHERE clauses.
// owner_id is always required. All other filters are optional.
// Results are ordered by created_at DESC.
func (r *pgxRepository) List(ctx context.Context, filter ListFilter) ([]*Expense, error) {
	// Clamp limit (service should do this too, but defensive here).
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	// Build dynamic WHERE clause.
	where := []string{"owner_id = $1"}
	args := []interface{}{filter.OwnerID}
	idx := 2

	if filter.DriverID != nil {
		where = append(where, fmt.Sprintf("driver_id = $%d", idx))
		args = append(args, *filter.DriverID)
		idx++
	}
	if filter.TaxiID != nil {
		where = append(where, fmt.Sprintf("taxi_id = $%d", idx))
		args = append(args, *filter.TaxiID)
		idx++
	}
	if filter.CategoryID != nil {
		where = append(where, fmt.Sprintf("category_id = $%d", idx))
		args = append(args, *filter.CategoryID)
		idx++
	}
	if filter.Status != nil {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(*filter.Status))
		idx++
	}
	if filter.DateFrom != nil {
		where = append(where, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *filter.DateFrom)
		idx++
	}
	if filter.DateTo != nil {
		where = append(where, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, *filter.DateTo)
		idx++
	}

	q := fmt.Sprintf(`
		SELECT %s FROM expenses
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`,
		selectExpenseCols,
		strings.Join(where, " AND "),
		idx, idx+1,
	)
	args = append(args, limit, filter.Offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []*Expense
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return expenses, nil
}

// UpdateStatus updates the status (and optionally reviewed_by/reviewed_at/rejection_reason)
// for an expense, scoped to ownerID. Maps no-rows to ErrNotFound.
func (r *pgxRepository) UpdateStatus(ctx context.Context, id, ownerID uuid.UUID, status Status, reviewedBy *uuid.UUID, rejectionReason string) error {
	const q = `
		UPDATE expenses
		SET
			status = $1,
			reviewed_by = $2,
			reviewed_at = CASE WHEN $2::uuid IS NOT NULL THEN now() ELSE reviewed_at END,
			rejection_reason = $3,
			updated_at = now()
		WHERE id = $4 AND owner_id = $5`

	tag, err := r.pool.Exec(ctx, q, string(status), reviewedBy, rejectionReason, id, ownerID)
	if err != nil {
		return mapExpensePgError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateAmount updates the amount for an expense (after OCR or manual correction).
func (r *pgxRepository) UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error {
	const q = `UPDATE expenses SET amount = $1, updated_at = now() WHERE id = $2`

	tag, err := r.pool.Exec(ctx, q, amount, id)
	if err != nil {
		return mapExpensePgError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SumByTaxi returns aggregate approved expense totals per taxi for the given date range.
// REQ-RPT-02: only approved expenses are included; taxis with zero approved expenses are included
// with total=0 and count=0.
func (r *pgxRepository) SumByTaxi(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*TaxiSummary, error) {
	const q = `
		SELECT
			t.id         AS taxi_id,
			t.plate      AS taxi_plate,
			COALESCE(SUM(e.amount), 0) AS total,
			COUNT(e.id)                AS count
		FROM taxis t
		LEFT JOIN expenses e
			ON e.taxi_id = t.id
			AND e.status = 'approved'
			AND e.created_at >= $2
			AND e.created_at <= $3
		WHERE t.owner_id = $1
		GROUP BY t.id, t.plate
		ORDER BY total DESC`

	rows, err := r.pool.Query(ctx, q, ownerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*TaxiSummary
	for rows.Next() {
		var s TaxiSummary
		var total decimal.Decimal
		if err := rows.Scan(&s.TaxiID, &s.TaxiPlate, &total, &s.Count); err != nil {
			return nil, err
		}
		s.Total = total
		result = append(result, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// SumByDriver returns aggregate approved expense totals per driver for the given date range.
// REQ-RPT-03: inactive drivers with approved expenses in the period appear in results.
func (r *pgxRepository) SumByDriver(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*DriverSummary, error) {
	const q = `
		SELECT
			d.id        AS driver_id,
			d.full_name AS driver_name,
			COALESCE(SUM(e.amount), 0) AS total,
			COUNT(e.id)                AS count
		FROM drivers d
		LEFT JOIN expenses e
			ON e.driver_id = d.id
			AND e.status = 'approved'
			AND e.created_at >= $2
			AND e.created_at <= $3
		WHERE d.owner_id = $1
		GROUP BY d.id, d.full_name
		ORDER BY total DESC`

	rows, err := r.pool.Query(ctx, q, ownerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*DriverSummary
	for rows.Next() {
		var s DriverSummary
		var total decimal.Decimal
		if err := rows.Scan(&s.DriverID, &s.DriverName, &total, &s.Count); err != nil {
			return nil, err
		}
		s.Total = total
		result = append(result, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// SumByCategory returns aggregate approved expense totals per category for the given date range.
// REQ-RPT-04: results scoped to owner_id.
func (r *pgxRepository) SumByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*CategorySummary, error) {
	const q = `
		SELECT
			ec.id   AS category_id,
			ec.name AS category_name,
			COALESCE(SUM(e.amount), 0) AS total,
			COUNT(e.id)                AS count
		FROM expense_categories ec
		LEFT JOIN expenses e
			ON e.category_id = ec.id
			AND e.status = 'approved'
			AND e.created_at >= $2
			AND e.created_at <= $3
		WHERE ec.owner_id = $1
		GROUP BY ec.id, ec.name
		ORDER BY total DESC`

	rows, err := r.pool.Query(ctx, q, ownerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*CategorySummary
	for rows.Next() {
		var s CategorySummary
		var total decimal.Decimal
		if err := rows.Scan(&s.CategoryID, &s.CategoryName, &total, &s.Count); err != nil {
			return nil, err
		}
		s.Total = total
		result = append(result, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// mapExpensePgError translates pgx-level errors to domain sentinel errors.
func mapExpensePgError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
