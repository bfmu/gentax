//go:build integration

package expense_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpenseRepository_Create verifies that a valid CreateInput creates an expense and
// reads back all fields correctly.
func TestExpenseRepository_Create(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := expense.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	driver := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)
	category := testutil.CreateExpenseCategory(t, pool, owner.ID)
	receipt := testutil.CreateReceipt(t, pool, driver.ID, taxi.ID)

	input := expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		CategoryID: category.ID,
		ReceiptID:  receipt.ID,
		Notes:      "test gasoline",
	}

	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, owner.ID, got.OwnerID)
	assert.Equal(t, driver.ID, got.DriverID)
	assert.Equal(t, taxi.ID, got.TaxiID)
	assert.Equal(t, category.ID, got.CategoryID)
	assert.Equal(t, receipt.ID, got.ReceiptID)
	assert.Equal(t, expense.StatusPending, got.Status)
	assert.Equal(t, "test gasoline", got.Notes)
	assert.False(t, got.CreatedAt.IsZero())
}

// TestExpenseRepository_List_Filters verifies dynamic WHERE clause filtering:
// by status, taxi_id, driver_id, and date range.
func TestExpenseRepository_List_Filters(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := expense.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	driver := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)
	category := testutil.CreateExpenseCategory(t, pool, owner.ID)
	receipt := testutil.CreateReceipt(t, pool, driver.ID, taxi.ID)

	// Create 3 expenses for the owner.
	for i := 0; i < 3; i++ {
		_, err := repo.Create(ctx, expense.CreateInput{
			OwnerID:    owner.ID,
			DriverID:   driver.ID,
			TaxiID:     taxi.ID,
			CategoryID: category.ID,
			ReceiptID:  receipt.ID,
			Notes:      "expense",
		})
		require.NoError(t, err)
	}

	t.Run("filter by status pending", func(t *testing.T) {
		filter := expense.ListFilter{
			OwnerID:  owner.ID,
			Statuses: []expense.Status{expense.StatusPending},
			Limit:    20,
		}
		got, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, got, 3)
		for _, e := range got {
			assert.Equal(t, expense.StatusPending, e.Status)
		}
	})

	t.Run("filter by taxi_id", func(t *testing.T) {
		filter := expense.ListFilter{
			OwnerID: owner.ID,
			TaxiID:  &taxi.ID,
			Limit:   20,
		}
		got, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, got, 3)
	})

	t.Run("filter by driver_id", func(t *testing.T) {
		filter := expense.ListFilter{
			OwnerID:  owner.ID,
			DriverID: &driver.ID,
			Limit:    20,
		}
		got, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, got, 3)
	})

	t.Run("filter by date range — no results outside range", func(t *testing.T) {
		past := time.Now().AddDate(-1, 0, 0)
		pastEnd := time.Now().AddDate(-1, 0, 1)
		filter := expense.ListFilter{
			OwnerID:  owner.ID,
			DateFrom: &past,
			DateTo:   &pastEnd,
			Limit:    20,
		}
		got, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, got, 0)
	})

	t.Run("filter by category_id", func(t *testing.T) {
		filter := expense.ListFilter{
			OwnerID:    owner.ID,
			CategoryID: &category.ID,
			Limit:      20,
		}
		got, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, got, 3)
	})

	t.Run("pagination offset", func(t *testing.T) {
		filter := expense.ListFilter{
			OwnerID: owner.ID,
			Limit:   2,
			Offset:  0,
		}
		page1, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page1, 2)

		filter.Offset = 2
		page2, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page2, 1)
	})
}

