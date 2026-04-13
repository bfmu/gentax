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
	callbackNewExpense     = "new_expense"
	callbackViewStatus     = "view_status"
	callbackSendEvidence   = "send_evidence"
	callbackOmitir         = "omitir"
	callbackCancelEvidence = "cancel_evidence"
	callbackRetryPhoto     = "retry_photo"
)

// ─── /start ───────────────────────────────────────────────────────────────────

// persistentMenuKeyboard returns the always-visible reply keyboard shown at the bottom of the chat.
func persistentMenuKeyboard() *tele.ReplyMarkup {
	kb := &tele.ReplyMarkup{ResizeKeyboard: true}
	kb.Reply(
		kb.Row(kb.Text("📝 Registrar gasto")),
		kb.Row(kb.Text("📊 Ver mis gastos")),
		kb.Row(kb.Text("📎 Enviar soporte")),
	)
	return kb
}

// handleStart processes the /start command.
// REQ-DRV-02: link token flow; REQ-DRV-03: issue JWT.
func (b *Bot) handleStart(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID

	// If already authenticated, just show the persistent keyboard again.
	cs := b.states.get(telegramID)
	if cs.Claims != nil {
		return c.Send("¿Qué querés hacer?", persistentMenuKeyboard())
	}

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

	return c.Send(fmt.Sprintf("Bienvenido, %s! Elegí una opción:", drv.FullName), persistentMenuKeyboard())
}

// ─── main menu ────────────────────────────────────────────────────────────────

// showMainMenu sends the inline main menu keyboard.
// It conditionally includes the [📎 Enviar soporte] button when the driver has
// at least one needs_evidence expense.
func (b *Bot) showMainMenu(ctx context.Context, c tele.Context, cs *ConversationState) error {
	newExpenseBtn := tele.Btn{Unique: callbackNewExpense, Text: "📝 Registrar gasto"}
	viewStatusBtn := tele.Btn{Unique: callbackViewStatus, Text: "📊 Ver mis gastos"}

	rows := []tele.Row{
		{newExpenseBtn},
		{viewStatusBtn},
	}

	// Only show evidence button if there is a pending expense.
	if cs != nil && cs.Claims != nil {
		driverID := cs.Claims.DriverID
		ownerID := cs.Claims.OwnerID
		pending, err := b.services.Expense.List(ctx, expense.ListFilter{
			OwnerID:  ownerID,
			DriverID: &driverID,
			Statuses: []expense.Status{expense.StatusNeedsEvidence},
			Limit:    1,
		})
		if err == nil && len(pending) > 0 {
			evidenceBtn := tele.Btn{Unique: callbackSendEvidence, Text: "📎 Enviar soporte"}
			rows = append(rows, tele.Row{evidenceBtn})
		}
	}

	kb := &tele.ReplyMarkup{}
	kb.Inline(rows...)
	return c.Send("¿Qué querés hacer?", kb)
}

// omitirDoneKeyboard returns a single-button inline keyboard with "Listo" to finish evidence.
func omitirDoneKeyboard() *tele.ReplyMarkup {
	btn := tele.Btn{Unique: callbackOmitir, Text: "✅ Listo, no agrego más soportes"}
	kb := &tele.ReplyMarkup{}
	kb.Inline(tele.Row{btn})
	return kb
}

// handleCallbackNewExpense handles the [📝 Registrar gasto] button.
func (b *Bot) handleCallbackNewExpense(c tele.Context) error {
	_ = c.Respond()
	return b.handleGasto(c)
}

// handleCallbackViewStatus handles the [📊 Ver mis gastos] button.
func (b *Bot) handleCallbackViewStatus(c tele.Context) error {
	_ = c.Respond()
	return b.handleEstado(c)
}

// handleCallbackSendEvidence handles the [📎 Enviar soporte] button.
func (b *Bot) handleCallbackSendEvidence(c tele.Context) error {
	_ = c.Respond()
	return b.handleSoporte(c)
}

// handleCallbackOmitir handles the [✅ Listo, no agrego más soportes] button.
func (b *Bot) handleCallbackOmitir(c tele.Context) error {
	_ = c.Respond()
	return b.handleOmitir(c)
}

// handleCallbackCancelEvidence handles the [❌ Cancelar] button during evidence flow.
func (b *Bot) handleCallbackCancelEvidence(c tele.Context) error {
	telegramID := c.Sender().ID
	_ = c.Respond()
	b.states.reset(telegramID)
	return c.Send("Cancelado.", persistentMenuKeyboard())
}

