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
	tests := []struct {
		name  string
		input entryInput
		valid bool
	}{
		{"valid with account", entryInput{Amount: testMoney("250"), Type: "expense", Date: "2026-07-09", AccountID: testAccountID(4)}, true},
		{"valid without account", entryInput{Amount: testMoney("250"), Type: "expense", Date: "2026-07-09"}, true},
		{"invalid amount", entryInput{Amount: 0, Type: "expense", Date: "2026-07-09", AccountID: testAccountID(4)}, false},
		{"invalid type", entryInput{Amount: testMoney("250"), Type: "transfer", Date: "2026-07-09", AccountID: testAccountID(4)}, false},
		{"invalid date", entryInput{Amount: testMoney("250"), Type: "expense", Date: "09-07-2026", AccountID: testAccountID(4)}, false},
		{"invalid currency", entryInput{Amount: testMoney("250"), Type: "expense", Currency: "USD", Date: "2026-07-09", AccountID: testAccountID(4)}, false},
		{"invalid source", entryInput{Amount: testMoney("250"), Type: "expense", Source: "import", Date: "2026-07-09", AccountID: testAccountID(4)}, false},
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
	entry := (entryInput{Amount: testMoney("250"), Type: "Expense", Date: "2026-07-09", AccountID: testAccountID(7)}).toModel(3)
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
