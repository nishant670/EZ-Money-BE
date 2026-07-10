package http

import (
	"encoding/json"
	"testing"

	"finance-parser-go/internal/models"
)

func testMoney(value string) models.Money {
	amount, err := models.ParseMoney(value)
	if err != nil {
		panic(err)
	}
	return amount
}

func testAccountID(value uint) *uint {
	return &value
}

func validEntryInput() entryInput {
	return entryInput{
		Title: "Lunch", Amount: testMoney("250"), Type: "expense",
		Mode: "UPI", Category: "Food", Date: "2026-07-09",
	}
}

func TestUpdateEntryInputDistinguishesMissingAndNullAccount(t *testing.T) {
	var missing updateEntryInput
	if err := json.Unmarshal([]byte(`{}`), &missing); err != nil {
		t.Fatal(err)
	}
	if missing.AccountID.Set {
		t.Fatal("expected omitted account_id to remain unset")
	}

	var cleared updateEntryInput
	if err := json.Unmarshal([]byte(`{"account_id":null}`), &cleared); err != nil {
		t.Fatal(err)
	}
	if !cleared.AccountID.Set || cleared.AccountID.Value != nil {
		t.Fatal("expected null account_id to request clearing the account")
	}
}

func TestEntryInputValidation(t *testing.T) {
	withAccount := validEntryInput()
	withAccount.AccountID = testAccountID(4)
	withoutAccount := validEntryInput()
	invalidAmount := validEntryInput()
	invalidAmount.Amount = 0
	invalidType := validEntryInput()
	invalidType.Type = "transfer"
	invalidDate := validEntryInput()
	invalidDate.Date = "09-07-2026"
	invalidCurrency := validEntryInput()
	invalidCurrency.Currency = "USD"
	invalidSource := validEntryInput()
	invalidSource.Source = "import"
	invalidMode := validEntryInput()
	invalidMode.Mode = "Cheque"
	missingCategory := validEntryInput()
	missingCategory.Category = ""
	tests := []struct {
		name  string
		input entryInput
		valid bool
	}{
		{"valid with account", withAccount, true},
		{"valid without account", withoutAccount, true},
		{"invalid amount", invalidAmount, false},
		{"invalid type", invalidType, false},
		{"invalid date", invalidDate, false},
		{"invalid currency", invalidCurrency, false},
		{"invalid source", invalidSource, false},
		{"invalid mode", invalidMode, false},
		{"missing category", missingCategory, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fields := test.input.validate()
			if test.valid && len(fields) != 0 {
				t.Fatalf("expected valid input, got %v", fields)
			}
			if !test.valid && len(fields) == 0 {
				t.Fatal("expected validation errors")
			}
		})
	}
}

func TestEntryInputDefaultsCurrencyAndLinksAccount(t *testing.T) {
	input := validEntryInput()
	input.Type = "Expense"
	input.AccountID = testAccountID(7)
	entry := input.toModel(3)
	if entry.Currency != "INR" {
		t.Fatalf("expected INR default, got %q", entry.Currency)
	}
	if entry.AccountID == nil || *entry.AccountID != 7 {
		t.Fatalf("expected account 7, got %v", entry.AccountID)
	}
	if entry.UserID != 3 {
		t.Fatalf("expected user 3, got %d", entry.UserID)
	}
}
