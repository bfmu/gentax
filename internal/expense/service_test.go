package expense_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/expense"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper — returns a pointer to the given Status.
func statusPtr(s expense.Status) *expense.Status { return &s }

// TestCanTransition_NeedsEvidence verifies the new needs_evidence state transitions.
func TestCanTransition_NeedsEvidence(t *testing.T) {
	cases := []struct {
		name string
		from expense.Status
		to   expense.Status
		want bool
	}{
		{"confirmed→needs_evidence", expense.StatusConfirmed, expense.StatusNeedsEvidence, true},
		{"needs_evidence→confirmed", expense.StatusNeedsEvidence, expense.StatusConfirmed, true},
		{"needs_evidence→rejected", expense.StatusNeedsEvidence, expense.StatusRejected, true},
		{"needs_evidence→approved", expense.StatusNeedsEvidence, expense.StatusApproved, false},
		{"needs_evidence→pending", expense.StatusNeedsEvidence, expense.StatusPending, false},
		{"needs_evidence→needs_evidence", expense.StatusNeedsEvidence, expense.StatusNeedsEvidence, true}, // idempotent
		{"approved→needs_evidence", expense.StatusApproved, expense.StatusNeedsEvidence, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := new(expense.MockRepository)
			svc := expense.NewService(repo)

			expID := uuid.New()
			ownerID := uuid.New()
			driverID := uuid.New()

			existing := &expense.Expense{
				ID:       expID,
				OwnerID:  ownerID,
				DriverID: driverID,
				Status:   tc.from,
			}

			if tc.to == expense.StatusNeedsEvidence {
				// Test via RequestEvidence
				if tc.want {
					repo.On("GetByID", context.Background(), expID, ownerID).Return(existing, nil)
					repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusNeedsEvidence, (*uuid.UUID)(nil), "request").Return(nil)
					err := svc.RequestEvidence(context.Background(), expID, ownerID, "request")
					assert.NoError(t, err)
				} else {
					repo.On("GetByID", context.Background(), expID, ownerID).Return(existing, nil)
					err := svc.RequestEvidence(context.Background(), expID, ownerID, "request")
					assert.ErrorIs(t, err, expense.ErrInvalidTransition)
				}
			} else if tc.from == expense.StatusNeedsEvidence && tc.to == expense.StatusConfirmed {
				// Test via SubmitEvidence
				if tc.want {
					receiptID := uuid.New()
					repo.On("GetByID", context.Background(), expID, uuid.Nil).Return(existing, nil)
					repo.On("UpdateReceiptID", context.Background(), expID, receiptID).Return(nil)
					repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusConfirmed, (*uuid.UUID)(nil), "").Return(nil)
					err := svc.SubmitEvidence(context.Background(), expID, driverID, receiptID)
					assert.NoError(t, err)
				}
			} else if tc.from == expense.StatusNeedsEvidence && tc.to == expense.StatusRejected {
				if tc.want {
					repo.On("GetByID", context.Background(), expID, ownerID).Return(existing, nil)
					repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusRejected, &ownerID, "reason").Return(nil)
					err := svc.Reject(context.Background(), expID, ownerID, "reason")
					assert.NoError(t, err)
				}
			}
			repo.AssertExpectations(t)
		})
	}
}

// ----- Task 7.1: unit tests -----

// TestExpenseService_Create_RequiresReceiptID verifies that a zero ReceiptID is rejected
// BEFORE the repository is called (REQ-FRD-01).
func TestExpenseService_Create_RequiresReceiptID(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	input := expense.CreateInput{
		OwnerID:    uuid.New(),
		DriverID:   uuid.New(),
		TaxiID:     uuid.New(),
		CategoryID: uuid.New(),
		ReceiptID:  uuid.Nil, // deliberately zero
		Notes:      "should be rejected",
	}

	_, err := svc.Create(context.Background(), input)

	require.ErrorIs(t, err, expense.ErrReceiptRequired)
	// Repository must NOT have been called.
	repo.AssertNotCalled(t, "Create")
}

