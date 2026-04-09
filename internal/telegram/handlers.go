package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	tele "gopkg.in/telebot.v3"
)

// Callback unique IDs for inline keyboards.
const (
	callbackSelectTaxi     = "select_taxi"
	callbackSelectCategory = "select_category"
	callbackConfirmOCR     = "confirm_ocr"
	callbackEditAmount     = "edit_amount"
)

// ─── /start ───────────────────────────────────────────────────────────────────

// handleStart processes the /start command.
// REQ-DRV-02: link token flow; REQ-DRV-03: issue JWT.
func (b *Bot) handleStart(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID

	// Check whether a link token was passed as the /start parameter.
	parts := strings.Fields(c.Text())
	if len(parts) > 1 {
		token := parts[1]
		return b.handleStartWithToken(ctx, c, telegramID, token)
	}

	return b.handleStartLookup(ctx, c, telegramID)
}

// handleStartLookup resolves a driver by their Telegram ID and issues a JWT.
func (b *Bot) handleStartLookup(ctx context.Context, c tele.Context, telegramID int64) error {
	drv, err := b.services.DriverRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, driver.ErrNotFound) {
			return c.Send("Tu cuenta no está vinculada. Pedí al administrador un enlace de activación y usá /start <token>.")
		}
		return c.Send("Error interno. Intentá de nuevo más tarde.")
	}

	if !drv.Active {
		return c.Send("Tu cuenta está inactiva. Contactá al administrador.")
	}

	return b.issueJWTAndWelcome(ctx, c, drv)
}

// handleStartWithToken attempts to link a Telegram ID to a driver using the token.
func (b *Bot) handleStartWithToken(ctx context.Context, c tele.Context, telegramID int64, token string) error {
	if err := b.services.Driver.LinkTelegramID(ctx, token, telegramID); err != nil {
		switch {
		case errors.Is(err, driver.ErrLinkTokenExpired):
			return c.Send("El enlace de activación ha expirado. Pedí uno nuevo al administrador.")
		case errors.Is(err, driver.ErrLinkTokenUsed):
			return c.Send("Este enlace ya fue utilizado. Intentá con /start si ya tenés una cuenta vinculada.")
		case errors.Is(err, driver.ErrDuplicateTelegram):
			return c.Send("Este Telegram ya está vinculado a otra cuenta.")
		default:
			return c.Send("No se pudo vincular tu cuenta. Verificá el enlace o pedí uno nuevo.")
		}
	}

	// After linking, look up the driver to get full info for JWT issuance.
	drv, err := b.services.DriverRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return c.Send("Cuenta vinculada, pero no se pudo cargar tu perfil. Intentá /start.")
	}
	if !drv.Active {
		return c.Send("Tu cuenta está inactiva. Contactá al administrador.")
	}
	return b.issueJWTAndWelcome(ctx, c, drv)
}

// issueJWTAndWelcome issues a JWT and stores claims in the FSM, then greets the driver.
func (b *Bot) issueJWTAndWelcome(_ context.Context, c tele.Context, drv *driver.Driver) error {
	claims := auth.Claims{
		UserID:   drv.ID,
		Role:     auth.RoleDriver,
		OwnerID:  drv.OwnerID,
		DriverID: &drv.ID,
	}
	token, err := b.services.Auth.Issue(claims, jwtTTL)
	if err != nil {
		return c.Send("Error al generar tu sesión. Intentá de nuevo.")
	}

	cs := &ConversationState{
		State: StateIdle,
		Claims: &botClaims{
			DriverID: drv.ID,
			OwnerID:  drv.OwnerID,
			UserID:   drv.ID,
			Token:    token,
		},
	}
	b.states.set(c.Sender().ID, cs)

	return c.Send(fmt.Sprintf("Bienvenido, %s! Usá /gasto para registrar un gasto.", drv.FullName))
}

// ─── /gasto ───────────────────────────────────────────────────────────────────

// handleGasto initiates the multi-step expense registration flow.
// REQ-EXP-01: starts flow; checks active taxi assignment.
func (b *Bot) handleGasto(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID

	cs := b.states.get(telegramID)
	if cs.Claims == nil {
		return c.Send("Necesitás iniciar sesión primero. Usá /start.")
	}

	taxiIDs, err := b.getDriverTaxiIDs(ctx, cs.Claims.DriverID)
	if err != nil || len(taxiIDs) == 0 {
		return c.Send("No tenés taxis asignados. Contactá al administrador.")
	}

	if len(taxiIDs) == 1 {
		taxiID := taxiIDs[0]
		cs.SelectedTaxiID = &taxiID
		cs.State = StateAwaitingCategorySelection
		b.states.set(telegramID, cs)
		return b.promptCategorySelection(ctx, c, cs)
	}

	// Multiple taxis — show inline keyboard.
	cs.State = StateAwaitingTaxiSelection
	b.states.set(telegramID, cs)
	return b.promptTaxiSelection(c, taxiIDs)
}

