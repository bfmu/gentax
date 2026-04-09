package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v3"
)

// ─── mock: Sender ─────────────────────────────────────────────────────────────

type mockSender struct {
	mock.Mock
	sent []sentMsg
}

type sentMsg struct {
	to   tele.Recipient
	what interface{}
	opts []interface{}
}

func (m *mockSender) Send(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error) {
	m.sent = append(m.sent, sentMsg{to: to, what: what, opts: opts})
	args := m.Called(to, what, opts)
	return nil, args.Error(0)
}

// sentTexts returns the text of every sent message, in order.
func (m *mockSender) sentTexts() []string {
	texts := make([]string, 0, len(m.sent))
	for _, s := range m.sent {
		if txt, ok := s.what.(string); ok {
			texts = append(texts, txt)
		}
	}
	return texts
}

// ─── mock: driver.Service ─────────────────────────────────────────────────────

type mockDriverService struct{ mock.Mock }

func (m *mockDriverService) Create(ctx context.Context, input driver.CreateInput) (*driver.Driver, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*driver.Driver), args.Error(1)
}

func (m *mockDriverService) GenerateLinkToken(ctx context.Context, driverID, ownerID uuid.UUID) (string, error) {
	args := m.Called(ctx, driverID, ownerID)
	return args.String(0), args.Error(1)
}

func (m *mockDriverService) LinkTelegramID(ctx context.Context, token string, telegramID int64) error {
	args := m.Called(ctx, token, telegramID)
	return args.Error(0)
}

func (m *mockDriverService) Deactivate(ctx context.Context, id, ownerID uuid.UUID) error {
	args := m.Called(ctx, id, ownerID)
	return args.Error(0)
}

func (m *mockDriverService) List(ctx context.Context, ownerID uuid.UUID) ([]*driver.Driver, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*driver.Driver), args.Error(1)
}

func (m *mockDriverService) AssignTaxi(ctx context.Context, driverID, taxiID, ownerID uuid.UUID) error {
	args := m.Called(ctx, driverID, taxiID, ownerID)
	return args.Error(0)
}

func (m *mockDriverService) UnassignTaxi(ctx context.Context, driverID, ownerID uuid.UUID) error {
	args := m.Called(ctx, driverID, ownerID)
	return args.Error(0)
}

// ─── mock: DriverRepo (DriverRepo interface) ──────────────────────────────────

type mockDriverRepo struct{ mock.Mock }

func (m *mockDriverRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*driver.Driver, error) {
	args := m.Called(ctx, telegramID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*driver.Driver), args.Error(1)
}

func (m *mockDriverRepo) GetActiveAssignment(ctx context.Context, driverID uuid.UUID) (*driver.Assignment, error) {
	args := m.Called(ctx, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*driver.Assignment), args.Error(1)
}

// ─── mock: expense.Service ────────────────────────────────────────────────────

type mockExpenseService struct{ mock.Mock }

func (m *mockExpenseService) Create(ctx context.Context, input expense.CreateInput) (*expense.Expense, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*expense.Expense), args.Error(1)
}

func (m *mockExpenseService) Confirm(ctx context.Context, id, driverID uuid.UUID) error {
	args := m.Called(ctx, id, driverID)
	return args.Error(0)
}

func (m *mockExpenseService) Approve(ctx context.Context, id, ownerID uuid.UUID) error {
	args := m.Called(ctx, id, ownerID)
	return args.Error(0)
}

func (m *mockExpenseService) Reject(ctx context.Context, id, ownerID uuid.UUID, reason string) error {
	args := m.Called(ctx, id, ownerID, reason)
	return args.Error(0)
}

func (m *mockExpenseService) List(ctx context.Context, filter expense.ListFilter) ([]*expense.Expense, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*expense.Expense), args.Error(1)
}

func (m *mockExpenseService) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*expense.Expense, error) {
	args := m.Called(ctx, id, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*expense.Expense), args.Error(1)
}