// TestExpenseRepository_UpdateStatus_StateMachine verifies that the DB CHECK constraint
// rejects invalid status values, and that valid transitions are persisted correctly.
func TestExpenseRepository_UpdateStatus_StateMachine(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := expense.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	driver := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)
	category := testutil.CreateExpenseCategory(t, pool, owner.ID)
	receipt := testutil.CreateReceipt(t, pool, driver.ID, taxi.ID)

	e, err := repo.Create(ctx, expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		CategoryID: category.ID,
		ReceiptID:  receipt.ID,
	})
	require.NoError(t, err)

	t.Run("pending → confirmed", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, e.ID, owner.ID, expense.StatusConfirmed, nil, "")
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, e.ID, owner.ID)
		require.NoError(t, err)
		assert.Equal(t, expense.StatusConfirmed, updated.Status)
	})

	t.Run("confirmed → approved with reviewedBy", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, e.ID, owner.ID, expense.StatusApproved, &owner.ID, "")
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, e.ID, owner.ID)
		require.NoError(t, err)
		assert.Equal(t, expense.StatusApproved, updated.Status)
		require.NotNil(t, updated.ReviewedBy)
		assert.Equal(t, owner.ID, *updated.ReviewedBy)
		assert.NotNil(t, updated.ReviewedAt)
	})

	t.Run("invalid status value is rejected by DB", func(t *testing.T) {
		// Use raw SQL to bypass the application layer and verify the DB constraint.
		_, dbErr := pool.Exec(ctx,
			`UPDATE expenses SET status = 'invalid_status' WHERE id = $1`,
			e.ID,
		)
		require.Error(t, dbErr, "DB should reject invalid status via CHECK constraint")
	})
}

// TestExpenseRepository_SumByTaxi verifies that aggregate totals are correct per taxi.
func TestExpenseRepository_SumByTaxi(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := expense.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	driver := testutil.CreateDriver(t, pool, owner.ID)
	taxiA := testutil.CreateTaxi(t, pool, owner.ID)
	taxiB := testutil.CreateTaxi(t, pool, owner.ID)
	category := testutil.CreateExpenseCategory(t, pool, owner.ID)
	receiptA := testutil.CreateReceipt(t, pool, driver.ID, taxiA.ID)
	receiptB := testutil.CreateReceipt(t, pool, driver.ID, taxiB.ID)

	// Create 2 approved expenses for taxiA (total = 150000)
	for _, amt := range []int64{100000, 50000} {
		e, err := repo.Create(ctx, expense.CreateInput{
			OwnerID:    owner.ID,
			DriverID:   driver.ID,
			TaxiID:     taxiA.ID,
			CategoryID: category.ID,
			ReceiptID:  receiptA.ID,
		})
		require.NoError(t, err)
		err = repo.UpdateAmount(ctx, e.ID, decimal.NewFromInt(amt))
		require.NoError(t, err)
		// pending → confirmed → approved
		err = repo.UpdateStatus(ctx, e.ID, owner.ID, expense.StatusConfirmed, nil, "")
		require.NoError(t, err)
		err = repo.UpdateStatus(ctx, e.ID, owner.ID, expense.StatusApproved, &owner.ID, "")
		require.NoError(t, err)
	}

	// Create 1 approved expense for taxiB (total = 75000)
	eB, err := repo.Create(ctx, expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxiB.ID,
		CategoryID: category.ID,
		ReceiptID:  receiptB.ID,
	})
	require.NoError(t, err)
	err = repo.UpdateAmount(ctx, eB.ID, decimal.NewFromInt(75000))
	require.NoError(t, err)
	err = repo.UpdateStatus(ctx, eB.ID, owner.ID, expense.StatusConfirmed, nil, "")
	require.NoError(t, err)
	err = repo.UpdateStatus(ctx, eB.ID, owner.ID, expense.StatusApproved, &owner.ID, "")
	require.NoError(t, err)

	from := time.Now().AddDate(0, -1, 0)
	to := time.Now().Add(time.Hour)

	summaries, err := repo.SumByTaxi(ctx, owner.ID, from, to)
	require.NoError(t, err)

	// Build a map for easy lookup.
	byTaxi := make(map[uuid.UUID]*expense.TaxiSummary)
	for _, s := range summaries {
		byTaxi[s.TaxiID] = s
	}

	require.Contains(t, byTaxi, taxiA.ID, "taxiA should appear in results")
	require.Contains(t, byTaxi, taxiB.ID, "taxiB should appear in results")

	assert.Equal(t, 2, byTaxi[taxiA.ID].Count)
	assert.Equal(t, decimal.NewFromInt(150000), byTaxi[taxiA.ID].Total)

	assert.Equal(t, 1, byTaxi[taxiB.ID].Count)
	assert.Equal(t, decimal.NewFromInt(75000), byTaxi[taxiB.ID].Total)
}