// promptTaxiSelection sends an inline keyboard for taxi selection.
func (b *Bot) promptTaxiSelection(c tele.Context, taxiIDs []uuid.UUID) error {
	rows := make([]tele.Row, 0, len(taxiIDs))
	for _, id := range taxiIDs {
		idCopy := id
		btn := tele.Btn{
			Unique: callbackSelectTaxi,
			Text:   idCopy.String()[:8],
			Data:   idCopy.String(),
		}
		rows = append(rows, tele.Row{btn})
	}
	kb := &tele.ReplyMarkup{}
	kb.Inline(rows...)
	return c.Send("Seleccioná el taxi:", kb)
}

// handleTaxiSelection processes inline keyboard taxi selection.
func (b *Bot) handleTaxiSelection(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)
	if cs.Claims == nil || cs.State != StateAwaitingTaxiSelection {
		return c.Respond()
	}

	taxiID, err := uuid.Parse(c.Data())
	if err != nil {
		return c.Send("Selección inválida. Usá /gasto para reiniciar.")
	}

	cs.SelectedTaxiID = &taxiID
	cs.State = StateAwaitingCategorySelection
	b.states.set(telegramID, cs)
	_ = c.Respond()
	return b.promptCategorySelection(ctx, c, cs)
}

// promptCategorySelection sends an inline keyboard for category selection.
func (b *Bot) promptCategorySelection(ctx context.Context, c tele.Context, cs *ConversationState) error {
	categories, err := b.getExpenseCategories(ctx, cs.Claims.OwnerID)
	if err != nil || len(categories) == 0 {
		return c.Send("No hay categorías disponibles. Contactá al administrador.")
	}

	rows := make([]tele.Row, 0, len(categories))
	for _, cat := range categories {
		catCopy := cat
		btn := tele.Btn{
			Unique: callbackSelectCategory,
			Text:   catCopy.Name,
			Data:   catCopy.ID.String(),
		}
		rows = append(rows, tele.Row{btn})
	}
	kb := &tele.ReplyMarkup{}
	kb.Inline(rows...)
	return c.Send("Seleccioná la categoría del gasto:", kb)
}

// handleCategorySelection processes inline keyboard category selection.
func (b *Bot) handleCategorySelection(c tele.Context) error {
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)
	if cs.Claims == nil || cs.State != StateAwaitingCategorySelection {
		return c.Respond()
	}

	catID, err := uuid.Parse(c.Data())
	if err != nil {
		return c.Send("Categoría inválida. Usá /gasto para reiniciar.")
	}

	cs.SelectedCategoryID = &catID
	cs.State = StateAwaitingReceiptPhoto
	b.states.set(telegramID, cs)
	_ = c.Respond()
	return c.Send("Enviá una foto de la factura o escribí el monto manualmente (en pesos COP).")
}

