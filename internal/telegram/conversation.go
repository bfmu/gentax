// Package telegram implements the Telegram bot, update dispatcher, and expense flow FSM.
package telegram

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// State represents the current step of the multi-step expense registration flow for a user.
type State int

const (
	// StateIdle — no active flow; the user is not in any conversation.
	StateIdle State = iota
	// StateAwaitingTaxiSelection — the bot has presented taxi options and is waiting for a choice.
	StateAwaitingTaxiSelection
	// StateAwaitingCategorySelection — taxi selected; waiting for expense category.
	StateAwaitingCategorySelection
	// StateAwaitingReceiptPhoto — category selected; waiting for a receipt photo or manual amount.
	StateAwaitingReceiptPhoto
	// StateAwaitingManualAmount — OCR failed or driver chose manual entry; waiting for amount text.
	StateAwaitingManualAmount
	// StateAwaitingOCRConfirmation — OCR completed; waiting for driver to confirm or edit.
	StateAwaitingOCRConfirmation
	// StateAwaitingEvidencePhoto — owner requested evidence; waiting for driver to send a new receipt photo.
	StateAwaitingEvidencePhoto
	// StateAwaitingOptionalEvidence — expense confirmed; driver may attach optional supporting evidence.
	StateAwaitingOptionalEvidence
)

// ConversationState holds per-user FSM data for an active expense flow.
type ConversationState struct {
	State              State
	Claims             *botClaims // JWT claims resolved at /start; nil if unauthenticated
	SelectedTaxiID     *uuid.UUID
	SelectedCategoryID *uuid.UUID
	PendingReceiptID   *uuid.UUID
	PendingExpenseID   *uuid.UUID
	UpdatedAt          time.Time
}

// botClaims holds the minimal JWT payload needed by the bot FSM.
type botClaims struct {
	DriverID uuid.UUID
	OwnerID  uuid.UUID
	UserID   uuid.UUID
	Token    string // signed JWT stored for downstream use
}

// conversationStore is a thread-safe in-memory store for per-user FSM state.
type conversationStore struct {
	mu sync.Map // map[int64]*ConversationState
}

// get returns the ConversationState for telegramID, or a fresh idle state if none exists.
func (s *conversationStore) get(telegramID int64) *ConversationState {
	v, ok := s.mu.Load(telegramID)
	if !ok {
		return &ConversationState{State: StateIdle}
	}
	return v.(*ConversationState)
}

// set stores the ConversationState for telegramID.
func (s *conversationStore) set(telegramID int64, cs *ConversationState) {
	cs.UpdatedAt = time.Now()
	s.mu.Store(telegramID, cs)
}

// reset returns the user to StateIdle while preserving their JWT claims.
// This keeps the driver authenticated across multiple expense registrations.
func (s *conversationStore) reset(telegramID int64) {
	v, ok := s.mu.Load(telegramID)
	if !ok {
		return
	}
	cs := v.(*ConversationState)
	s.mu.Store(telegramID, &ConversationState{
		State:     StateIdle,
		Claims:    cs.Claims, // preserve JWT — driver stays logged in
		UpdatedAt: time.Now(),
	})
}
