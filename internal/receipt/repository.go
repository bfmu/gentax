package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// pgxRepository implements Repository using raw pgx queries.
type pgxRepository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new Repository backed by a pgxpool.Pool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &pgxRepository{db: db}
}

// Create inserts a new receipt record. Returns ErrEmptyStorageURL if StorageURL is empty.
func (r *pgxRepository) Create(ctx context.Context, rec *Receipt) (*Receipt, error) {
	if rec.StorageURL == "" {
		return nil, ErrEmptyStorageURL
	}

	const q = `
		INSERT INTO receipts (
			driver_id, taxi_id, storage_url, telegram_file_id,
			ocr_status, ocr_raw,
			extracted_total, extracted_date, extracted_vendor,
			extracted_nit, extracted_cufe, extracted_concept
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8, $9,
			$10, $11, $12
		)
		RETURNING id, created_at`

	var extractedTotal *string
	if rec.ExtractedTotal != nil {
		s := rec.ExtractedTotal.String()
		extractedTotal = &s
	}

	row := r.db.QueryRow(ctx, q,
		rec.DriverID,
		rec.TaxiID,
		rec.StorageURL,
		nullableString(rec.TelegramFileID),
		string(rec.OCRStatus),
		nilableJSON(rec.OCRRaw),
		extractedTotal,
		rec.ExtractedDate,
		rec.ExtractedVendor,
		rec.ExtractedNIT,
		rec.ExtractedCUFE,
		rec.ExtractedConcept,
	)

	if err := row.Scan(&rec.ID, &rec.CreatedAt); err != nil {
		return nil, fmt.Errorf("receipt create: %w", err)
	}
	return rec, nil
}

// GetByID retrieves a receipt by its primary key.
func (r *pgxRepository) GetByID(ctx context.Context, id uuid.UUID) (*Receipt, error) {
	const q = `
		SELECT id, driver_id, taxi_id, storage_url, telegram_file_id,
		       ocr_status, ocr_raw,
		       extracted_total, extracted_date, extracted_vendor,
		       extracted_nit, extracted_cufe, extracted_concept,
		       created_at
		FROM receipts
		WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	rec, err := scanReceipt(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("receipt get by id: %w", err)
	}
	return rec, nil
}

// ListPendingOCR returns up to 5 receipts with ocr_status='pending', oldest first,
// using FOR UPDATE SKIP LOCKED so concurrent workers don't pick the same rows.
func (r *pgxRepository) ListPendingOCR(ctx context.Context) ([]*Receipt, error) {
	const q = `
		SELECT id, driver_id, taxi_id, storage_url, telegram_file_id,
		       ocr_status, ocr_raw,
		       extracted_total, extracted_date, extracted_vendor,
		       extracted_nit, extracted_cufe, extracted_concept,
		       created_at
		FROM receipts
		WHERE ocr_status = 'pending'
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 5`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("receipt list pending: %w", err)
	}
	defer rows.Close()

	var result []*Receipt
	for rows.Next() {
		rec, err := scanReceipt(rows)
		if err != nil {
			return nil, fmt.Errorf("receipt list pending scan: %w", err)
		}
		result = append(result, rec)
	}
	return result, rows.Err()
}

// UpdateOCRFields sets the extracted DIAN fields from an OCRResult and marks status=done.
func (r *pgxRepository) UpdateOCRFields(ctx context.Context, id uuid.UUID, result *OCRResult) error {
	var total *decimal.Decimal
	if result.Total != nil {
		d, err := decimal.NewFromString(*result.Total)
		if err == nil {
			total = &d
		}
	}

	var totalStr *string
	if total != nil {
		s := total.String()
		totalStr = &s
	}

	var parsedDate *time.Time
	if result.Date != nil {
		for _, layout := range []string{"02/01/2006", "2006-01-02"} {
			if t, err := time.Parse(layout, *result.Date); err == nil {
				parsedDate = &t
				break
			}
		}
	}

	const q = `
		UPDATE receipts SET
			extracted_total   = $2,
			extracted_date    = $3,
			extracted_vendor  = $4,
			extracted_nit     = $5,
			extracted_cufe    = $6,
			extracted_concept = $7,
			ocr_raw           = $8
		WHERE id = $1`

	_, err := r.db.Exec(ctx, q,
		id,
		totalStr,
		parsedDate,
		result.Vendor,
		result.NIT,
		result.CUFE,
		result.Concept,
		nilableJSON(result.RawJSON),
	)
	if err != nil {
		return fmt.Errorf("receipt update ocr fields: %w", err)
	}
	return nil
}

// SetOCRStatus updates the ocr_status and optionally ocr_raw for a receipt.
func (r *pgxRepository) SetOCRStatus(ctx context.Context, id uuid.UUID, status OCRStatus, rawJSON []byte) error {
	const q = `
		UPDATE receipts SET
			ocr_status = $2,
			ocr_raw    = COALESCE($3, ocr_raw)
		WHERE id = $1`

	_, err := r.db.Exec(ctx, q, id, string(status), nilableJSON(rawJSON))
	if err != nil {
		return fmt.Errorf("receipt set ocr status: %w", err)
	}
	return nil
}

// scanReceipt scans a single receipt row from a pgx.Row or pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanReceipt(s scanner) (*Receipt, error) {
	var rec Receipt
	var ocrStatus string
	var telegramFileID *string
	var extractedTotalStr *string
	var extractedDate *time.Time
	var ocrRaw []byte

	err := s.Scan(
		&rec.ID,
		&rec.DriverID,
		&rec.TaxiID,
		&rec.StorageURL,
		&telegramFileID,
		&ocrStatus,
		&ocrRaw,
		&extractedTotalStr,
		&extractedDate,
		&rec.ExtractedVendor,
		&rec.ExtractedNIT,
		&rec.ExtractedCUFE,
		&rec.ExtractedConcept,
		&rec.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	rec.OCRStatus = OCRStatus(ocrStatus)
	rec.OCRRaw = ocrRaw
	rec.ExtractedDate = extractedDate

	if telegramFileID != nil {
		rec.TelegramFileID = *telegramFileID
	}
	if extractedTotalStr != nil {
		d, err := decimal.NewFromString(*extractedTotalStr)
		if err == nil {
			rec.ExtractedTotal = &d
		}
	}

	return &rec, nil
}

// nullableString converts an empty string to nil for optional TEXT columns.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nilableJSON returns nil if b is empty, otherwise b.
func nilableJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}