func (m *mockExpenseService) UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error {
	args := m.Called(ctx, id, amount)
	return args.Error(0)
}

func (m *mockExpenseService) SumByTaxi(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*expense.TaxiSummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*expense.TaxiSummary), args.Error(1)
}

func (m *mockExpenseService) SumByDriver(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*expense.DriverSummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*expense.DriverSummary), args.Error(1)
}

func (m *mockExpenseService) SumByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*expense.CategorySummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*expense.CategorySummary), args.Error(1)
}

// ─── mock: auth.TokenIssuer ───────────────────────────────────────────────────

type mockTokenIssuer struct{ mock.Mock }

func (m *mockTokenIssuer) Issue(claims auth.Claims, ttl time.Duration) (string, error) {
	args := m.Called(claims, ttl)
	return args.String(0), args.Error(1)
}

// ─── mock: receipt.Repository ─────────────────────────────────────────────────

type mockReceiptRepo struct{ mock.Mock }

func (m *mockReceiptRepo) Create(ctx context.Context, r *receipt.Receipt) (*receipt.Receipt, error) {
	args := m.Called(ctx, r)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*receipt.Receipt), args.Error(1)
}

func (m *mockReceiptRepo) GetByID(ctx context.Context, id uuid.UUID) (*receipt.Receipt, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*receipt.Receipt), args.Error(1)
}

func (m *mockReceiptRepo) ListPendingOCR(ctx context.Context) ([]*receipt.Receipt, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*receipt.Receipt), args.Error(1)
}

func (m *mockReceiptRepo) UpdateOCRFields(ctx context.Context, id uuid.UUID, result *receipt.OCRResult) error {
	args := m.Called(ctx, id, result)
	return args.Error(0)
}

func (m *mockReceiptRepo) SetOCRStatus(ctx context.Context, id uuid.UUID, status receipt.OCRStatus, rawJSON []byte) error {
	args := m.Called(ctx, id, status, rawJSON)
	return args.Error(0)
}

// ─── fake tele.Context ────────────────────────────────────────────────────────

// fakeCtx is a minimal stub of tele.Context used in tests.
// It records calls to Send and Respond and provides configurable returns for
// Sender(), Text(), Data(), and Message().
type fakeCtx struct {
	sender  *tele.User
	text    string
	data    string
	message *tele.Message

	sentMsgs   []interface{}
	sentOpts   [][]interface{}
	respondErr error
}