// handleCallbackRetryPhoto handles [🔄 Reenviar foto] — lets the driver retry OCR with a new photo.
// Keeps the existing expense but clears PendingReceiptID so handlePhoto creates a fresh receipt.
func (b *Bot) handleCallbackRetryPhoto(c tele.Context) error {
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)
	if cs.Claims == nil || cs.PendingExpenseID == nil {
		_ = c.Respond()
		return c.Send("No hay gasto pendiente. Usá el menú para registrar uno.")
	}
	cs.State = StateAwaitingReceiptPhoto
	cs.PendingReceiptID = nil
	b.states.set(telegramID, cs)
	_ = c.Respond()
	return c.Send("Enviá una foto más clara de la factura:")
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

// ─── /soporte ─────────────────────────────────────────────────────────────────

// handleSoporte allows a driver to submit evidence for a needs_evidence expense.
func (b *Bot) handleSoporte(c tele.Context) error {
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
		Statuses: []expense.Status{expense.StatusNeedsEvidence},
		Limit:    1,
	})
	if err != nil || len(expenses) == 0 {
		return c.Send("No tenés gastos pendientes de evidencia.")
	}

	exp := expenses[0]
	expID := exp.ID
	taxiID := exp.TaxiID
	cs.PendingExpenseID = &expID
	cs.SelectedTaxiID = &taxiID
	cs.State = StateAwaitingEvidencePhoto
	b.states.set(telegramID, cs)

	msg := "Enviá la foto del comprobante para el gasto pendiente de evidencia."
	if exp.RejectionReason != "" {
		msg = fmt.Sprintf("El administrador solicita más evidencia:\n\n%s\n\nEnviá la foto del comprobante.", exp.RejectionReason)
	}

	cancelBtn := tele.Btn{Unique: callbackCancelEvidence, Text: "❌ Cancelar"}
	kb := &tele.ReplyMarkup{}
	kb.Inline(tele.Row{cancelBtn})
	return c.Send(msg, kb)
}

// handlePhoto processes a receipt photo from the driver.
// REQ-EXP-03, REQ-FRD-02: upload to storage BEFORE any DB write.
func (b *Bot) handlePhoto(c tele.Context) error {
	ctx := context.Background()
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)

	if cs.Claims == nil {
		return c.Send("No estás autenticado. Enviá /start para iniciar sesión.")
	}

	if cs.State == StateAwaitingEvidencePhoto {
		return b.handleEvidencePhoto(ctx, c, cs)
	}

	if cs.State == StateAwaitingOptionalEvidence {
		return b.handleOptionalEvidencePhoto(ctx, c, cs)
	}

	if cs.State != StateAwaitingReceiptPhoto {
		return nil
	}

	// Retry path: expense exists but receipt was cleared for re-upload (handleCallbackRetryPhoto).
	// Create a new receipt and update the existing expense instead of creating a new one.
	if cs.PendingExpenseID != nil && cs.PendingReceiptID == nil {
		return b.handleRetryPhoto(ctx, c, cs)
	}

	// Idempotency guard: expense + receipt already created, waiting for OCR.
	if cs.PendingExpenseID != nil {
		return c.Send("Ya tenés un gasto pendiente de confirmación. Esperá el resultado del procesamiento.")
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
		return c.Send("No estás autenticado. Pedí al administrador un enlace de activación.")
	}

	// Route persistent keyboard button taps.
	switch c.Text() {
	case "📝 Registrar gasto":
		return b.handleGasto(c)
	case "📊 Ver mis gastos":
		return b.handleEstado(c)
	case "📎 Enviar soporte":
		return b.handleSoporte(c)
	}

	// FSM state routing.
	switch cs.State {
	case StateAwaitingReceiptPhoto:
		return b.handleManualAmount(ctx, c, cs)
	case StateAwaitingManualAmount:
		return b.handleManualAmount(ctx, c, cs)
	default:
		return c.Send("Elegí una opción del menú.", persistentMenuKeyboard())
	}
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

	// Transition to optional evidence state instead of resetting.
	cs.State = StateAwaitingOptionalEvidence
	b.states.set(telegramID, cs)
	_ = c.Respond()
	return c.Send("Gasto confirmado ✓\n\nSi querés adjuntar una foto adicional como evidencia, enviala ahora. O tocá el botón para terminar.", omitirDoneKeyboard())
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
	sb.WriteString("📋 Tus últimos gastos:\n\n")
	for i, exp := range expenses {
		date := exp.CreatedAt.Format("02/01/2006")
		amount := "—"
		if exp.Amount != nil {
			amount = "$" + exp.Amount.StringFixed(0)
		}
		statusLabel := statusEmoji(exp.Status)
		categoryName := ""
		if exp.CategoryName != "" {
			categoryName = exp.CategoryName
		} else {
			categoryName = string(exp.Status)
		}
		sb.WriteString(fmt.Sprintf("%d. %s — %s — %s — %s\n", i+1, categoryName, amount, statusLabel, date))
	}
	_ = c.Send(sb.String())
	return b.showMainMenu(ctx, c, cs)
}

