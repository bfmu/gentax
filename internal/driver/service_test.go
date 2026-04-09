package driver_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestDriverService_Create_Success verifies that creating a driver stores the correct owner_id.
func TestDriverService_Create_Success(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	ownerID := uuid.New()
	input := driver.CreateInput{
		OwnerID:  ownerID,
		FullName: "Juan Pérez",
		Phone:    "3001234567",
	}
	expected := &driver.Driver{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		FullName:  "Juan Pérez",
		Phone:     "3001234567",
		Active:    true,
		CreatedAt: time.Now(),
	}

	repo.On("Create", context.Background(), input).Return(expected, nil)

	got, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
	// owner_id must match the input parameter — never mutated by service.
	assert.Equal(t, ownerID, got.OwnerID)
	repo.AssertExpectations(t)
}

// TestDriverService_Create_BlankName verifies that a blank full_name returns ErrInvalidInput
// without ever calling the repository.
func TestDriverService_Create_BlankName(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	input := driver.CreateInput{
		OwnerID:  uuid.New(),
		FullName: "   ", // whitespace only
		Phone:    "3001234567",
	}

	_, err := svc.Create(context.Background(), input)

	require.ErrorIs(t, err, driver.ErrInvalidInput)
	// Repository must NOT have been called.
	repo.AssertNotCalled(t, "Create")
}

// TestDriverService_GenerateLinkToken_Success verifies that a token is generated,
// stored with a 24h expiry, and returned.
func TestDriverService_GenerateLinkToken_Success(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	ownerID := uuid.New()
	driverID := uuid.New()
	drv := &driver.Driver{
		ID:      driverID,
		OwnerID: ownerID,
		Active:  true,
	}

	repo.On("GetByID", context.Background(), driverID, ownerID).Return(drv, nil)

	// Capture the SetLinkToken call to inspect arguments.
	var capturedToken string
	var capturedExpiry time.Time

	repo.On("SetLinkToken", context.Background(), driverID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Run(func(args mock.Arguments) {
			capturedToken = args.String(2)
			capturedExpiry = args.Get(3).(time.Time)
		}).
		Return(nil)

	before := time.Now()
	token, err := svc.GenerateLinkToken(context.Background(), driverID, ownerID)
	after := time.Now()

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, capturedToken, token)
	// Expiry must be approximately 24h from now.
	assert.WithinDuration(t, before.Add(24*time.Hour), capturedExpiry, after.Sub(before)+time.Second)
	repo.AssertExpectations(t)
}

