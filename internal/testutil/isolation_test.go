//go:build integration

package testutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/taxi"
	"github.com/bmunoz/gentax/internal/testutil"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTwoTenants seeds two independent owners (A and B), each with 1 taxi, 1 driver,
// 1 expense category, 1 receipt, and 3 expenses. Returns the created resources for
// assertions.
func setupTwoTenants(t *testing.T) (
	ownerA, ownerB testutil.Owner,
	taxiA, taxiB testutil.Taxi,
	driverA, driverB testutil.Driver,
	expenseRepo expense.Repository,
) {
	t.Helper()

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	ownerA = testutil.CreateOwner(t, pool)
	ownerB = testutil.CreateOwner(t, pool)

	taxiA = testutil.CreateTaxi(t, pool, ownerA.ID)
	taxiB = testutil.CreateTaxi(t, pool, ownerB.ID)

	driverA = testutil.CreateDriver(t, pool, ownerA.ID)
	driverB = testutil.CreateDriver(t, pool, ownerB.ID)

	categoryA := testutil.CreateExpenseCategory(t, pool, ownerA.ID)
	categoryB := testutil.CreateExpenseCategory(t, pool, ownerB.ID)

	receiptA := testutil.CreateReceipt(t, pool, driverA.ID, taxiA.ID)
	receiptB := testutil.CreateReceipt(t, pool, driverB.ID, taxiB.ID)

	expenseRepo = expense.NewRepository(pool)

	// Create 3 expenses for owner A.
	for i := 0; i < 3; i++ {
		_, err := expenseRepo.Create(ctx, expense.CreateInput{
			OwnerID:    ownerA.ID,
			DriverID:   driverA.ID,
			TaxiID:     taxiA.ID,
			CategoryID: categoryA.ID,
			ReceiptID:  receiptA.ID,
			Notes:      "owner A expense",
		})
		require.NoError(t, err)
	}

	// Create 3 expenses for owner B.
	for i := 0; i < 3; i++ {
		_, err := expenseRepo.Create(ctx, expense.CreateInput{
			OwnerID:    ownerB.ID,
			DriverID:   driverB.ID,
			TaxiID:     taxiB.ID,
			CategoryID: categoryB.ID,
			ReceiptID:  receiptB.ID,
			Notes:      "owner B expense",
		})
		require.NoError(t, err)
	}

	return ownerA, ownerB, taxiA, taxiB, driverA, driverB, expenseRepo
}

// TestIsolation_TaxiList verifies that owner A's taxi list contains only owner A's taxi.
func TestIsolation_TaxiList(t *testing.T) {
	pool := testutil.NewTestDB(t)

	ownerA := testutil.CreateOwner(t, pool)
	ownerB := testutil.CreateOwner(t, pool)

	_ = testutil.CreateTaxi(t, pool, ownerA.ID)
	_ = testutil.CreateTaxi(t, pool, ownerB.ID)
	_ = testutil.CreateTaxi(t, pool, ownerB.ID)

	taxiRepo := taxi.NewRepository(pool)
	ctx := context.Background()

	listA, err := taxiRepo.List(ctx, ownerA.ID)
	require.NoError(t, err)
	assert.Len(t, listA, 1, "owner A should only see 1 taxi")
	assert.Equal(t, ownerA.ID, listA[0].OwnerID)

	listB, err := taxiRepo.List(ctx, ownerB.ID)
	require.NoError(t, err)
	assert.Len(t, listB, 2, "owner B should see 2 taxis")
	for _, tx := range listB {
		assert.Equal(t, ownerB.ID, tx.OwnerID)
	}
}

// TestIsolation_DriverList verifies that owner A's driver list contains only owner A's drivers.
func TestIsolation_DriverList(t *testing.T) {
	pool := testutil.NewTestDB(t)

	ownerA := testutil.CreateOwner(t, pool)
	ownerB := testutil.CreateOwner(t, pool)

	_ = testutil.CreateDriver(t, pool, ownerA.ID)
	_ = testutil.CreateDriver(t, pool, ownerB.ID)
	_ = testutil.CreateDriver(t, pool, ownerB.ID)

	driverRepo := driver.NewRepository(pool)
	ctx := context.Background()

	listA, err := driverRepo.List(ctx, ownerA.ID)
	require.NoError(t, err)
	assert.Len(t, listA, 1, "owner A should only see 1 driver")
	assert.Equal(t, ownerA.ID, listA[0].OwnerID)

	listB, err := driverRepo.List(ctx, ownerB.ID)
	require.NoError(t, err)
	assert.Len(t, listB, 2, "owner B should see 2 drivers")
	for _, d := range listB {
		assert.Equal(t, ownerB.ID, d.OwnerID)
	}
}

// TestIsolation_ExpenseList verifies that owner A's expense list contains exactly 3 expenses
// belonging only to owner A, and owner B's list contains exactly 3 expenses for owner B.
func TestIsolation_ExpenseList(t *testing.T) {
	ownerA, ownerB, _, _, _, _, expenseRepo := setupTwoTenants(t)
	ctx := context.Background()

	listA, err := expenseRepo.List(ctx, expense.ListFilter{OwnerID: ownerA.ID, Limit: 20})
	require.NoError(t, err)
	assert.Len(t, listA, 3, "owner A should see exactly 3 expenses")
	for _, e := range listA {
		assert.Equal(t, ownerA.ID, e.OwnerID, "all expenses must belong to owner A")
	}

	listB, err := expenseRepo.List(ctx, expense.ListFilter{OwnerID: ownerB.ID, Limit: 20})
	require.NoError(t, err)
	assert.Len(t, listB, 3, "owner B should see exactly 3 expenses")
	for _, e := range listB {
		assert.Equal(t, ownerB.ID, e.OwnerID, "all expenses must belong to owner B")
	}
}