// handlePhoto processes a receipt photo from the driver.
// REQ-EXP-03, REQ-FRD-02: upload to storage BEFORE any DB write.
func (b *Bot) handlePhoto(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)

	if cs.Claims == nil || cs.State != StateAwaitingReceiptPhoto {
		return nil
	}

	photo := c.Message().Photo
	if photo == nil {
		return c.Send("No se pudo leer la foto. Intentá de nuevo.")
	}

	photoBytes, fileID, err := b.downloadPhoto(ctx, c, photo)
	if err != nil {
		slog.Error("failed to download photo from telegram", "err", err)
		return c.Send("No se pudo descargar la foto. Intentá de nuevo.")
	}

	// Upload to persistent storage FIRST (REQ-FRD-02).
	storageKey := fmt.Sprintf("receipts/%s/%s.jpg", cs.Claims.DriverID, uuid.New())
	storageURL, err := b.services.Storage.Upload(ctx, storageKey, photoBytes, "image/jpeg")
	if err != nil {
		slog.Error("failed to upload receipt to storage", "err", err)
		return c.Send("No se pudo guardar la foto. Intentá de nuevo.")
	}

	// Create receipt record with ocr_status=pending.
	r := &receipt.Receipt{
		DriverID:       cs.Claims.DriverID,
		TaxiID:         *cs.SelectedTaxiID,
		StorageURL:     storageURL,
		TelegramFileID: fileID,
		OCRStatus:      receipt.OCRStatusPending,
	}
	createdReceipt, err := b.services.Receipt.Create(ctx, r)
	if err != nil {
		slog.Error("failed to create receipt record", "err", err)
		return c.Send("Error al registrar el recibo. Intentá de nuevo.")
	}

	// Create expense record with status=pending.
	exp, err := b.services.Expense.Create(ctx, expense.CreateInput{
		OwnerID:    cs.Claims.OwnerID,
		DriverID:   cs.Claims.DriverID,
		TaxiID:     *cs.SelectedTaxiID,
		CategoryID: *cs.SelectedCategoryID,
		ReceiptID:  createdReceipt.ID,
	})
	if err != nil {
		slog.Error("failed to create expense record", "err", err)
		return c.Send("Error al registrar el gasto. Intentá de nuevo.")
	}

	receiptID := createdReceipt.ID
	expenseID := exp.ID
	cs.PendingReceiptID = &receiptID
	cs.PendingExpenseID = &expenseID
	cs.State = StateAwaitingOCRConfirmation
	b.states.set(telegramID, cs)

	return c.Send("Recibo registrado ✓ Procesando factura...")
}

// handleText routes text messages based on FSM state.
func (b *Bot) handleText(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)

	if cs.Claims == nil {
		return nil
	}

	switch cs.State {
	case StateAwaitingReceiptPhoto:
		return b.handleManualAmount(ctx, c, cs)
	case StateAwaitingManualAmount:
		return b.handleManualAmount(ctx, c, cs)
	}
	return nil
}

// handleManualAmount parses a COP amount from text and creates a receipt+expense.
// REQ-EXP-04: even for manual entry, a receipt record MUST be created.
func (b *Bot) handleManualAmount(ctx context.Context, c tele.Context, cs *ConversationState) error {
	telegramID := c.Sender().ID
	amountStr := strings.TrimSpace(c.Text())

	amount, err := decimal.NewFromString(amountStr)
	if err != nil || amount.IsNegative() {
		return c.Send("Monto inválido. Ingresá un número en pesos COP (ej: 50000 o 50000.50).")
	}

	var receiptID uuid.UUID

	if cs.PendingReceiptID != nil {
		// Reuse existing receipt (e.g. OCR failed, now providing manual amount).
		receiptID = *cs.PendingReceiptID
	} else {
		r := &receipt.Receipt{
			DriverID:   cs.Claims.DriverID,
			TaxiID:     *cs.SelectedTaxiID,
			StorageURL: "manual-entry",
			OCRStatus:  receipt.OCRStatusSkipped,
		}
		createdReceipt, err := b.services.Receipt.Create(ctx, r)
		if err != nil {
			return c.Send("Error al registrar el recibo. Intentá de nuevo.")
		}
		receiptID = createdReceipt.ID
	}

	if cs.PendingExpenseID != nil {
		// Update existing expense with corrected amount and confirm.
		if err := b.services.Expense.UpdateAmount(ctx, *cs.PendingExpenseID, amount); err != nil {
			return c.Send("Error al actualizar el gasto. Intentá de nuevo.")
		}
		if err := b.services.Expense.Confirm(ctx, *cs.PendingExpenseID, cs.Claims.DriverID); err != nil {
			return c.Send("Error al confirmar el gasto. Intentá de nuevo.")
		}
	} else {
		exp, err := b.services.Expense.Create(ctx, expense.CreateInput{
			OwnerID:    cs.Claims.OwnerID,
			DriverID:   cs.Claims.DriverID,
			TaxiID:     *cs.SelectedTaxiID,
			CategoryID: *cs.SelectedCategoryID,
			ReceiptID:  receiptID,
		})
		if err != nil {
			return c.Send("Error al registrar el gasto. Intentá de nuevo.")
		}
		if err := b.services.Expense.UpdateAmount(ctx, exp.ID, amount); err != nil {
			return c.Send("Error al actualizar el monto. Intentá de nuevo.")
		}
		if err := b.services.Expense.Confirm(ctx, exp.ID, cs.Claims.DriverID); err != nil {
			return c.Send("Error al confirmar el gasto. Intentá de nuevo.")
		}
	}

	b.states.reset(telegramID)
	return c.Send("Gasto registrado ✓")
}

