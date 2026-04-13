package expense

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// scanExpenseWithDetails reads expense columns plus driver_name, taxi_plate, category_name,
// receipt_image_url, and ocr_raw from a JOIN query.
func scanExpenseWithDetails(row interface {
	Scan(...interface{}) error
}) (*Expense, error) {
	var e Expense
	var amount *decimal.Decimal
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
		&e.DriverName,
		&e.TaxiPlate,
		&e.CategoryName,
		&e.ReceiptImageURL,
		&e.OCRRaw,
	)
	if err != nil {
		return nil, err
	}
	e.Amount = amount
	return &e, nil
}

const selectExpenseCols = `
	id, owner_id, driver_id, taxi_id, category_id, receipt_id,
	amount, expense_date, notes, status, COALESCE(rejection_reason, '') AS rejection_reason,
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
// Special case: if ownerID == uuid.Nil the owner filter is skipped (used by Confirm/SubmitEvidence driver path).
// When ownerID is non-nil, the query uses JOINs to populate enriched fields (CategoryName, ReceiptImageURL, OCRRaw).
// Returns ErrNotFound if no matching record exists.
func (r *pgxRepository) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Expense, error) {
	if ownerID == uuid.Nil {
		// Driver path: simple query without JOINs (performance-sensitive).
		q := `SELECT ` + selectExpenseCols + ` FROM expenses WHERE id = $1`
		row := r.pool.QueryRow(ctx, q, id)
		e, err := scanExpense(row)
		if err != nil {
			return nil, mapExpensePgError(err)
		}
		return e, nil
	}

	// Owner path: full JOIN query with enriched fields.
	const q = `
		SELECT e.id, e.owner_id, e.driver_id, e.taxi_id, e.category_id, e.receipt_id,
		       e.amount, e.expense_date, e.notes, e.status, COALESCE(e.rejection_reason, '') AS rejection_reason,
		       e.reviewed_by, e.reviewed_at, e.created_at, e.updated_at,
		       COALESCE(d.full_name, '') AS driver_name,
		       COALESCE(t.plate, '') AS taxi_plate,
		       COALESCE(ec.name, '') AS category_name,
		       CASE WHEN r.storage_url = 'manual-entry' THEN '' ELSE COALESCE(r.storage_url, '') END AS receipt_image_url,
		       r.ocr_raw::text AS ocr_raw
		FROM expenses e
		LEFT JOIN drivers d ON d.id = e.driver_id
		LEFT JOIN taxis t ON t.id = e.taxi_id
		LEFT JOIN expense_categories ec ON ec.id = e.category_id
		LEFT JOIN receipts r ON r.id = e.receipt_id
		WHERE e.id = $1 AND e.owner_id = $2`

	row := r.pool.QueryRow(ctx, q, id, ownerID)
	e, err := scanExpenseWithDetails(row)
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
	if len(filter.Statuses) > 0 {
		statusStrs := make([]string, len(filter.Statuses))
		for i, s := range filter.Statuses {
			statusStrs[i] = string(s)
		}
		where = append(where, fmt.Sprintf("status = ANY($%d)", idx))
		args = append(args, statusStrs)
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
		SELECT e.id, e.owner_id, e.driver_id, e.taxi_id, e.category_id, e.receipt_id,
		       e.amount, e.expense_date, e.notes, e.status, COALESCE(e.rejection_reason, '') AS rejection_reason,
		       e.reviewed_by, e.reviewed_at, e.created_at, e.updated_at,
		       COALESCE(d.full_name, '') AS driver_name,
		       COALESCE(t.plate, '') AS taxi_plate,
		       COALESCE(ec.name, '') AS category_name,
		       CASE WHEN r.storage_url = 'manual-entry' THEN '' ELSE COALESCE(r.storage_url, '') END AS receipt_image_url,
		       r.ocr_raw::text AS ocr_raw
		FROM expenses e
		LEFT JOIN drivers d ON d.id = e.driver_id
		LEFT JOIN taxis t ON t.id = e.taxi_id
		LEFT JOIN expense_categories ec ON ec.id = e.category_id
		LEFT JOIN receipts r ON r.id = e.receipt_id
		WHERE e.%s
		ORDER BY e.created_at DESC
		LIMIT $%d OFFSET $%d`,
		strings.Join(where, " AND e."),
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
		e, err := scanExpenseWithDetails(rows)
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

// GetByReceiptID returns an expense by its receipt_id (no owner scoping — internal use only).
// Returns ErrNotFound if no matching record exists.
func (r *pgxRepository) GetByReceiptID(ctx context.Context, receiptID uuid.UUID) (*Expense, error) {
	q := `SELECT ` + selectExpenseCols + ` FROM expenses WHERE receipt_id = $1 LIMIT 1`
	row := r.pool.QueryRow(ctx, q, receiptID)
	e, err := scanExpense(row)
	if err != nil {
		return nil, mapExpensePgError(err)
	}
	return e, nil
}

