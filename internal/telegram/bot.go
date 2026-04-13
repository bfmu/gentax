// Package telegram implements the Telegram bot, update dispatcher, and expense flow FSM.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/google/uuid"
	tele "gopkg.in/telebot.v3"
)

const jwtTTL = 30 * 24 * time.Hour // 30 days — drivers stay logged in across bot restarts

// Sender is an abstraction over *tele.Bot.Send that allows injection of a mock in tests.
type Sender interface {
	Send(to tele.Recipient, msg interface{}, opts ...interface{}) (*tele.Message, error)
}

// DriverRepo is the subset of driver.Repository the bot needs directly.
// It exposes operations not available on driver.Service (e.g. GetByTelegramID,
// GetActiveAssignment) without requiring the full repository to be injected.
type DriverRepo interface {
	GetByTelegramID(ctx context.Context, telegramID int64) (*driver.Driver, error)
	GetActiveAssignment(ctx context.Context, driverID uuid.UUID) (*driver.Assignment, error)
	GetDriverTelegramID(ctx context.Context, driverID uuid.UUID) (*int64, error)
}

// Services bundles all domain dependencies needed by the bot.
type Services struct {
	Auth       auth.TokenIssuer
	Driver     driver.Service
	DriverRepo DriverRepo // repository-level operations not on Service interface
	Expense    expense.Service
	Receipt    receipt.Repository
	Storage    receipt.StorageClient
}

// Bot is the Telegram bot implementation.
type Bot struct {
	bot      *tele.Bot // nil in unit tests
	sender   Sender
	services Services
	states   conversationStore
}

// NewBot creates a new Bot wired to the Telegram API.
// Call Start() to begin long-polling.
func NewBot(token string, svc Services) (*Bot, error) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create telebot: %w", err)
	}
	bot := &Bot{bot: b, sender: b, services: svc}
	bot.registerHandlers(b)
	return bot, nil
}

// newBotWithSender creates a Bot with a custom Sender — used in unit tests.
func newBotWithSender(sender Sender, svc Services) *Bot {
	b := &Bot{sender: sender, services: svc}
	return b
}

// registerHandlers wires command/callback handlers to the telebot instance.
func (b *Bot) registerHandlers(bot *tele.Bot) {
	bot.Handle("/start", b.handleStart)
	bot.Handle("/gasto", b.handleGasto)
	bot.Handle("/estado", b.handleEstado)
	bot.Handle("/soporte", b.handleSoporte)
	bot.Handle("/omitir", b.handleOmitir)

	// Callback handlers for inline keyboards
	bot.Handle(&tele.Btn{Unique: callbackSelectTaxi}, b.handleTaxiSelection)
	bot.Handle(&tele.Btn{Unique: callbackSelectCategory}, b.handleCategorySelection)
	bot.Handle(&tele.Btn{Unique: callbackConfirmOCR}, b.handleConfirmOCR)
	bot.Handle(&tele.Btn{Unique: callbackEditAmount}, b.handleEditAmount)
	bot.Handle(&tele.Btn{Unique: callbackNewExpense}, b.handleCallbackNewExpense)
	bot.Handle(&tele.Btn{Unique: callbackViewStatus}, b.handleCallbackViewStatus)
	bot.Handle(&tele.Btn{Unique: callbackSendEvidence}, b.handleCallbackSendEvidence)
	bot.Handle(&tele.Btn{Unique: callbackOmitir}, b.handleCallbackOmitir)
	bot.Handle(&tele.Btn{Unique: callbackCancelEvidence}, b.handleCallbackCancelEvidence)

	// Photo and text messages are handled by the FSM router
	bot.Handle(tele.OnPhoto, b.handlePhoto)
	bot.Handle(tele.OnText, b.handleText)
}

// Start begins the bot's long-polling loop (blocking).
func (b *Bot) Start() {
	slog.Info("telegram bot starting")
	b.bot.Start()
}

// Stop gracefully shuts down the bot.
func (b *Bot) Stop() {
	if b.bot != nil {
		b.bot.Stop()
	}
}

// send is a helper that calls b.sender.Send for easy mocking.
func (b *Bot) send(to tele.Recipient, msg interface{}, opts ...interface{}) error {
	_, err := b.sender.Send(to, msg, opts...)
	return err
}