// statusEmoji returns a display label (with emoji) for an expense status.
func statusEmoji(s expense.Status) string {
	switch s {
	case expense.StatusPending:
		return "⏳ Pendiente"
	case expense.StatusConfirmed:
		return "✅ Confirmado"
	case expense.StatusNeedsEvidence:
		return "📎 Necesita evidencia"
	case expense.StatusApproved:
		return "✓ Aprobado"
	case expense.StatusRejected:
		return "❌ Rechazado"
	default:
		return string(s)
	}
}

// handleOmitir handles /omitir — allows the driver to skip optional evidence submission.
// Only active in StateAwaitingOptionalEvidence; silently ignored in all other states.
func (b *Bot) handleOmitir(c tele.Context) error {
	telegramID := c.Sender().ID
	cs := b.states.get(telegramID)

	if cs.Claims == nil || cs.State != StateAwaitingOptionalEvidence {
		return nil
	}

	b.states.reset(telegramID)
	return c.Send("Gasto registrado ✓", persistentMenuKeyboard())
}

// handleOptionalEvidencePhoto processes a photo submitted as optional evidence after OCR confirmation.
// Attaches the receipt to the expense via AddAttachment (no status change) then stays in
// StateAwaitingOptionalEvidence, allowing the driver to add more photos.
// The driver uses /omitir to finish.
func (b *Bot) handleOptionalEvidencePhoto(ctx context.Context, c tele.Context, cs *ConversationState) error {
	photo := c.Message().Photo
	if photo == nil {
		return c.Send("No se pudo leer la foto. Intentá de nuevo o usá /omitir.")
	}

	photoBytes, fileID, err := b.downloadPhoto(ctx, c, photo)
	if err != nil {
		slog.Error("failed to download optional evidence photo", "err", err)
		return c.Send("No se pudo descargar la foto. Intentá de nuevo.")
	}

	storageKey := fmt.Sprintf("receipts/%s/optional-%s.jpg", cs.Claims.DriverID, uuid.New())
	storageURL, err := b.services.Storage.Upload(ctx, storageKey, photoBytes, "image/jpeg")
	if err != nil {
		slog.Error("failed to upload optional evidence", "err", err)
		return c.Send("No se pudo guardar la foto. Intentá de nuevo.")
	}

	r := &receipt.Receipt{
		DriverID:       cs.Claims.DriverID,
		TaxiID:         *cs.SelectedTaxiID,
		StorageURL:     storageURL,
		TelegramFileID: fileID,
		OCRStatus:      receipt.OCRStatusSkipped,
	}
	createdReceipt, err := b.services.Receipt.Create(ctx, r)
	if err != nil {
		slog.Error("failed to create optional evidence receipt", "err", err)
		return c.Send("Error al registrar el comprobante. Intentá de nuevo.")
	}

	// Also keep backward-compat by updating the primary receipt on the expense.
	if err := b.services.Expense.AttachOptionalEvidence(ctx, *cs.PendingExpenseID, cs.Claims.DriverID, createdReceipt.ID); err != nil {
		slog.Error("failed to attach optional evidence (primary)", "err", err)
		return c.Send("Error al adjuntar la evidencia. Intentá de nuevo.")
	}

	// Also record in the multi-attachment table (label left empty for optional evidence).
	if err := b.services.Expense.AddAttachment(ctx, *cs.PendingExpenseID, cs.Claims.DriverID, createdReceipt.ID, ""); err != nil {
		slog.Error("failed to add attachment record", "err", err)
		// Non-fatal: primary attachment succeeded, log and continue.
	}

	// Stay in StateAwaitingOptionalEvidence so the driver can add more photos.
	// State is already set; no change needed.
	return c.Send("Soporte adjuntado ✅. ¿Querés agregar otro soporte? Enviá otra foto o tocá el botón para terminar.", omitirDoneKeyboard())
}

