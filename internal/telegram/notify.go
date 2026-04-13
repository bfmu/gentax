package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/google/uuid"
	tele "gopkg.in/telebot.v3"
)


// telegramUser is a lightweight tele.Recipient backed by a Telegram user ID.
type telegramUser struct {
	id int64
}

func (u *telegramUser) Recipient() string {
	return fmt.Sprintf("%d", u.id)
}

// NotifyRejection sends a rejection notification to a driver.
// REQ-APR-03: notification MUST NOT roll back rejection if Telegram send fails.
func (b *Bot) NotifyRejection(ctx context.Context, telegramID int64, expenseID uuid.UUID, reason string) error {
	_ = ctx
	msg := fmt.Sprintf("Tu gasto fue rechazado.")
	if strings.TrimSpace(reason) != "" {
		msg = fmt.Sprintf("Tu gasto fue rechazado. Motivo: %s", reason)
	}
	return b.send(&telegramUser{id: telegramID}, msg)
}

// NotifyEvidenceRequest looks up the expense's driver and sends a Telegram notification
// informing them that additional evidence is required.
// Best-effort: errors are logged but never propagate to the caller.
func (b *Bot) NotifyEvidenceRequest(ctx context.Context, expenseID uuid.UUID, message string) error {
	exp, err := b.services.Expense.GetByID(ctx, expenseID, uuid.Nil)
	if err != nil {
		return nil // best-effort: driver path lookup failed, skip
	}
	tid, err := b.services.DriverRepo.GetDriverTelegramID(ctx, exp.DriverID)
	if err != nil || tid == nil {
		return nil // driver has no telegram linked, skip
	}
	text := "El administrador solicita más evidencia para tu gasto."
	if strings.TrimSpace(message) != "" {
		text += "\n\n" + message
	}
	text += "\n\nUsá /soporte para enviar la foto del comprobante."
	chat := &tele.Chat{ID: *tid}
	_, err = b.sender.Send(chat, text)
	return err
}

// NotifyOCRResult sends an OCR processing result to the driver.
// REQ-OCR-04: on success show fields + confirm/edit buttons; on failure prompt manual entry.
func (b *Bot) NotifyOCRResult(ctx context.Context, telegramID int64, receiptID uuid.UUID, result *receipt.OCRResult) error {
	_ = ctx
	cs := b.states.get(telegramID)

	if result == nil {
		// OCR failed — prompt manual entry.
		if cs.Claims != nil && cs.PendingExpenseID != nil {
			cs.State = StateAwaitingManualAmount
			b.states.set(telegramID, cs)
		}
		return b.send(&telegramUser{id: telegramID},
			"No pudimos leer la factura. Ingresá el monto manualmente.")
	}

	// Build summary of extracted fields.
	var sb strings.Builder
	if result.Total != nil {
		sb.WriteString(fmt.Sprintf("Factura procesada ✓\nMonto: $%s COP\n", *result.Total))
	} else {
		sb.WriteString("Factura procesada, pero no se pudo leer el monto.\n")
	}
	if result.Date != nil {
		sb.WriteString(fmt.Sprintf("Fecha: %s\n", *result.Date))
	}

	// Build inline keyboard.
	confirmBtn := tele.Btn{Unique: callbackConfirmOCR, Text: "Confirmar ✓"}
	editBtn := tele.Btn{Unique: callbackEditAmount, Text: "Ingresar monto ✏️"}
	retryBtn := tele.Btn{Unique: callbackRetryPhoto, Text: "🔄 Reenviar foto"}
	kb := &tele.ReplyMarkup{}
	if result.Total != nil {
		// OCR found amount: confirm or correct.
		kb.Inline(tele.Row{confirmBtn, editBtn})
	} else {
		// No amount found: retry photo or enter manually.
		kb.Inline(tele.Row{retryBtn}, tele.Row{editBtn})
	}

	return b.send(&telegramUser{id: telegramID}, sb.String(), kb)
}