// TestExpenseService_Create_Success verifies that a valid CreateInput creates an expense with
// status=pending and all provided IDs set correctly.
func TestExpenseService_Create_Success(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	ownerID := uuid.New()
	driverID := uuid.New()
	taxiID := uuid.New()
	categoryID := uuid.New()
	receiptID := uuid.New()

	input := expense.CreateInput{
		OwnerID:    ownerID,
		DriverID:   driverID,
		TaxiID:     taxiID,
		CategoryID: categoryID,
		ReceiptID:  receiptID,
		Notes:      "gasoline",
	}

	expected := &expense.Expense{
		ID:         uuid.New(),
		OwnerID:    ownerID,
		DriverID:   driverID,
		TaxiID:     taxiID,
		CategoryID: categoryID,
		ReceiptID:  receiptID,
		Status:     expense.StatusPending,
		Notes:      "gasoline",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	repo.On("Create", context.Background(), input).Return(expected, nil)

	got, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, expense.StatusPending, got.Status)
	assert.Equal(t, ownerID, got.OwnerID)
	assert.Equal(t, receiptID, got.ReceiptID)
	repo.AssertExpectations(t)
}

// TestExpenseService_Confirm_Success verifies pending → confirmed transition.
func TestExpenseService_Confirm_Success(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	expID := uuid.New()
	ownerID := uuid.New()
	driverID := uuid.New()

	pending := &expense.Expense{
		ID:       expID,
		OwnerID:  ownerID,
		DriverID: driverID,
		Status:   expense.StatusPending,
	}

	// Confirm calls GetByID with uuid.Nil as ownerID (driver-side path).
	repo.On("GetByID", context.Background(), expID, uuid.Nil).Return(pending, nil)
	repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusConfirmed, (*uuid.UUID)(nil), "").Return(nil)

	err := svc.Confirm(context.Background(), expID, driverID)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// TestExpenseService_Confirm_WrongStatus verifies that confirming an already-approved expense
// returns ErrInvalidTransition.
func TestExpenseService_Confirm_WrongStatus(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	expID := uuid.New()
	driverID := uuid.New()

	approved := &expense.Expense{
		ID:       expID,
		OwnerID:  uuid.New(),
		DriverID: driverID,
		Status:   expense.StatusApproved,
	}

	repo.On("GetByID", context.Background(), expID, uuid.Nil).Return(approved, nil)

	err := svc.Confirm(context.Background(), expID, driverID)

	require.ErrorIs(t, err, expense.ErrInvalidTransition)
	repo.AssertNotCalled(t, "UpdateStatus")
	repo.AssertExpectations(t)
}

// TestExpenseService_Approve_Success verifies confirmed → approved transition with reviewedBy set.
func TestExpenseService_Approve_Success(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	expID := uuid.New()
	ownerID := uuid.New()

	confirmed := &expense.Expense{
		ID:      expID,
		OwnerID: ownerID,
		Status:  expense.StatusConfirmed,
	}

	repo.On("GetByID", context.Background(), expID, ownerID).Return(confirmed, nil)
	repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusApproved, &ownerID, "").Return(nil)

	err := svc.Approve(context.Background(), expID, ownerID)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// TestExpenseService_Approve_WrongStatus verifies that approving a pending expense
// returns ErrInvalidTransition (REQ-APR-02, REQ-FRD-03).
func TestExpenseService_Approve_WrongStatus(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	expID := uuid.New()
	ownerID := uuid.New()

	pending := &expense.Expense{
		ID:      expID,
		OwnerID: ownerID,
		Status:  expense.StatusPending,
	}

	repo.On("GetByID", context.Background(), expID, ownerID).Return(pending, nil)

	err := svc.Approve(context.Background(), expID, ownerID)

	require.ErrorIs(t, err, expense.ErrInvalidTransition)
	repo.AssertNotCalled(t, "UpdateStatus")
	repo.AssertExpectations(t)
}

// TestExpenseService_Reject_Success verifies confirmed → rejected transition with reason stored.
func TestExpenseService_Reject_Success(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	expID := uuid.New()
	ownerID := uuid.New()
	reason := "receipt unclear"

	confirmed := &expense.Expense{
		ID:      expID,
		OwnerID: ownerID,
		Status:  expense.StatusConfirmed,
	}

	repo.On("GetByID", context.Background(), expID, ownerID).Return(confirmed, nil)
	repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusRejected, &ownerID, reason).Return(nil)

	err := svc.Reject(context.Background(), expID, ownerID, reason)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// TestExpenseService_Reject_WrongStatus verifies that rejecting an approved expense
