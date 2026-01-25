package models

import (
	"time"
)

type Account struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `json:"user_id"`
	Type        string    `json:"type"` // credit, debit, wallet, upi, bank, other
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Provider    string    `json:"provider"`   // bank name or issuer
	Identifier  string    `json:"identifier"` // last 4 digits or upi id
	CreditLimit float64   `json:"credit_limit"`
	DueDay      int       `json:"due_day"`
	FeeMonth    string    `json:"fee_month"`
	Balance     float64   `json:"balance"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