// TestExpenseRepository_GetByID_NotFound verifies that a missing expense returns ErrNotFound.
func TestExpenseRepository_GetByID_NotFound(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := expense.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)

	_, err := repo.GetByID(ctx, uuid.New(), owner.ID)
	require.ErrorIs(t, err, expense.ErrNotFound)
}

// TestExpenseRepository_List_MultiStatus verifies that Statuses filter works with ANY($N).
func TestExpenseRepository_List_MultiStatus(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := expense.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	driver := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)
	category := testutil.CreateExpenseCategory(t, pool, owner.ID)
	receipt := testutil.CreateReceipt(t, pool, driver.ID, taxi.ID)

	// Create one pending expense.
	ep, err := repo.Create(ctx, expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		CategoryID: category.ID,
		ReceiptID:  receipt.ID,
	})
	require.NoError(t, err)

	// Create one confirmed expense.
	ec, err := repo.Create(ctx, expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		CategoryID: category.ID,
		ReceiptID:  receipt.ID,
	})
	require.NoError(t, err)
	err = repo.UpdateStatus(ctx, ec.ID, owner.ID, expense.StatusConfirmed, nil, "")
	require.NoError(t, err)

	// Create one approved expense.
	ea, err := repo.Create(ctx, expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		CategoryID: category.ID,
		ReceiptID:  receipt.ID,
	})
	require.NoError(t, err)
	err = repo.UpdateStatus(ctx, ea.ID, owner.ID, expense.StatusConfirmed, nil, "")
	require.NoError(t, err)
	err = repo.UpdateStatus(ctx, ea.ID, owner.ID, expense.StatusApproved, &owner.ID, "")
	require.NoError(t, err)

	_ = ep // used

	// Filter for pending + confirmed only.
	filter := expense.ListFilter{
		OwnerID:  owner.ID,
		Statuses: []expense.Status{expense.StatusPending, expense.StatusConfirmed},
		Limit:    20,
	}
	got, err := repo.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, e := range got {
		assert.NotEqual(t, expense.StatusApproved, e.Status)
	}
}

// TestExpenseRepository_GetByReceiptID verifies lookup by receipt_id.
func TestExpenseRepository_GetByReceiptID(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := expense.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	driver := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)
	category := testutil.CreateExpenseCategory(t, pool, owner.ID)
	receipt := testutil.CreateReceipt(t, pool, driver.ID, taxi.ID)

	created, err := repo.Create(ctx, expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		CategoryID: category.ID,
		ReceiptID:  receipt.ID,
	})
	require.NoError(t, err)

	got, err := repo.GetByReceiptID(ctx, receipt.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	_, err = repo.GetByReceiptID(ctx, uuid.New())
	require.ErrorIs(t, err, expense.ErrNotFound)
}

// TestExpenseRepository_GetReceiptStorageURL verifies URL retrieval.
func TestExpenseRepository_GetReceiptStorageURL(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := expense.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	driver := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)
	category := testutil.CreateExpenseCategory(t, pool, owner.ID)
	receipt := testutil.CreateReceipt(t, pool, driver.ID, taxi.ID)

	created, err := repo.Create(ctx, expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		CategoryID: category.ID,
		ReceiptID:  receipt.ID,
	})
	require.NoError(t, err)

	url, err := repo.GetReceiptStorageURL(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, url)

	_, err = repo.GetReceiptStorageURL(ctx, uuid.New(), owner.ID)
	require.ErrorIs(t, err, expense.ErrNotFound)
}