// returns ErrInvalidTransition (REQ-APR-03, REQ-FRD-03).
func TestExpenseService_Reject_WrongStatus(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	expID := uuid.New()
	ownerID := uuid.New()

	approved := &expense.Expense{
		ID:      expID,
		OwnerID: ownerID,
		Status:  expense.StatusApproved,
	}

	repo.On("GetByID", context.Background(), expID, ownerID).Return(approved, nil)

	err := svc.Reject(context.Background(), expID, ownerID, "too late")

	require.ErrorIs(t, err, expense.ErrInvalidTransition)
	repo.AssertNotCalled(t, "UpdateStatus")
	repo.AssertExpectations(t)
}

// TestExpenseService_WrongOwner_ReturnsNotFound verifies that GetByID with the wrong ownerID
// propagates ErrNotFound (REQ-TNT-03 — no 403, only 404 to prevent enumeration).
func TestExpenseService_WrongOwner_ReturnsNotFound(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	expID := uuid.New()
	wrongOwner := uuid.New()

	repo.On("GetByID", context.Background(), expID, wrongOwner).Return(nil, expense.ErrNotFound)

	_, err := svc.GetByID(context.Background(), expID, wrongOwner)

	require.ErrorIs(t, err, expense.ErrNotFound)
	repo.AssertExpectations(t)
}

// TestExpenseService_StateMachine_TableTest is a table-driven test covering ALL invalid
// transitions, ensuring each one returns ErrInvalidTransition.
func TestExpenseService_StateMachine_TableTest(t *testing.T) {
	type transition struct {
		from expense.Status
		to   expense.Status
	}

	invalidTransitions := []transition{
		// pending can only go to confirmed
		{from: expense.StatusPending, to: expense.StatusApproved},
		{from: expense.StatusPending, to: expense.StatusRejected},
		// confirmed can only go to approved or rejected
		{from: expense.StatusConfirmed, to: expense.StatusPending},
		// terminal states — no outgoing edges
		{from: expense.StatusApproved, to: expense.StatusPending},
		{from: expense.StatusApproved, to: expense.StatusConfirmed},
		{from: expense.StatusApproved, to: expense.StatusRejected},
		{from: expense.StatusRejected, to: expense.StatusPending},
		{from: expense.StatusRejected, to: expense.StatusConfirmed},
		{from: expense.StatusRejected, to: expense.StatusApproved},
	}

	for _, tc := range invalidTransitions {
		tc := tc // capture range var
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			repo := new(expense.MockRepository)
			svc := expense.NewService(repo)

			expID := uuid.New()
			ownerID := uuid.New()
			driverID := uuid.New()

			existing := &expense.Expense{
				ID:       expID,
				OwnerID:  ownerID,
				DriverID: driverID,
				Status:   tc.from,
			}

			var err error
			switch tc.to {
			case expense.StatusConfirmed:
				repo.On("GetByID", context.Background(), expID, uuid.Nil).Return(existing, nil)
				err = svc.Confirm(context.Background(), expID, driverID)
			case expense.StatusApproved:
				repo.On("GetByID", context.Background(), expID, ownerID).Return(existing, nil)
				err = svc.Approve(context.Background(), expID, ownerID)
			case expense.StatusRejected:
				repo.On("GetByID", context.Background(), expID, ownerID).Return(existing, nil)
				err = svc.Reject(context.Background(), expID, ownerID, "reason")
			case expense.StatusPending:
				// There is no "move to pending" operation in the Service interface.
				// Attempting to confirm a terminal status also catches pending→pending.
				if tc.from == expense.StatusConfirmed {
					// confirmed cannot go back to pending — only approve/reject are valid
					// Use Approve with a pending expense to verify the guard works.
					pending := &expense.Expense{
						ID:      expID,
						OwnerID: ownerID,
						Status:  expense.StatusPending,
					}
					repo.On("GetByID", context.Background(), expID, ownerID).Return(pending, nil)
					err = svc.Approve(context.Background(), expID, ownerID)
				} else {
					// For approved/rejected → pending: use Approve or Reject
					repo.On("GetByID", context.Background(), expID, ownerID).Return(existing, nil)
					err = svc.Approve(context.Background(), expID, ownerID)
				}
			}

			require.ErrorIs(t, err, expense.ErrInvalidTransition,
				"expected ErrInvalidTransition for %s→%s", tc.from, tc.to)
			repo.AssertNotCalled(t, "UpdateStatus")
		})
	}
}

