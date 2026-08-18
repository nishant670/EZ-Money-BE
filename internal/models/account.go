package models

import (
	"time"
)

type Account struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserID      uint   `json:"user_id"`
	Type        string `json:"type"` // cash, upi, bank, credit_card, debit_card, wallet, other
	Name        string `json:"name"`
	Color       string `json:"color"`
	Provider    string `json:"provider"`   // bank name or issuer
	Identifier  string `json:"identifier"` // last 4 digits or upi id
	CreditLimit Money  `gorm:"type:numeric(19,2);not null;default:0" json:"credit_limit"`
	DueDay      int    `json:"due_day"`
	// StatementDay is the day of month a card bills on. Zero means the user
	// has not told us; the first statement they enter infers it.
	StatementDay int `gorm:"not null;default:0" json:"statement_day"`
	// ReminderDaysBefore is the lead time on the "bill due soon" reminder.
	ReminderDaysBefore int `gorm:"not null;default:3" json:"reminder_days_before"`
	// AutopayEnabled only changes how the due-date reminder is worded. A
	// payment is never recorded without the user confirming it.
	AutopayEnabled bool      `gorm:"not null;default:false" json:"autopay_enabled"`
	FeeMonth       string    `json:"fee_month"`
	Balance        Money     `gorm:"type:numeric(19,2);not null;default:0" json:"balance"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
