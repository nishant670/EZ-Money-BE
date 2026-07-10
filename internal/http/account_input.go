package http

import (
	"strings"

	"finance-parser-go/internal/models"
)

var validAccountTypes = map[string]bool{
	"cash": true, "credit": true, "debit": true, "wallet": true,
	"upi": true, "bank": true, "other": true,
}

type accountInput struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Provider    string  `json:"provider"`
	Identifier  string  `json:"identifier"`
	CreditLimit float64 `json:"credit_limit"`
	DueDay      int     `json:"due_day"`
	FeeMonth    string  `json:"fee_month"`
	Balance     float64 `json:"balance"`
	IsDefault   bool    `json:"is_default"`
}

func (input accountInput) validate() map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = "is required"
	}
	if !validAccountTypes[strings.ToLower(strings.TrimSpace(input.Type))] {
		fields["type"] = "is invalid"
	}
	if input.CreditLimit < 0 {
		fields["credit_limit"] = "must not be negative"
	}
	if input.DueDay < 0 || input.DueDay > 31 {
		fields["due_day"] = "must be between 1 and 31"
	}
	return fields
}

func (input accountInput) apply(account *models.Account) {
	account.Type = strings.ToLower(strings.TrimSpace(input.Type))
	account.Name = strings.TrimSpace(input.Name)
	account.Color = strings.TrimSpace(input.Color)
	account.Provider = strings.TrimSpace(input.Provider)
	account.Identifier = strings.TrimSpace(input.Identifier)
	account.CreditLimit = input.CreditLimit
	account.DueDay = input.DueDay
	account.FeeMonth = strings.TrimSpace(input.FeeMonth)
	account.Balance = input.Balance
	account.IsDefault = input.IsDefault
}