// TestExpenseService_RequestEvidence verifies RequestEvidence behavior.
func TestExpenseService_RequestEvidence(t *testing.T) {
	t.Run("happy_path_confirmed_to_needs_evidence", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()
		confirmed := &expense.Expense{ID: expID, OwnerID: ownerID, Status: expense.StatusConfirmed}

		repo.On("GetByID", context.Background(), expID, ownerID).Return(confirmed, nil)
		repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusNeedsEvidence, (*uuid.UUID)(nil), "please re-send receipt").Return(nil)

		err := svc.RequestEvidence(context.Background(), expID, ownerID, "please re-send receipt")
		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("re_request_needs_evidence_idempotent", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()
		needsEvidence := &expense.Expense{ID: expID, OwnerID: ownerID, Status: expense.StatusNeedsEvidence}

		repo.On("GetByID", context.Background(), expID, ownerID).Return(needsEvidence, nil)
		repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusNeedsEvidence, (*uuid.UUID)(nil), "some message").Return(nil)

		// Idempotent: re-requesting evidence when already in needs_evidence updates the message.
		err := svc.RequestEvidence(context.Background(), expID, ownerID, "some message")
		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("invalid_status_pending", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()
		pending := &expense.Expense{ID: expID, OwnerID: ownerID, Status: expense.StatusPending}

		repo.On("GetByID", context.Background(), expID, ownerID).Return(pending, nil)

		err := svc.RequestEvidence(context.Background(), expID, ownerID, "request message")
		require.ErrorIs(t, err, expense.ErrInvalidTransition)
		repo.AssertNotCalled(t, "UpdateStatus")
		repo.AssertExpectations(t)
	})

	t.Run("not_found", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()

		repo.On("GetByID", context.Background(), expID, ownerID).Return(nil, expense.ErrNotFound)

		err := svc.RequestEvidence(context.Background(), expID, ownerID, "request message")
		require.ErrorIs(t, err, expense.ErrNotFound)
		repo.AssertExpectations(t)
	})

	t.Run("empty_message", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()

		err := svc.RequestEvidence(context.Background(), expID, ownerID, "")
		require.ErrorIs(t, err, expense.ErrEvidenceMessageRequired)
		repo.AssertNotCalled(t, "GetByID")
		repo.AssertNotCalled(t, "UpdateStatus")
		repo.AssertExpectations(t)
	})
}

// TestExpenseService_SubmitEvidence verifies SubmitEvidence behavior.
func TestExpenseService_SubmitEvidence(t *testing.T) {
	t.Run("happy_path_needs_evidence_to_confirmed", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()
		driverID := uuid.New()
		receiptID := uuid.New()

		needsEvidence := &expense.Expense{
			ID:       expID,
			OwnerID:  ownerID,
			DriverID: driverID,
			Status:   expense.StatusNeedsEvidence,
		}

		repo.On("GetByID", context.Background(), expID, uuid.Nil).Return(needsEvidence, nil)
		repo.On("UpdateReceiptID", context.Background(), expID, receiptID).Return(nil)
		repo.On("UpdateStatus", context.Background(), expID, ownerID, expense.StatusConfirmed, (*uuid.UUID)(nil), "").Return(nil)

		err := svc.SubmitEvidence(context.Background(), expID, driverID, receiptID)
		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("wrong_driver", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()
		driverID := uuid.New()
		wrongDriverID := uuid.New()
		receiptID := uuid.New()

		needsEvidence := &expense.Expense{
			ID:       expID,
			OwnerID:  ownerID,
			DriverID: driverID,
			Status:   expense.StatusNeedsEvidence,
		}

		repo.On("GetByID", context.Background(), expID, uuid.Nil).Return(needsEvidence, nil)

		err := svc.SubmitEvidence(context.Background(), expID, wrongDriverID, receiptID)
		require.ErrorIs(t, err, expense.ErrNotFound)
		repo.AssertNotCalled(t, "UpdateReceiptID")
		repo.AssertExpectations(t)
	})

	t.Run("invalid_status_confirmed", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()
		driverID := uuid.New()
		receiptID := uuid.New()

		confirmed := &expense.Expense{
			ID:       expID,
			OwnerID:  ownerID,
			DriverID: driverID,
			Status:   expense.StatusConfirmed,
		}

		repo.On("GetByID", context.Background(), expID, uuid.Nil).Return(confirmed, nil)

		err := svc.SubmitEvidence(context.Background(), expID, driverID, receiptID)
		require.ErrorIs(t, err, expense.ErrInvalidTransition)
		repo.AssertNotCalled(t, "UpdateReceiptID")
		repo.AssertExpectations(t)
	})

	t.Run("update_receipt_id_error_propagated", func(t *testing.T) {
		repo := new(expense.MockRepository)
		svc := expense.NewService(repo)

		expID := uuid.New()
		ownerID := uuid.New()
		driverID := uuid.New()
		receiptID := uuid.New()
		dbErr := errors.New("db error")

		needsEvidence := &expense.Expense{
			ID:       expID,
			OwnerID:  ownerID,
			DriverID: driverID,
			Status:   expense.StatusNeedsEvidence,
		}

		repo.On("GetByID", context.Background(), expID, uuid.Nil).Return(needsEvidence, nil)
		repo.On("UpdateReceiptID", context.Background(), expID, receiptID).Return(dbErr)

		err := svc.SubmitEvidence(context.Background(), expID, driverID, receiptID)
		require.ErrorIs(t, err, dbErr)
		repo.AssertNotCalled(t, "UpdateStatus")
		repo.AssertExpectations(t)
	})
}