// handleConfirmOCR processes the "Confirmar" callback.
func (b *Bot) handleConfirmOCR(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)

	if cs.Claims == nil || cs.PendingExpenseID == nil {
		_ = c.Respond()
		return c.Send("No hay gasto pendiente de confirmación.")
	}

	if err := b.services.Expense.Confirm(ctx, *cs.PendingExpenseID, cs.Claims.DriverID); err != nil {
		_ = c.Respond()
		return c.Send("No se pudo confirmar el gasto. Intentá de nuevo.")
	}

	b.states.reset(telegramID)
	_ = c.Respond()
	return c.Send("Gasto confirmado ✓")
}

// handleEditAmount processes the "Corregir monto" callback.
func (b *Bot) handleEditAmount(c tele.Context) error {
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)

	if cs.Claims == nil || cs.PendingExpenseID == nil {
		_ = c.Respond()
		return c.Send("No hay gasto pendiente de corrección.")
	}

	cs.State = StateAwaitingManualAmount
	b.states.set(telegramID, cs)
	_ = c.Respond()
	return c.Send("Ingresá el monto correcto en pesos COP:")
}

// ─── /estado ──────────────────────────────────────────────────────────────────

// handleEstado displays the driver's last 10 expenses.
// REQ-EXP-05, REQ-TNT-02.
func (b *Bot) handleEstado(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)

	if cs.Claims == nil {
		return c.Send("Necesitás iniciar sesión primero. Usá /start.")
	}

	driverID := cs.Claims.DriverID
	ownerID := cs.Claims.OwnerID

	expenses, err := b.services.Expense.List(ctx, expense.ListFilter{
		OwnerID:  ownerID,
		DriverID: &driverID,
		Limit:    10,
	})
	if err != nil {
		return c.Send("Error al obtener los gastos. Intentá de nuevo.")
	}

	if len(expenses) == 0 {
		return c.Send("No tenés gastos registrados.")
	}

	var sb strings.Builder
	sb.WriteString("Tus últimos gastos:\n\n")
	for _, exp := range expenses {
		date := exp.CreatedAt.Format("02/01/2006")
		amount := "—"
		if exp.Amount != nil {
			amount = "$" + exp.Amount.StringFixed(0)
		}
		sb.WriteString(fmt.Sprintf("%s | %s | %s\n", date, amount, string(exp.Status)))
	}
	return c.Send(sb.String())
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// expenseCategory is a lightweight view used in category selection UI.
type expenseCategory struct {
	ID   uuid.UUID
	Name string
}

// getDriverTaxiIDs fetches the active taxi IDs for a driver.
func (b *Bot) getDriverTaxiIDs(ctx context.Context, driverID uuid.UUID) ([]uuid.UUID, error) {
	assignment, err := b.services.DriverRepo.GetActiveAssignment(ctx, driverID)
	if err != nil {
		return nil, err
	}
	return []uuid.UUID{assignment.TaxiID}, nil
}

// getExpenseCategories returns the expense categories for an owner.
// Falls back to a stub "General" category if no CategoryLister is available.
func (b *Bot) getExpenseCategories(ctx context.Context, ownerID uuid.UUID) ([]expenseCategory, error) {
	if lister, ok := b.services.Expense.(categoryLister); ok {
		return lister.ListCategories(ctx, ownerID)
	}
	return []expenseCategory{{ID: ownerID, Name: "General"}}, nil
}

// downloadPhoto downloads the highest-resolution Telegram photo and returns its bytes and file ID.
func (b *Bot) downloadPhoto(_ context.Context, c tele.Context, photo *tele.Photo) ([]byte, string, error) {
	fileID := photo.FileID
	file, err := c.Bot().FileByID(fileID)
	if err != nil {
		return nil, "", fmt.Errorf("get file info: %w", err)
	}
	rd, err := c.Bot().File(&file)
	if err != nil {
		return nil, "", fmt.Errorf("open file: %w", err)
	}
	defer rd.Close()

	buf := make([]byte, 0, photo.FileSize)
	tmp := make([]byte, 4096)
	for {
		n, readErr := rd.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			break
		}
	}
	return buf, fileID, nil
}

// categoryLister is an optional extension interface for expense.Service.
type categoryLister interface {
	ListCategories(ctx context.Context, ownerID uuid.UUID) ([]expenseCategory, error)
}