func (f *fakeCtx) Bot() *tele.Bot                                    { return nil }
func (f *fakeCtx) Update() tele.Update                               { return tele.Update{} }
func (f *fakeCtx) Callback() *tele.Callback                         { return nil }
func (f *fakeCtx) Query() *tele.Query                                { return nil }
func (f *fakeCtx) InlineResult() *tele.InlineResult                 { return nil }
func (f *fakeCtx) ShippingQuery() *tele.ShippingQuery               { return nil }
func (f *fakeCtx) PreCheckoutQuery() *tele.PreCheckoutQuery         { return nil }
func (f *fakeCtx) Poll() *tele.Poll                                  { return nil }
func (f *fakeCtx) PollAnswer() *tele.PollAnswer                     { return nil }
func (f *fakeCtx) ChatMember() *tele.ChatMemberUpdate               { return nil }
func (f *fakeCtx) ChatJoinRequest() *tele.ChatJoinRequest           { return nil }
func (f *fakeCtx) Migration() (int64, int64)                        { return 0, 0 }
func (f *fakeCtx) Topic() *tele.Topic                               { return nil }
func (f *fakeCtx) Boost() *tele.BoostUpdated                        { return nil }
func (f *fakeCtx) BoostRemoved() *tele.BoostRemoved                 { return nil }
func (f *fakeCtx) Chat() *tele.Chat                                  { return nil }
func (f *fakeCtx) Recipient() tele.Recipient                        { return f.sender }
func (f *fakeCtx) Entities() tele.Entities                          { return nil }
func (f *fakeCtx) Args() []string                                    { return nil }
func (f *fakeCtx) SendAlbum(a tele.Album, opts ...interface{}) error { return nil }
func (f *fakeCtx) Reply(what interface{}, opts ...interface{}) error { return nil }
func (f *fakeCtx) Forward(msg tele.Editable, opts ...interface{}) error { return nil }
func (f *fakeCtx) ForwardTo(to tele.Recipient, opts ...interface{}) error { return nil }
func (f *fakeCtx) Edit(what interface{}, opts ...interface{}) error  { return nil }
func (f *fakeCtx) EditCaption(caption string, opts ...interface{}) error { return nil }
func (f *fakeCtx) EditOrSend(what interface{}, opts ...interface{}) error { return nil }
func (f *fakeCtx) EditOrReply(what interface{}, opts ...interface{}) error { return nil }
func (f *fakeCtx) Delete() error                                     { return nil }
func (f *fakeCtx) DeleteAfter(d time.Duration) *time.Timer          { return nil }
func (f *fakeCtx) Notify(action tele.ChatAction) error               { return nil }
func (f *fakeCtx) Ship(what ...interface{}) error                    { return nil }
func (f *fakeCtx) Accept(errorMessage ...string) error               { return nil }
func (f *fakeCtx) Answer(resp *tele.QueryResponse) error             { return nil }
func (f *fakeCtx) RespondText(text string) error                     { return nil }
func (f *fakeCtx) RespondAlert(text string) error                    { return nil }
func (f *fakeCtx) Get(key string) interface{}                        { return nil }
func (f *fakeCtx) Set(key string, val interface{})                   {}

func (f *fakeCtx) Sender() *tele.User { return f.sender }
func (f *fakeCtx) Text() string       { return f.text }
func (f *fakeCtx) Data() string       { return f.data }
func (f *fakeCtx) Message() *tele.Message {
	if f.message != nil {
		return f.message
	}
	return &tele.Message{Text: f.text}
}

func (f *fakeCtx) Send(what interface{}, opts ...interface{}) error {
	f.sentMsgs = append(f.sentMsgs, what)
	f.sentOpts = append(f.sentOpts, opts)
	return nil
}

func (f *fakeCtx) Respond(resp ...*tele.CallbackResponse) error {
	return f.respondErr
}