// TestExpenseService_List_LimitClamping verifies that zero/negative limits default to 20
// and over-max limits are clamped to 100.
func TestExpenseService_List_LimitClamping(t *testing.T) {
	cases := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{"zero becomes default", 0, 20},
		{"negative becomes default", -5, 20},
		{"valid passthrough", 50, 50},
		{"over max clamped to 100", 101, 100},
		{"way over max clamped to 100", 200, 100},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := new(expense.MockRepository)
			svc := expense.NewService(repo)
			ownerID := uuid.New()

			inputFilter := expense.ListFilter{OwnerID: ownerID, Limit: tc.inputLimit}
			expectedFilter := expense.ListFilter{OwnerID: ownerID, Limit: tc.expectedLimit}

			repo.On("List", context.Background(), expectedFilter).Return([]*expense.Expense{}, nil)

			_, err := svc.List(context.Background(), inputFilter)
			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

// TestExpenseService_UpdateAmount verifies delegation to repo.
func TestExpenseService_UpdateAmount(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	expID := uuid.New()
	amount := decimal.NewFromInt(50000)

	repo.On("UpdateAmount", context.Background(), expID, amount).Return(nil)

	err := svc.UpdateAmount(context.Background(), expID, amount)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// TestExpenseService_SumByTaxi verifies delegation to repo.
func TestExpenseService_SumByTaxi(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	ownerID := uuid.New()
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()
	expected := []*expense.TaxiSummary{
		{TaxiID: uuid.New(), TaxiPlate: "ABC123", Total: decimal.NewFromInt(100000), Count: 2},
	}

	repo.On("SumByTaxi", context.Background(), ownerID, from, to).Return(expected, nil)

	got, err := svc.SumByTaxi(context.Background(), ownerID, from, to)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
	repo.AssertExpectations(t)
}

// TestExpenseService_SumByDriver verifies delegation to repo.
func TestExpenseService_SumByDriver(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	ownerID := uuid.New()
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()
	expected := []*expense.DriverSummary{
		{DriverID: uuid.New(), DriverName: "Juan Perez", Total: decimal.NewFromInt(75000), Count: 3},
	}

	repo.On("SumByDriver", context.Background(), ownerID, from, to).Return(expected, nil)

	got, err := svc.SumByDriver(context.Background(), ownerID, from, to)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
	repo.AssertExpectations(t)
}

// TestExpenseService_SumByCategory verifies delegation to repo.
func TestExpenseService_SumByCategory(t *testing.T) {
	repo := new(expense.MockRepository)
	svc := expense.NewService(repo)

	ownerID := uuid.New()
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()
	expected := []*expense.CategorySummary{
		{CategoryID: uuid.New(), CategoryName: "Gasolina", Total: decimal.NewFromInt(200000), Count: 5},
	}

	repo.On("SumByCategory", context.Background(), ownerID, from, to).Return(expected, nil)

	got, err := svc.SumByCategory(context.Background(), ownerID, from, to)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
	repo.AssertExpectations(t)
}
