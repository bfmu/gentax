package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
)

// DirectNotifier sends Telegram messages directly via the Bot API HTTP endpoint.
// Use in processes that do not run the full telebot.v3 bot (e.g., the API process).
type DirectNotifier struct {
	botToken   string
	expenseSvc expense.Service
	driverRepo driver.Repository
}

// NewDirectNotifier creates a DirectNotifier.
func NewDirectNotifier(botToken string, expenseSvc expense.Service, driverRepo driver.Repository) *DirectNotifier {
	return &DirectNotifier{
		botToken:   botToken,
		expenseSvc: expenseSvc,
		driverRepo: driverRepo,
	}
}

// NotifyEvidenceRequest sends a Telegram message to the expense's driver requesting more evidence.
func (n *DirectNotifier) NotifyEvidenceRequest(ctx context.Context, expenseID uuid.UUID, message string) error {
	text := "El administrador solicita más evidencia para tu gasto."
	if strings.TrimSpace(message) != "" {
		text += "\n\n" + message
	}
	text += "\n\nUsá /soporte para enviar la foto del comprobante."
	return n.notifyDriver(ctx, expenseID, text)
}

// NotifyRejection sends a Telegram message to the expense's driver informing them of rejection.
func (n *DirectNotifier) NotifyRejection(ctx context.Context, expenseID uuid.UUID, reason string) error {
	msg := "Tu gasto fue rechazado."
	if strings.TrimSpace(reason) != "" {
		msg = fmt.Sprintf("Tu gasto fue rechazado. Motivo: %s", reason)
	}
	return n.notifyDriver(ctx, expenseID, msg)
}

// notifyDriver looks up the driver's Telegram ID and sends them a message.
// Best-effort: returns nil if the driver has no Telegram account linked.
func (n *DirectNotifier) notifyDriver(ctx context.Context, expenseID uuid.UUID, text string) error {
	exp, err := n.expenseSvc.GetByID(ctx, expenseID, uuid.Nil)
	if err != nil {
		return nil // best-effort: expense not found, skip
	}
	tid, err := n.driverRepo.GetDriverTelegramID(ctx, exp.DriverID)
	if err != nil || tid == nil {
		return nil // driver not linked to Telegram, skip
	}
	return n.sendMessage(*tid, text)
}

type sendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func (n *DirectNotifier) sendMessage(chatID int64, text string) error {
	payload, _ := json.Marshal(sendMessageRequest{ChatID: chatID, Text: text})
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload)) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