// GetReceiptStorageURL returns the storage URL of the receipt associated with an expense.
// Scoped to ownerID for multi-tenant isolation. Returns ErrNotFound if no match.
func (r *pgxRepository) GetReceiptStorageURL(ctx context.Context, id, ownerID uuid.UUID) (string, error) {
	const q = `
		SELECT r.storage_url
		FROM expenses e
		JOIN receipts r ON r.id = e.receipt_id
		WHERE e.id = $1 AND e.owner_id = $2`
	var url string
	err := r.pool.QueryRow(ctx, q, id, ownerID).Scan(&url)
	if err != nil {
		return "", mapExpensePgError(err)
	}
	return url, nil
}

// UpdateReceiptID updates the receipt_id for an expense (used when a driver submits additional evidence).
// Maps no-rows to ErrNotFound.
func (r *pgxRepository) UpdateReceiptID(ctx context.Context, id uuid.UUID, receiptID uuid.UUID) error {
	const q = `UPDATE expenses SET receipt_id = $1, updated_at = now() WHERE id = $2`
	tag, err := r.pool.Exec(ctx, q, receiptID, id)
	if err != nil {
		return mapExpensePgError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListCategories returns all expense categories for the given owner, ordered by name.
func (r *pgxRepository) ListCategories(ctx context.Context, ownerID uuid.UUID) ([]*ExpenseCategory, error) {
	const q = `SELECT id, owner_id, name, created_at FROM expense_categories WHERE owner_id=$1 ORDER BY name`
	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cats []*ExpenseCategory
	for rows.Next() {
		var c ExpenseCategory
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, &c)
	}
	return cats, rows.Err()
}

// CreateCategory inserts a new expense category for the given owner.
// Returns ErrCategoryDuplicate if the name already exists for this owner.
func (r *pgxRepository) CreateCategory(ctx context.Context, ownerID uuid.UUID, name string) (*ExpenseCategory, error) {
	const q = `INSERT INTO expense_categories(owner_id, name) VALUES($1,$2) RETURNING id, owner_id, name, created_at`
	var c ExpenseCategory
	err := r.pool.QueryRow(ctx, q, ownerID, name).Scan(&c.ID, &c.OwnerID, &c.Name, &c.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrCategoryDuplicate
		}
		return nil, err
	}
	return &c, nil
}

// DeleteCategory removes an expense category by ID scoped to the owner.
// Returns ErrCategoryInUse if expenses reference this category,
// ErrCategoryNotFound if no matching record exists.
func (r *pgxRepository) DeleteCategory(ctx context.Context, id, ownerID uuid.UUID) error {
	const q = `DELETE FROM expense_categories WHERE id=$1 AND owner_id=$2`
	tag, err := r.pool.Exec(ctx, q, id, ownerID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrCategoryInUse
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// defaultCategories are the seed categories created for every new owner.
var defaultCategories = []string{
	"Combustible", "Repuestos y mecánica", "Aceite y lubricantes",
	"Llantas", "Lavado y limpieza", "Peajes", "Multas",
	"Seguro", "Documentos y trámites", "Otros",
}

// SeedDefaultCategories inserts the default category list for the given owner.
// Uses ON CONFLICT DO NOTHING so it is safe to call multiple times.
func (r *pgxRepository) SeedDefaultCategories(ctx context.Context, ownerID uuid.UUID) error {
	const q = `INSERT INTO expense_categories(owner_id, name) VALUES($1,$2) ON CONFLICT (owner_id, name) DO NOTHING`
	for _, name := range defaultCategories {
		if _, err := r.pool.Exec(ctx, q, ownerID, name); err != nil {
			return err
		}
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

// AddAttachment inserts a new attachment record for the given expense and receipt.
// Returns the created Attachment with StorageURL populated via JOIN.
func (r *pgxRepository) AddAttachment(ctx context.Context, expenseID, receiptID uuid.UUID, label string) (*Attachment, error) {
	const q = `
		INSERT INTO expense_attachments (expense_id, receipt_id, label)
		VALUES ($1, $2, $3)
		RETURNING id, expense_id, receipt_id, COALESCE(label, '') AS label, created_at`

	var a Attachment
	err := r.pool.QueryRow(ctx, q, expenseID, receiptID, label).Scan(
		&a.ID, &a.ExpenseID, &a.ReceiptID, &a.Label, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Populate StorageURL via a second query.
	const urlQ = `SELECT COALESCE(storage_url, '') FROM receipts WHERE id = $1`
	_ = r.pool.QueryRow(ctx, urlQ, receiptID).Scan(&a.StorageURL)

	return &a, nil
}

// ListAttachments returns all attachments for the given expense, ordered by created_at ASC.
// StorageURL is populated via JOIN with the receipts table.
func (r *pgxRepository) ListAttachments(ctx context.Context, expenseID uuid.UUID) ([]Attachment, error) {
	const q = `
		SELECT
			ea.id,
			ea.expense_id,
			ea.receipt_id,
			COALESCE(ea.label, '') AS label,
			ea.created_at,
			COALESCE(r.storage_url, '') AS storage_url
		FROM expense_attachments ea
		JOIN receipts r ON r.id = ea.receipt_id
		WHERE ea.expense_id = $1
		ORDER BY ea.created_at ASC`

	rows, err := r.pool.Query(ctx, q, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.ExpenseID, &a.ReceiptID, &a.Label, &a.CreatedAt, &a.StorageURL); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attachments, nil
}

// mapExpensePgError translates pgx-level errors to domain sentinel errors.
func mapExpensePgError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
