package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Owner is a minimal representation of the owners table row.
type Owner struct {
	ID        uuid.UUID
	Name      string
	Email     string
	CreatedAt time.Time
}

// Taxi is a minimal representation of the taxis table row.
type Taxi struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Plate     string
	Model     string
	Year      int
	Active    bool
	CreatedAt time.Time
}

// Driver is a minimal representation of the drivers table row.
type Driver struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	FullName  string
	Active    bool
	CreatedAt time.Time
}

// ExpenseCategory is a minimal representation of the expense_categories table row.
type ExpenseCategory struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Name      string
	CreatedAt time.Time
}

var fixtureCounter int

// nextID returns a unique suffix for fixture names.
func nextSuffix() string {
	fixtureCounter++
	return fmt.Sprintf("%d_%d", time.Now().UnixNano(), fixtureCounter)
}

// CreateOwner inserts a test owner and returns it.
func CreateOwner(t *testing.T, pool *pgxpool.Pool) Owner {
	t.Helper()
	ctx := context.Background()

	suffix := nextSuffix()
	name := fmt.Sprintf("Test Owner %s", suffix)
	email := fmt.Sprintf("owner_%s@test.com", suffix)

	var o Owner
	err := pool.QueryRow(ctx,
		`INSERT INTO owners (name, email) VALUES ($1, $2) RETURNING id, name, email, created_at`,
		name, email,
	).Scan(&o.ID, &o.Name, &o.Email, &o.CreatedAt)
	if err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}
	return o
}

// CreateTaxi inserts a test taxi for the given owner and returns it.
func CreateTaxi(t *testing.T, pool *pgxpool.Pool, ownerID uuid.UUID) Taxi {
	t.Helper()
	ctx := context.Background()

	suffix := nextSuffix()
	plate := fmt.Sprintf("AAA%s", suffix[:4])
	model := "Toyota Corolla"
	year := 2020

	var tx Taxi
	err := pool.QueryRow(ctx,
		`INSERT INTO taxis (owner_id, plate, model, year) VALUES ($1, $2, $3, $4)
		 RETURNING id, owner_id, plate, model, year, active, created_at`,
		ownerID, plate, model, year,
	).Scan(&tx.ID, &tx.OwnerID, &tx.Plate, &tx.Model, &tx.Year, &tx.Active, &tx.CreatedAt)
	if err != nil {
		t.Fatalf("CreateTaxi: %v", err)
	}
	return tx
}

// CreateDriver inserts a test driver for the given owner and returns it.
func CreateDriver(t *testing.T, pool *pgxpool.Pool, ownerID uuid.UUID) Driver {
	t.Helper()
	ctx := context.Background()

	suffix := nextSuffix()
	fullName := fmt.Sprintf("Test Driver %s", suffix)

	var d Driver
	err := pool.QueryRow(ctx,
		`INSERT INTO drivers (owner_id, full_name) VALUES ($1, $2)
		 RETURNING id, owner_id, full_name, active, created_at`,
		ownerID, fullName,
	).Scan(&d.ID, &d.OwnerID, &d.FullName, &d.Active, &d.CreatedAt)
	if err != nil {
		t.Fatalf("CreateDriver: %v", err)
	}
	return d
}

// Receipt is a minimal representation of the receipts table row.
type Receipt struct {
	ID             uuid.UUID
	DriverID       uuid.UUID
	TaxiID         uuid.UUID
	StorageURL     string
	TelegramFileID string
	OCRStatus      string
	CreatedAt      time.Time
}

// Expense is a minimal representation of the expenses table row.
type Expense struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	DriverID   uuid.UUID
	TaxiID     uuid.UUID
	CategoryID uuid.UUID
	ReceiptID  uuid.UUID
	Status     string
	CreatedAt  time.Time
}

// CreateReceipt inserts a test receipt for the given driver and taxi and returns it.
func CreateReceipt(t *testing.T, pool *pgxpool.Pool, driverID, taxiID uuid.UUID) Receipt {
	t.Helper()
	ctx := context.Background()

	suffix := nextSuffix()
	storageURL := fmt.Sprintf("https://storage.example.com/receipts/%s.jpg", suffix)

	var r Receipt
	err := pool.QueryRow(ctx,
		`INSERT INTO receipts (driver_id, taxi_id, storage_url)
		 VALUES ($1, $2, $3)
		 RETURNING id, driver_id, taxi_id, storage_url, COALESCE(telegram_file_id, ''), ocr_status, created_at`,
		driverID, taxiID, storageURL,
	).Scan(&r.ID, &r.DriverID, &r.TaxiID, &r.StorageURL, &r.TelegramFileID, &r.OCRStatus, &r.CreatedAt)
	if err != nil {
		t.Fatalf("CreateReceipt: %v", err)
	}
	return r
}

// CreateExpense inserts a test expense and returns it.
func CreateExpense(t *testing.T, pool *pgxpool.Pool, ownerID, driverID, taxiID, categoryID, receiptID uuid.UUID) Expense {
	t.Helper()
	ctx := context.Background()

	var e Expense
	err := pool.QueryRow(ctx,
		`INSERT INTO expenses (owner_id, driver_id, taxi_id, category_id, receipt_id, notes, status)
		 VALUES ($1, $2, $3, $4, $5, 'test expense', 'pending')
		 RETURNING id, owner_id, driver_id, taxi_id, category_id, receipt_id, status, created_at`,
		ownerID, driverID, taxiID, categoryID, receiptID,
	).Scan(&e.ID, &e.OwnerID, &e.DriverID, &e.TaxiID, &e.CategoryID, &e.ReceiptID, &e.Status, &e.CreatedAt)
	if err != nil {
		t.Fatalf("CreateExpense: %v", err)
	}
	return e
}

// CreateExpenseCategory inserts a test expense category for the given owner and returns it.
func CreateExpenseCategory(t *testing.T, pool *pgxpool.Pool, ownerID uuid.UUID) ExpenseCategory {
	t.Helper()
	ctx := context.Background()

	suffix := nextSuffix()
	name := fmt.Sprintf("Category %s", suffix)

	var ec ExpenseCategory
	err := pool.QueryRow(ctx,
		`INSERT INTO expense_categories (owner_id, name) VALUES ($1, $2)
		 RETURNING id, owner_id, name, created_at`,
		ownerID, name,
	).Scan(&ec.ID, &ec.OwnerID, &ec.Name, &ec.CreatedAt)
	if err != nil {
		t.Fatalf("CreateExpenseCategory: %v", err)
	}
	return ec
}