// TestDriverService_UseLinkToken_Success verifies that a valid unused non-expired token
// links the telegram_id to the driver.
func TestDriverService_UseLinkToken_Success(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	driverID := uuid.New()
	token := "validtoken123"
	telegramID := int64(987654321)
	expiresAt := time.Now().Add(time.Hour) // not expired

	drv := &driver.Driver{
		ID:                 driverID,
		FullName:           "Carlos",
		LinkToken:          &token,
		LinkTokenExpiresAt: &expiresAt,
		LinkTokenUsed:      false,
	}

	repo.On("GetByLinkToken", context.Background(), token).Return(drv, nil)
	repo.On("SetTelegramID", context.Background(), driverID, telegramID).Return(nil)
	repo.On("MarkLinkTokenUsed", context.Background(), driverID).Return(nil)

	err := svc.LinkTelegramID(context.Background(), token, telegramID)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// TestDriverService_UseLinkToken_Expired verifies that an expired token returns ErrLinkTokenExpired.
func TestDriverService_UseLinkToken_Expired(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	driverID := uuid.New()
	token := "expiredtoken"
	telegramID := int64(111222333)
	expiresAt := time.Now().Add(-time.Hour) // already expired

	drv := &driver.Driver{
		ID:                 driverID,
		LinkToken:          &token,
		LinkTokenExpiresAt: &expiresAt,
		LinkTokenUsed:      false,
	}

	repo.On("GetByLinkToken", context.Background(), token).Return(drv, nil)

	err := svc.LinkTelegramID(context.Background(), token, telegramID)

	require.ErrorIs(t, err, driver.ErrLinkTokenExpired)
	repo.AssertNotCalled(t, "SetTelegramID")
	repo.AssertNotCalled(t, "MarkLinkTokenUsed")
	repo.AssertExpectations(t)
}

// TestDriverService_UseLinkToken_AlreadyUsed verifies that a previously used token
// returns ErrLinkTokenUsed.
func TestDriverService_UseLinkToken_AlreadyUsed(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	driverID := uuid.New()
	token := "usedtoken"
	telegramID := int64(444555666)
	expiresAt := time.Now().Add(time.Hour) // not expired

	drv := &driver.Driver{
		ID:                 driverID,
		LinkToken:          &token,
		LinkTokenExpiresAt: &expiresAt,
		LinkTokenUsed:      true, // already used
	}

	repo.On("GetByLinkToken", context.Background(), token).Return(drv, nil)

	err := svc.LinkTelegramID(context.Background(), token, telegramID)

	require.ErrorIs(t, err, driver.ErrLinkTokenUsed)
	repo.AssertNotCalled(t, "SetTelegramID")
	repo.AssertNotCalled(t, "MarkLinkTokenUsed")
	repo.AssertExpectations(t)
}

// TestDriverService_Deactivate_Success verifies that deactivating a driver
// unassigns any active taxi and sets active=false.
func TestDriverService_Deactivate_Success(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	ownerID := uuid.New()
	driverID := uuid.New()
	drv := &driver.Driver{
		ID:      driverID,
		OwnerID: ownerID,
		Active:  true,
	}

	repo.On("GetByID", context.Background(), driverID, ownerID).Return(drv, nil)
	repo.On("UnassignDriver", context.Background(), driverID).Return(nil)
	repo.On("SetActive", context.Background(), driverID, ownerID, false).Return(nil)

	err := svc.Deactivate(context.Background(), driverID, ownerID)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// TestDriverService_Deactivate_WrongOwner verifies that deactivating a driver
// belonging to a different owner returns ErrNotFound.
func TestDriverService_Deactivate_WrongOwner(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	realOwnerID := uuid.New()
	requestingOwnerID := uuid.New()
	driverID := uuid.New()

	// GetByID with the requesting (wrong) owner returns ErrNotFound.
	repo.On("GetByID", context.Background(), driverID, requestingOwnerID).Return(nil, driver.ErrNotFound)

	err := svc.Deactivate(context.Background(), driverID, requestingOwnerID)

	require.ErrorIs(t, err, driver.ErrNotFound)
	repo.AssertNotCalled(t, "UnassignDriver")
	repo.AssertNotCalled(t, "SetActive")
	repo.AssertExpectations(t)
	_ = realOwnerID
}

// TestDriverService_List_ReturnsOnlyOwnerDrivers verifies that all returned drivers
// have the same owner_id as the requesting owner.
func TestDriverService_List_ReturnsOnlyOwnerDrivers(t *testing.T) {
	repo := new(driver.MockRepository)
	svc := driver.NewService(repo)

	ownerID := uuid.New()
	drivers := []*driver.Driver{
		{ID: uuid.New(), OwnerID: ownerID, FullName: "Driver Alpha", Active: true},
		{ID: uuid.New(), OwnerID: ownerID, FullName: "Driver Beta", Active: false},
		{ID: uuid.New(), OwnerID: ownerID, FullName: "Driver Gamma", Active: true},
	}

	repo.On("List", context.Background(), ownerID).Return(drivers, nil)

	got, err := svc.List(context.Background(), ownerID)

	require.NoError(t, err)
	require.Len(t, got, 3)
	for _, d := range got {
		assert.Equal(t, ownerID, d.OwnerID)
	}
	repo.AssertExpectations(t)
}