// sentTexts returns the string payloads of all Send calls in order.
func (f *fakeCtx) sentTexts() []string {
	out := make([]string, 0, len(f.sentMsgs))
	for _, m := range f.sentMsgs {
		if s, ok := m.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func newTestBot(sender Sender, svc Services) *Bot {
	return newBotWithSender(sender, svc)
}

func userWithID(id int64) *tele.User {
	return &tele.User{ID: id}
}

// makeBot builds a Bot with mocked services and a mock Sender.
// The mock Sender swallows all Send calls by default (returns nil).
func makeBot(
	tokenIssuer *mockTokenIssuer,
	driverSvc *mockDriverService,
	driverRepo *mockDriverRepo,
	expenseSvc *mockExpenseService,
	receiptRepo *mockReceiptRepo,
) *Bot {
	s := &mockSender{}
	s.On("Send", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	svc := Services{
		Auth:       tokenIssuer,
		Driver:     driverSvc,
		DriverRepo: driverRepo,
		Expense:    expenseSvc,
		Receipt:    receiptRepo,
	}
	return newTestBot(s, svc)
}

// ─── /start ───────────────────────────────────────────────────────────────────

func TestHandleStart_KnownDriver_IssuesJWT(t *testing.T) {
	driverID := uuid.New()
	ownerID := uuid.New()
	const telegramID = int64(111)

	drv := &driver.Driver{
		ID:       driverID,
		OwnerID:  ownerID,
		FullName: "Pedro López",
		Active:   true,
	}

	repo := &mockDriverRepo{}
	repo.On("GetByTelegramID", mock.Anything, telegramID).Return(drv, nil)

	issuer := &mockTokenIssuer{}
	issuer.On("Issue", mock.MatchedBy(func(c auth.Claims) bool {
		return c.UserID == driverID && c.OwnerID == ownerID
	}), jwtTTL).Return("signed.jwt.token", nil)

	b := makeBot(issuer, &mockDriverService{}, repo, &mockExpenseService{}, &mockReceiptRepo{})

	ctx := &fakeCtx{sender: userWithID(telegramID), text: "/start"}
	err := b.handleStart(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "Bienvenido")
	assert.Contains(t, texts[0], drv.FullName)

	// JWT claims should be stored in the FSM.
	cs := b.states.get(telegramID)
	require.NotNil(t, cs.Claims)
	assert.Equal(t, driverID, cs.Claims.DriverID)
	assert.Equal(t, "signed.jwt.token", cs.Claims.Token)

	repo.AssertExpectations(t)
	issuer.AssertExpectations(t)
}

func TestHandleStart_UnknownDriver_PromptsLink(t *testing.T) {
	const telegramID = int64(222)

	repo := &mockDriverRepo{}
	repo.On("GetByTelegramID", mock.Anything, telegramID).Return(nil, driver.ErrNotFound)

	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, repo, &mockExpenseService{}, &mockReceiptRepo{})

	ctx := &fakeCtx{sender: userWithID(telegramID), text: "/start"}
	err := b.handleStart(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "enlace de activación")
	assert.Nil(t, b.states.get(telegramID).Claims)

	repo.AssertExpectations(t)
}

func TestHandleStart_InactiveDriver_Rejects(t *testing.T) {
	const telegramID = int64(333)

	drv := &driver.Driver{
		ID:      uuid.New(),
		OwnerID: uuid.New(),
		Active:  false,
	}
	repo := &mockDriverRepo{}
	repo.On("GetByTelegramID", mock.Anything, telegramID).Return(drv, nil)

	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, repo, &mockExpenseService{}, &mockReceiptRepo{})

	ctx := &fakeCtx{sender: userWithID(telegramID), text: "/start"}
	err := b.handleStart(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "inactiva")
	assert.Nil(t, b.states.get(telegramID).Claims)

	repo.AssertExpectations(t)
}

// ─── /gasto ───────────────────────────────────────────────────────────────────

func TestHandleGasto_NoTaxiAssignment_Rejects(t *testing.T) {
	const telegramID = int64(444)
	driverID := uuid.New()

	// Seed an authenticated FSM state.
	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, &mockDriverRepo{}, &mockExpenseService{}, &mockReceiptRepo{})
	b.states.set(telegramID, &ConversationState{
		State: StateIdle,
		Claims: &botClaims{
			DriverID: driverID,
			OwnerID:  uuid.New(),
		},
	})

	// Repo returns error — no active assignment.
	repo := &mockDriverRepo{}
	repo.On("GetActiveAssignment", mock.Anything, driverID).Return(nil, driver.ErrNotFound)
	b.services.DriverRepo = repo

	ctx := &fakeCtx{sender: userWithID(telegramID), text: "/gasto"}
	err := b.handleGasto(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "taxis asignados")

	// FSM state must NOT have advanced.
	cs := b.states.get(telegramID)
	assert.Equal(t, StateIdle, cs.State)
	assert.Nil(t, cs.SelectedTaxiID)

	repo.AssertExpectations(t)
}