// handleEvidencePhoto processes a photo submitted as evidence for a needs_evidence expense.
func (b *Bot) handleEvidencePhoto(ctx context.Context, c tele.Context, cs *ConversationState) error {
	telegramID := c.Sender().ID

	photo := c.Message().Photo
	if photo == nil {
		return c.Send("No se pudo leer la foto. Intentá de nuevo.")
	}

	photoBytes, fileID, err := b.downloadPhoto(ctx, c, photo)
	if err != nil {
		slog.Error("failed to download evidence photo from telegram", "err", err)
		return c.Send("No se pudo descargar la foto. Intentá de nuevo.")
	}

	// Upload to persistent storage FIRST (REQ-FRD-02).
	storageKey := fmt.Sprintf("receipts/%s/evidence-%s.jpg", cs.Claims.DriverID, uuid.New())
	storageURL, err := b.services.Storage.Upload(ctx, storageKey, photoBytes, "image/jpeg")
	if err != nil {
		slog.Error("failed to upload evidence to storage", "err", err)
		return c.Send("No se pudo guardar la foto. Intentá de nuevo.")
	}

	// Create receipt record with ocr_status=skipped (evidence doesn't need OCR).
	r := &receipt.Receipt{
		DriverID:       cs.Claims.DriverID,
		TaxiID:         *cs.SelectedTaxiID,
		StorageURL:     storageURL,
		TelegramFileID: fileID,
		OCRStatus:      receipt.OCRStatusSkipped,
	}
	createdReceipt, err := b.services.Receipt.Create(ctx, r)
	if err != nil {
		slog.Error("failed to create evidence receipt record", "err", err)
		return c.Send("Error al registrar el comprobante. Intentá de nuevo.")
	}

	// Submit evidence — transitions expense back to confirmed.
	if err := b.services.Expense.SubmitEvidence(ctx, *cs.PendingExpenseID, cs.Claims.DriverID, createdReceipt.ID); err != nil {
		slog.Error("failed to submit evidence", "err", err)
		return c.Send("Error al enviar la evidencia. Intentá de nuevo.")
	}

	// Also record in the multi-attachment table.
	if err := b.services.Expense.AddAttachment(ctx, *cs.PendingExpenseID, cs.Claims.DriverID, createdReceipt.ID, "Soporte requerido"); err != nil {
		slog.Error("failed to add attachment record for evidence photo", "err", err)
		// Non-fatal: primary submission succeeded.
	}

	// Transition to optional evidence state so the driver can add more photos.
	cs.State = StateAwaitingOptionalEvidence
	b.states.set(telegramID, cs)
	return c.Send("Evidencia enviada ✓ El administrador revisará tu gasto.\n\n¿Querés adjuntar otro soporte? Enviá otra foto o tocá el botón para terminar.", omitirDoneKeyboard())
}

// handleRetryPhoto handles a new photo upload when the driver chose to retry OCR.
// It creates a fresh receipt, links it to the existing expense, then waits for OCR.
func (b *Bot) handleRetryPhoto(ctx context.Context, c tele.Context, cs *ConversationState) error {
	telegramID := c.Sender().ID

	photo := c.Message().Photo
	if photo == nil {
		return c.Send("No se pudo leer la foto. Intentá de nuevo.")
	}

	photoBytes, fileID, err := b.downloadPhoto(ctx, c, photo)
	if err != nil {
		slog.Error("retry: failed to download photo", "err", err)
		return c.Send("No se pudo descargar la foto. Intentá de nuevo.")
	}

	storageKey := fmt.Sprintf("receipts/%s/%s.jpg", cs.Claims.DriverID, uuid.New())
	storageURL, err := b.services.Storage.Upload(ctx, storageKey, photoBytes, "image/jpeg")
	if err != nil {
		slog.Error("retry: failed to upload photo", "err", err)
		return c.Send("No se pudo guardar la foto. Intentá de nuevo.")
	}

	r := &receipt.Receipt{
		DriverID:       cs.Claims.DriverID,
		TaxiID:         *cs.SelectedTaxiID,
		StorageURL:     storageURL,
		TelegramFileID: fileID,
		OCRStatus:      receipt.OCRStatusPending,
	}
	createdReceipt, err := b.services.Receipt.Create(ctx, r)
	if err != nil {
		slog.Error("retry: failed to create receipt", "err", err)
		return c.Send("Error al registrar el recibo. Intentá de nuevo.")
	}

	// Link the new receipt to the existing expense.
	if err := b.services.Expense.AttachOptionalEvidence(ctx, *cs.PendingExpenseID, cs.Claims.DriverID, createdReceipt.ID); err != nil {
		slog.Error("retry: failed to update receipt on expense", "err", err)
		return c.Send("Error al actualizar el gasto. Intentá de nuevo.")
	}

	receiptID := createdReceipt.ID
	cs.PendingReceiptID = &receiptID
	cs.State = StateAwaitingOCRConfirmation
	b.states.set(telegramID, cs)

	return c.Send("Foto recibida ✓ Procesando factura de nuevo...")
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
func (b *Bot) getExpenseCategories(ctx context.Context, ownerID uuid.UUID) ([]expenseCategory, error) {
	lister, ok := b.services.Expense.(categoryLister)
	if !ok {
		return nil, fmt.Errorf("expense service does not implement ListCategories")
	}
	cats, err := lister.ListCategories(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	result := make([]expenseCategory, 0, len(cats))
	for _, c := range cats {
		result = append(result, expenseCategory{ID: c.ID, Name: c.Name})
	}
	return result, nil
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
	ListCategories(ctx context.Context, ownerID uuid.UUID) ([]*expense.ExpenseCategory, error)
}