// TestIsolation_TaxiGetByID_WrongOwner verifies that requesting owner A's taxi with owner B's
// ID returns ErrNotFound (REQ-TNT-03 — 404 not 403 to prevent enumeration).
func TestIsolation_TaxiGetByID_WrongOwner(t *testing.T) {
	pool := testutil.NewTestDB(t)

	ownerA := testutil.CreateOwner(t, pool)
	ownerB := testutil.CreateOwner(t, pool)

	taxiA := testutil.CreateTaxi(t, pool, ownerA.ID)

	taxiRepo := taxi.NewRepository(pool)
	ctx := context.Background()

	// Owner B tries to read owner A's taxi by its ID — must get ErrNotFound.
	_, err := taxiRepo.GetByID(ctx, taxiA.ID, ownerB.ID)
	require.ErrorIs(t, err, taxi.ErrNotFound,
		"cross-tenant taxi lookup must return ErrNotFound, not the record")
}

// TestIsolation_ExpenseGetByID_WrongOwner verifies that requesting owner A's expense with
// owner B's ID returns ErrNotFound (REQ-TNT-03).
func TestIsolation_ExpenseGetByID_WrongOwner(t *testing.T) {
	ownerA, ownerB, taxiA, _, driverA, _, expenseRepo := setupTwoTenants(t)
	ctx := context.Background()

	pool := testutil.NewTestDB(t)

	categoryA := testutil.CreateExpenseCategory(t, pool, ownerA.ID)
	receiptA := testutil.CreateReceipt(t, pool, driverA.ID, taxiA.ID)

	// Create a fresh expense for owner A.
	expA, err := expenseRepo.Create(ctx, expense.CreateInput{
		OwnerID:    ownerA.ID,
		DriverID:   driverA.ID,
		TaxiID:     taxiA.ID,
		CategoryID: categoryA.ID,
		ReceiptID:  receiptA.ID,
		Notes:      "owner A private expense",
	})
	require.NoError(t, err)

	// Owner B attempts to read owner A's expense — must get ErrNotFound.
	_, err = expenseRepo.GetByID(ctx, expA.ID, ownerB.ID)
	require.ErrorIs(t, err, expense.ErrNotFound,
		"cross-tenant expense lookup must return ErrNotFound, not the record")
}

// TestIsolation_SumByTaxi_CrossTenant verifies that aggregate totals are scoped per owner and
// owner B's approved expenses do not appear in owner A's SumByTaxi results.
func TestIsolation_SumByTaxi_CrossTenant(t *testing.T) {
	ownerA, ownerB, taxiA, taxiB, driverA, driverB, expenseRepo := setupTwoTenants(t)
	ctx := context.Background()

	pool := testutil.NewTestDB(t)

	categoryA := testutil.CreateExpenseCategory(t, pool, ownerA.ID)
	categoryB := testutil.CreateExpenseCategory(t, pool, ownerB.ID)
	receiptA := testutil.CreateReceipt(t, pool, driverA.ID, taxiA.ID)
	receiptB := testutil.CreateReceipt(t, pool, driverB.ID, taxiB.ID)

	// Create and approve one expense for each owner.
	eA, err := expenseRepo.Create(ctx, expense.CreateInput{
		OwnerID:    ownerA.ID,
		DriverID:   driverA.ID,
		TaxiID:     taxiA.ID,
		CategoryID: categoryA.ID,
		ReceiptID:  receiptA.ID,
	})
	require.NoError(t, err)
	require.NoError(t, expenseRepo.UpdateAmount(ctx, eA.ID, decimal.NewFromInt(100000)))
	require.NoError(t, expenseRepo.UpdateStatus(ctx, eA.ID, ownerA.ID, expense.StatusConfirmed, nil, ""))
	require.NoError(t, expenseRepo.UpdateStatus(ctx, eA.ID, ownerA.ID, expense.StatusApproved, &ownerA.ID, ""))

	eB, err := expenseRepo.Create(ctx, expense.CreateInput{
		OwnerID:    ownerB.ID,
		DriverID:   driverB.ID,
		TaxiID:     taxiB.ID,
		CategoryID: categoryB.ID,
		ReceiptID:  receiptB.ID,
	})
	require.NoError(t, err)
	require.NoError(t, expenseRepo.UpdateAmount(ctx, eB.ID, decimal.NewFromInt(200000)))
	require.NoError(t, expenseRepo.UpdateStatus(ctx, eB.ID, ownerB.ID, expense.StatusConfirmed, nil, ""))
	require.NoError(t, expenseRepo.UpdateStatus(ctx, eB.ID, ownerB.ID, expense.StatusApproved, &ownerB.ID, ""))

	from := time.Now().AddDate(0, -1, 0)
	to := time.Now().Add(time.Hour)

	// Owner A's summary should only include taxi A.
	summariesA, err := expenseRepo.SumByTaxi(ctx, ownerA.ID, from, to)
	require.NoError(t, err)
	for _, s := range summariesA {
		assert.Equal(t, ownerA.ID, s.TaxiID == taxiA.ID && s.TaxiID != taxiB.ID, "owner A sum should not reference owner B's taxi")
		assert.NotEqual(t, taxiB.ID, s.TaxiID, "owner B's taxi must not appear in owner A's report")
	}

	// Owner B's summary should only include taxi B.
	summariesB, err := expenseRepo.SumByTaxi(ctx, ownerB.ID, from, to)
	require.NoError(t, err)
	for _, s := range summariesB {
		assert.NotEqual(t, taxiA.ID, s.TaxiID, "owner A's taxi must not appear in owner B's report")
	}
}