func TestHandleGasto_SingleTaxi_AutoSelects(t *testing.T) {
	const telegramID = int64(555)
	driverID := uuid.New()
	ownerID := uuid.New()
	taxiID := uuid.New()
	catID := uuid.New()

	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, &mockDriverRepo{}, &mockExpenseService{}, &mockReceiptRepo{})
	b.states.set(telegramID, &ConversationState{
		State: StateIdle,
		Claims: &botClaims{
			DriverID: driverID,
			OwnerID:  ownerID,
		},
	})

	repo := &mockDriverRepo{}
	repo.On("GetActiveAssignment", mock.Anything, driverID).Return(&driver.Assignment{
		ID:       uuid.New(),
		DriverID: driverID,
		TaxiID:   taxiID,
	}, nil)
	b.services.DriverRepo = repo

	// Use a mockExpenseService that also implements categoryLister.
	expSvc := &mockExpenseServiceWithCategories{}
	expSvc.On("ListCategories", mock.Anything, ownerID).Return([]expenseCategory{
		{ID: catID, Name: "Combustible"},
	}, nil)
	b.services.Expense = expSvc

	ctx := &fakeCtx{sender: userWithID(telegramID), text: "/gasto"}
	err := b.handleGasto(ctx)
	require.NoError(t, err)

	cs := b.states.get(telegramID)
	require.NotNil(t, cs.SelectedTaxiID, "taxi should be auto-selected")
	assert.Equal(t, taxiID, *cs.SelectedTaxiID)
	assert.Equal(t, StateAwaitingCategorySelection, cs.State)

	// Should have sent the category selection prompt.
	texts := ctx.sentTexts()
	require.NotEmpty(t, texts)
	assert.Contains(t, texts[0], "categoría")

	repo.AssertExpectations(t)
	expSvc.AssertExpectations(t)
}

func TestHandleGasto_Unauthenticated_PromptsStart(t *testing.T) {
	const telegramID = int64(777)

	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, &mockDriverRepo{}, &mockExpenseService{}, &mockReceiptRepo{})
	// No FSM state seeded — claims will be nil.

	ctx := &fakeCtx{sender: userWithID(telegramID), text: "/gasto"}
	err := b.handleGasto(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "/start")
}

// TestHandleGasto_ManualAmount_CreatesExpense exercises the full manual-amount
// path: user is authenticated with a taxi + category already selected and types
// an amount. The handler must create a receipt, create an expense, update its
// amount, and confirm it.
func TestHandleGasto_ManualAmount_CreatesExpense(t *testing.T) {
	const telegramID = int64(666)
	driverID := uuid.New()
	ownerID := uuid.New()
	taxiID := uuid.New()
	catID := uuid.New()
	receiptID := uuid.New()
	expenseID := uuid.New()
	amount := decimal.NewFromInt(50000)

	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, &mockDriverRepo{}, &mockExpenseService{}, &mockReceiptRepo{})
	b.states.set(telegramID, &ConversationState{
		State: StateAwaitingReceiptPhoto, // manual entry is valid in this state too
		Claims: &botClaims{
			DriverID: driverID,
			OwnerID:  ownerID,
		},
		SelectedTaxiID:     &taxiID,
		SelectedCategoryID: &catID,
	})

	rcptRepo := &mockReceiptRepo{}
	rcptRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *receipt.Receipt) bool {
		return r.DriverID == driverID && r.TaxiID == taxiID && r.OCRStatus == receipt.OCRStatusSkipped
	})).Return(&receipt.Receipt{ID: receiptID}, nil)
	b.services.Receipt = rcptRepo

	expSvc := &mockExpenseService{}
	expSvc.On("Create", mock.Anything, mock.MatchedBy(func(inp expense.CreateInput) bool {
		return inp.DriverID == driverID && inp.TaxiID == taxiID && inp.CategoryID == catID && inp.ReceiptID == receiptID
	})).Return(&expense.Expense{ID: expenseID}, nil)
	expSvc.On("UpdateAmount", mock.Anything, expenseID, amount).Return(nil)
	expSvc.On("Confirm", mock.Anything, expenseID, driverID).Return(nil)
	b.services.Expense = expSvc

	ctx := &fakeCtx{sender: userWithID(telegramID), text: "50000"}
	err := b.handleText(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "registrado")

	// FSM should be reset.
	cs := b.states.get(telegramID)
	assert.Equal(t, StateIdle, cs.State)
	assert.Nil(t, cs.Claims)

	rcptRepo.AssertExpectations(t)
	expSvc.AssertExpectations(t)
}

