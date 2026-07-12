package models

import (
	"time"
)

type Account struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `json:"user_id"`
	Type        string    `json:"type"` // cash, upi, bank, credit_card, debit_card, wallet, other
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Provider    string    `json:"provider"`   // bank name or issuer
	Identifier  string    `json:"identifier"` // last 4 digits or upi id
	CreditLimit Money     `gorm:"type:numeric(19,2);not null;default:0" json:"credit_limit"`
	DueDay      int       `json:"due_day"`
	FeeMonth    string    `json:"fee_month"`
	Balance     Money     `gorm:"type:numeric(19,2);not null;default:0" json:"balance"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
