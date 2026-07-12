package http

import (
	"testing"

	"finance-parser-go/internal/models"
)

func TestAccountInputValidation(t *testing.T) {
	tests := []struct {
		name  string
		input accountInput
		valid bool
	}{
		{"valid", accountInput{Name: "Cash", Type: "cash"}, true},
		{"missing name", accountInput{Type: "cash"}, false},
		{"invalid type", accountInput{Name: "Broker", Type: "investment"}, false},
		{"canonical credit card", accountInput{Name: "Card", Type: "credit_card"}, true},
		{"legacy credit alias", accountInput{Name: "Card", Type: "credit"}, true},
		{"legacy wallets alias", accountInput{Name: "Wallet", Type: "wallets"}, true},
		{"invalid due day", accountInput{Name: "Card", Type: "credit_card", DueDay: 32}, false},
		{"negative limit", accountInput{Name: "Card", Type: "credit_card", CreditLimit: -1}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := len(test.input.validate()) == 0; got != test.valid {
				t.Fatalf("valid = %v, fields = %v", got, test.input.validate())
			}
		})
	}
}

func TestAccountInputDoesNotOverwriteOwnership(t *testing.T) {
	account := models.Account{ID: 7, UserID: 9}
	(accountInput{Name: " Daily Cash ", Type: "CASH"}).apply(&account)
	if account.ID != 7 || account.UserID != 9 {
		t.Fatalf("ownership changed: %#v", account)
	}
	if account.Name != "Daily Cash" || account.Type != "cash" {
		t.Fatalf("input was not normalized: %#v", account)
	}
}

func TestAccountInputCanonicalizesLegacyTypeAliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"credit", "credit_card"},
		{" CREDIT_CARD ", "credit_card"},
		{"debit", "debit_card"},
		{"wallets", "wallet"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			account := models.Account{}
			(accountInput{Name: "Alias", Type: test.input}).apply(&account)
			if account.Type != test.want {
				t.Fatalf("Type = %q, want %q", account.Type, test.want)
			}
		})
	}
}
