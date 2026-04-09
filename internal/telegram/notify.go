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
	sb.WriteString("Factura procesada:\n")
	if result.Vendor != nil {
		sb.WriteString(fmt.Sprintf("Proveedor: %s\n", *result.Vendor))
	}
	if result.Total != nil {
		sb.WriteString(fmt.Sprintf("Total: $%s COP\n", *result.Total))
	}
	if result.Date != nil {
		sb.WriteString(fmt.Sprintf("Fecha: %s\n", *result.Date))
	}
	if result.NIT != nil {
		sb.WriteString(fmt.Sprintf("NIT: %s\n", *result.NIT))
	}
	if result.CUFE != nil {
		sb.WriteString(fmt.Sprintf("CUFE: %s\n", *result.CUFE))
	}

	// Build inline keyboard for confirm / edit.
	confirmBtn := tele.Btn{Unique: callbackConfirmOCR, Text: "Confirmar ✓"}
	editBtn := tele.Btn{Unique: callbackEditAmount, Text: "Corregir monto ✏️"}
	kb := &tele.ReplyMarkup{}
	kb.Inline(tele.Row{confirmBtn, editBtn})

	return b.send(&telegramUser{id: telegramID}, sb.String(), kb)
}
