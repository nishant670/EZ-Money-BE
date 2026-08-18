package http

import (
	"strings"

	"finance-parser-go/internal/models"
)

var canonicalAccountTypes = map[string]string{
	"cash":        "cash",
	"upi":         "upi",
	"bank":        "bank",
	"credit_card": "credit_card",
	"credit":      "credit_card",
	"debit_card":  "debit_card",
	"debit":       "debit_card",
	"wallet":      "wallet",
	"wallets":     "wallet",
	"other":       "other",
}

type accountInput struct {
	Type        string       `json:"type"`
	Name        string       `json:"name"`
	Color       string       `json:"color"`
	Provider    string       `json:"provider"`
	Identifier  string       `json:"identifier"`
	CreditLimit models.Money `json:"credit_limit"`
	DueDay      int          `json:"due_day"`
	FeeMonth    string       `json:"fee_month"`
	Balance     models.Money `json:"balance"`
	IsDefault   bool         `json:"is_default"`

	// Card statement settings are pointers so that omitting them means "leave
	// as is" rather than "reset to zero". The app sends the whole account back
	// on every edit, and StatementDay in particular is inferred once from the
	// user's first bill — a rename must not silently wipe it and take the
	// card's billing cycle with it.
	StatementDay       *int  `json:"statement_day"`
	ReminderDaysBefore *int  `json:"reminder_days_before"`
	AutopayEnabled     *bool `json:"autopay_enabled"`
}

func (input accountInput) validate() map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = "is required"
	}
	if normalizeAccountType(input.Type) == "" {
		fields["type"] = "is invalid"
	}
	if input.CreditLimit < 0 {
		fields["credit_limit"] = "must not be negative"
	}
	if input.DueDay < 0 || input.DueDay > 31 {
		fields["due_day"] = "must be between 1 and 31"
	}
	if input.StatementDay != nil && (*input.StatementDay < 0 || *input.StatementDay > 31) {
		fields["statement_day"] = "must be between 1 and 31"
	}
	if input.ReminderDaysBefore != nil && (*input.ReminderDaysBefore < 0 || *input.ReminderDaysBefore > 30) {
		fields["reminder_days_before"] = "must be between 0 and 30"
	}
	return fields
}

func (input accountInput) apply(account *models.Account) {
	account.Type = normalizeAccountType(input.Type)
	account.Name = strings.TrimSpace(input.Name)
	account.Color = strings.TrimSpace(input.Color)
	account.Provider = strings.TrimSpace(input.Provider)
	account.Identifier = strings.TrimSpace(input.Identifier)
	account.CreditLimit = input.CreditLimit
	account.DueDay = input.DueDay
	account.FeeMonth = strings.TrimSpace(input.FeeMonth)
	account.Balance = input.Balance
	account.IsDefault = input.IsDefault

	if input.StatementDay != nil {
		account.StatementDay = *input.StatementDay
	}
	if input.ReminderDaysBefore != nil {
		account.ReminderDaysBefore = *input.ReminderDaysBefore
	}
	if input.AutopayEnabled != nil {
		account.AutopayEnabled = *input.AutopayEnabled
	}
}

func normalizeAccountType(accountType string) string {
	return canonicalAccountTypes[strings.ToLower(strings.TrimSpace(accountType))]
}