// ─── /estado ──────────────────────────────────────────────────────────────────

func TestHandleEstado_ReturnsFormattedList(t *testing.T) {
	const telegramID = int64(888)
	driverID := uuid.New()
	ownerID := uuid.New()
	amt := decimal.NewFromInt(30000)
	now := time.Now()

	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, &mockDriverRepo{}, &mockExpenseService{}, &mockReceiptRepo{})
	b.states.set(telegramID, &ConversationState{
		State: StateIdle,
		Claims: &botClaims{
			DriverID: driverID,
			OwnerID:  ownerID,
		},
	})

	expSvc := &mockExpenseService{}
	expSvc.On("List", mock.Anything, expense.ListFilter{
		OwnerID:  ownerID,
		DriverID: &driverID,
		Limit:    10,
	}).Return([]*expense.Expense{
		{ID: uuid.New(), OwnerID: ownerID, DriverID: driverID, Amount: &amt, Status: expense.StatusConfirmed, CreatedAt: now},
		{ID: uuid.New(), OwnerID: ownerID, DriverID: driverID, Amount: &amt, Status: expense.StatusPending, CreatedAt: now},
	}, nil)
	b.services.Expense = expSvc

	ctx := &fakeCtx{sender: userWithID(telegramID)}
	err := b.handleEstado(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "últimos gastos")
	assert.Contains(t, texts[0], "$30000")
	assert.Contains(t, texts[0], string(expense.StatusConfirmed))

	expSvc.AssertExpectations(t)
}

func TestHandleEstado_Empty_ReturnsNoExpensesMsg(t *testing.T) {
	const telegramID = int64(999)
	driverID := uuid.New()
	ownerID := uuid.New()

	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, &mockDriverRepo{}, &mockExpenseService{}, &mockReceiptRepo{})
	b.states.set(telegramID, &ConversationState{
		State: StateIdle,
		Claims: &botClaims{DriverID: driverID, OwnerID: ownerID},
	})

	expSvc := &mockExpenseService{}
	expSvc.On("List", mock.Anything, mock.Anything).Return([]*expense.Expense{}, nil)
	b.services.Expense = expSvc

	ctx := &fakeCtx{sender: userWithID(telegramID)}
	err := b.handleEstado(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "No tenés gastos")
}

func TestHandleEstado_Unauthenticated_PromptsStart(t *testing.T) {
	const telegramID = int64(1001)

	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, &mockDriverRepo{}, &mockExpenseService{}, &mockReceiptRepo{})
	// No FSM state — claims are nil.

	ctx := &fakeCtx{sender: userWithID(telegramID)}
	err := b.handleEstado(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "/start")
}

// ─── NotifyOCRResult ──────────────────────────────────────────────────────────

