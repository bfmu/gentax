package taxi_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/taxi"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxiService_Create_Success(t *testing.T) {
	repo := new(taxi.MockRepository)
	svc := taxi.NewService(repo)

	ownerID := uuid.New()
	input := taxi.CreateInput{
		OwnerID: ownerID,
		Plate:   "ABC123",
		Model:   "Toyota Corolla",
		Year:    2020,
	}
	expected := &taxi.Taxi{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Plate:     "ABC123",
		Model:     "Toyota Corolla",
		Year:      2020,
		Active:    true,
		CreatedAt: time.Now(),
	}

	repo.On("Create", context.Background(), input).Return(expected, nil)

	got, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
	// owner_id must match the input parameter — never mutated by service
	assert.Equal(t, ownerID, got.OwnerID)
	repo.AssertExpectations(t)
}

func TestTaxiService_Create_DuplicatePlate(t *testing.T) {
	repo := new(taxi.MockRepository)
	svc := taxi.NewService(repo)

	ownerID := uuid.New()
	input := taxi.CreateInput{
		OwnerID: ownerID,
		Plate:   "DUP123",
		Model:   "Honda Civic",
		Year:    2021,
	}

	repo.On("Create", context.Background(), input).Return(nil, taxi.ErrDuplicatePlate)

	_, err := svc.Create(context.Background(), input)

	require.ErrorIs(t, err, taxi.ErrDuplicatePlate)
	repo.AssertExpectations(t)
}

func TestTaxiService_Create_InvalidYear(t *testing.T) {
	repo := new(taxi.MockRepository)
	svc := taxi.NewService(repo)

	ownerID := uuid.New()
	input := taxi.CreateInput{
		OwnerID: ownerID,
		Plate:   "OLD123",
		Model:   "Ancient Model",
		Year:    1850,
	}

	// Year 1850 is below 1990 — service must reject WITHOUT calling repository.
	_, err := svc.Create(context.Background(), input)

	require.ErrorIs(t, err, taxi.ErrInvalidYear)
	// Repository must NOT have been called.
	repo.AssertNotCalled(t, "Create")
}

func TestTaxiService_List_OnlyReturnsOwnerTaxis(t *testing.T) {
	repo := new(taxi.MockRepository)
	svc := taxi.NewService(repo)

	ownerID := uuid.New()
	taxis := []*taxi.Taxi{
		{ID: uuid.New(), OwnerID: ownerID, Plate: "AAA001", Model: "Model A", Year: 2019, Active: true},
		{ID: uuid.New(), OwnerID: ownerID, Plate: "BBB002", Model: "Model B", Year: 2020, Active: true},
	}

	repo.On("List", context.Background(), ownerID).Return(taxis, nil)

	got, err := svc.List(context.Background(), ownerID)

	require.NoError(t, err)
	require.Len(t, got, 2)
	// All returned taxis must belong to the same owner.
	for _, tx := range got {
		assert.Equal(t, ownerID, tx.OwnerID)
	}
	repo.AssertExpectations(t)
}

func TestTaxiService_Deactivate_Success(t *testing.T) {
	repo := new(taxi.MockRepository)
	svc := taxi.NewService(repo)

	ownerID := uuid.New()
	taxiID := uuid.New()
	existing := &taxi.Taxi{
		ID:      taxiID,
		OwnerID: ownerID,
		Plate:   "ACT001",
		Model:   "Active Model",
		Year:    2022,
		Active:  true,
	}

	repo.On("GetByID", context.Background(), taxiID, ownerID).Return(existing, nil)
	repo.On("SetActive", context.Background(), taxiID, ownerID, false).Return(nil)

	err := svc.Deactivate(context.Background(), taxiID, ownerID)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestTaxiService_Deactivate_NotFound(t *testing.T) {
	repo := new(taxi.MockRepository)
	svc := taxi.NewService(repo)

	ownerID := uuid.New()
	taxiID := uuid.New()

	repo.On("GetByID", context.Background(), taxiID, ownerID).Return(nil, taxi.ErrNotFound)

	err := svc.Deactivate(context.Background(), taxiID, ownerID)

	require.ErrorIs(t, err, taxi.ErrNotFound)
	repo.AssertNotCalled(t, "SetActive")
	repo.AssertExpectations(t)
}

func TestTaxiService_Deactivate_WrongOwner(t *testing.T) {
	repo := new(taxi.MockRepository)
	svc := taxi.NewService(repo)

	// The taxi belongs to a different owner.
	realOwnerID := uuid.New()
	requestingOwnerID := uuid.New()
	taxiID := uuid.New()

	// GetByID with the requesting owner returns ErrNotFound (DB filters by owner_id).
	repo.On("GetByID", context.Background(), taxiID, requestingOwnerID).Return(nil, taxi.ErrNotFound)

	err := svc.Deactivate(context.Background(), taxiID, requestingOwnerID)

	// Must return ErrNotFound — not a 403 to prevent enumeration.
	require.ErrorIs(t, err, taxi.ErrNotFound)
	repo.AssertNotCalled(t, "SetActive")
	repo.AssertExpectations(t)
	// Silence unused variable warning.
	_ = realOwnerID
}