func TestNotifyOCRResult_Success_SendsConfirmKeyboard(t *testing.T) {
	const telegramID = int64(1100)
	driverID := uuid.New()
	expenseID := uuid.New()
	receiptID := uuid.New()
	vendor := "TaxiSupplies SA"
	total := "50000"

	sender := &mockSender{}
	// Capture any send to telegramUser{1100}
	sender.On("Send", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	b := newTestBot(sender, Services{})
	b.states.set(telegramID, &ConversationState{
		State:            StateAwaitingOCRConfirmation,
		Claims:           &botClaims{DriverID: driverID},
		PendingExpenseID: &expenseID,
	})

	ocrResult := &receipt.OCRResult{
		Vendor: &vendor,
		Total:  &total,
	}

	err := b.NotifyOCRResult(context.Background(), telegramID, receiptID, ocrResult)
	require.NoError(t, err)

	sender.AssertCalled(t, "Send", mock.Anything, mock.Anything, mock.Anything)

	// Verify the message text contains extracted fields.
	require.Len(t, sender.sent, 1)
	msgText, ok := sender.sent[0].what.(string)
	require.True(t, ok)
	assert.Contains(t, msgText, "Factura procesada")
	assert.Contains(t, msgText, vendor)
	assert.Contains(t, msgText, total)

	// Verify an inline keyboard was attached as an option.
	opts := sender.sent[0].opts
	require.NotEmpty(t, opts)
	hasKeyboard := false
	for _, o := range opts {
		if _, ok := o.(*tele.ReplyMarkup); ok {
			hasKeyboard = true
		}
	}
	assert.True(t, hasKeyboard, "expected inline keyboard in Send opts")
}

func TestNotifyOCRResult_Failed_PromptsManualEntry(t *testing.T) {
	const telegramID = int64(1200)
	driverID := uuid.New()
	expenseID := uuid.New()
	receiptID := uuid.New()

	sender := &mockSender{}
	sender.On("Send", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	b := newTestBot(sender, Services{})
	b.states.set(telegramID, &ConversationState{
		State:            StateAwaitingOCRConfirmation,
		Claims:           &botClaims{DriverID: driverID},
		PendingExpenseID: &expenseID,
	})

	// nil result = OCR failure
	err := b.NotifyOCRResult(context.Background(), telegramID, receiptID, nil)
	require.NoError(t, err)

	sender.AssertCalled(t, "Send", mock.Anything, mock.Anything, mock.Anything)
	require.Len(t, sender.sent, 1)
	msgText, ok := sender.sent[0].what.(string)
	require.True(t, ok)
	assert.True(t, strings.Contains(msgText, "monto") || strings.Contains(msgText, "manual"),
		"expected manual entry prompt, got: %q", msgText)

	// FSM state must advance to StateAwaitingManualAmount.
	cs := b.states.get(telegramID)
	assert.Equal(t, StateAwaitingManualAmount, cs.State)
}

// ─── helpers for extended mock ────────────────────────────────────────────────

// mockExpenseServiceWithCategories extends mockExpenseService with ListCategories.
type mockExpenseServiceWithCategories struct {
	mockExpenseService
}

func (m *mockExpenseServiceWithCategories) ListCategories(ctx context.Context, ownerID uuid.UUID) ([]expenseCategory, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]expenseCategory), args.Error(1)
}

// ─── mock: StorageClient ──────────────────────────────────────────────────────

type mockStorageClient struct{ mock.Mock }

func (m *mockStorageClient) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	args := m.Called(ctx, key, data, contentType)
	return args.String(0), args.Error(1)
}

func (m *mockStorageClient) Download(ctx context.Context, url string) ([]byte, error) {
	args := m.Called(ctx, url)
	res, _ := args.Get(0).([]byte)
	return res, args.Error(1)
}

// ─── fakeCtxWithBot ───────────────────────────────────────────────────────────

// fakeCtxWithBot extends fakeCtx with a configurable *tele.Bot return.
// This allows tests to inject a fake Telegram bot so that handlers that call
// c.Bot() (e.g. handlePhoto → downloadPhoto) receive a non-nil bot.
type fakeCtxWithBot struct {
	fakeCtx
	teleBot *tele.Bot
}

func (f *fakeCtxWithBot) Bot() *tele.Bot { return f.teleBot }

// ─── /handlePhoto ─────────────────────────────────────────────────────────────

// TestHandleGasto_PhotoReceived_CreatesReceiptAndExpense exercises the photo
// upload path: FSM is in StateAwaitingReceiptPhoto with a taxi and category
// already selected. A photo message triggers Storage.Upload, receipt.Create,
// and expense.Create in sequence. The handler must transition the FSM to
// StateAwaitingOCRConfirmation and send a confirmation message.
func TestHandleGasto_PhotoReceived_CreatesReceiptAndExpense(t *testing.T) {
	const telegramID = int64(7001)
	driverID := uuid.New()
	ownerID := uuid.New()
	taxiID := uuid.New()
	catID := uuid.New()
	receiptID := uuid.New()
	expenseID := uuid.New()

	const fakeFileID = "file-abc123"
	const fakeStorageURL = "https://storage.example.com/receipts/test.jpg"

	// Build a minimal mock HTTP server that responds to Telegram API calls.
	// handlePhoto calls c.Bot().FileByID (POST /bot<token>/getFile) and then
	// c.Bot().File (GET /file/bot<token>/<path>).
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/getFile"):
			// Respond with a valid File object.
			resp := map[string]interface{}{
				"ok": true,
				"result": map[string]interface{}{
					"file_id":   fakeFileID,
					"file_size": 100,
					"file_path": "photos/test.jpg",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			// Respond with minimal JPEG bytes for the file download.
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF, 0xD9}) // minimal JPEG
		}
	})
	fakeSrv := httptest.NewServer(mux)
	defer fakeSrv.Close()

	teleBot, err := tele.NewBot(tele.Settings{
		Token:   "testtoken",
		Offline: true,
		URL:     fakeSrv.URL,
		Client:  fakeSrv.Client(),
	})
	require.NoError(t, err)

	// Seed FSM state.
	b := makeBot(&mockTokenIssuer{}, &mockDriverService{}, &mockDriverRepo{}, &mockExpenseService{}, &mockReceiptRepo{})
	b.states.set(telegramID, &ConversationState{
		State: StateAwaitingReceiptPhoto,
		Claims: &botClaims{
			DriverID: driverID,
			OwnerID:  ownerID,
		},
		SelectedTaxiID:     &taxiID,
		SelectedCategoryID: &catID,
	})

	// Wire mocked services.
	storageMock := &mockStorageClient{}
	storageMock.On("Upload", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "receipts/")
	}), mock.Anything, "image/jpeg").Return(fakeStorageURL, nil)
	b.services.Storage = storageMock

	rcptRepo := &mockReceiptRepo{}
	rcptRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *receipt.Receipt) bool {
		return r.DriverID == driverID &&
			r.TaxiID == taxiID &&
			r.StorageURL == fakeStorageURL &&
			r.OCRStatus == receipt.OCRStatusPending
	})).Return(&receipt.Receipt{ID: receiptID}, nil)
	b.services.Receipt = rcptRepo

	expSvc := &mockExpenseService{}
	expSvc.On("Create", mock.Anything, mock.MatchedBy(func(inp expense.CreateInput) bool {
		return inp.DriverID == driverID &&
			inp.TaxiID == taxiID &&
			inp.CategoryID == catID &&
			inp.ReceiptID == receiptID
	})).Return(&expense.Expense{ID: expenseID}, nil)
	b.services.Expense = expSvc

	// Build the fake context with a photo.
	photo := &tele.Photo{File: tele.File{FileID: fakeFileID, FileSize: 100}}
	ctx := &fakeCtxWithBot{
		fakeCtx: fakeCtx{
			sender: userWithID(telegramID),
			message: &tele.Message{
				Sender: userWithID(telegramID),
				Photo:  photo,
			},
		},
		teleBot: teleBot,
	}

	err = b.handlePhoto(ctx)
	require.NoError(t, err)

	texts := ctx.sentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "Recibo registrado")

	// FSM must advance to StateAwaitingOCRConfirmation.
	cs := b.states.get(telegramID)
	assert.Equal(t, StateAwaitingOCRConfirmation, cs.State)
	require.NotNil(t, cs.PendingReceiptID)
	assert.Equal(t, receiptID, *cs.PendingReceiptID)
	require.NotNil(t, cs.PendingExpenseID)
	assert.Equal(t, expenseID, *cs.PendingExpenseID)

	storageMock.AssertExpectations(t)
	rcptRepo.AssertExpectations(t)
	expSvc.AssertExpectations(t)
}
