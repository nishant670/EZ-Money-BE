package http

import (
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

func TestEntryInputValidation(t *testing.T) {
	tests := []struct {
		name  string
		input entryInput
		valid bool
	}{
		{"valid", entryInput{Amount: testMoney("250"), Type: "expense", Date: "2026-07-09", AccountID: 4}, true},
		{"missing account", entryInput{Amount: testMoney("250"), Type: "expense", Date: "2026-07-09"}, false},
		{"invalid amount", entryInput{Amount: 0, Type: "expense", Date: "2026-07-09", AccountID: 4}, false},
		{"invalid type", entryInput{Amount: testMoney("250"), Type: "transfer", Date: "2026-07-09", AccountID: 4}, false},
		{"invalid date", entryInput{Amount: testMoney("250"), Type: "expense", Date: "09-07-2026", AccountID: 4}, false},
		{"invalid currency", entryInput{Amount: testMoney("250"), Type: "expense", Currency: "USD", Date: "2026-07-09", AccountID: 4}, false},
		{"invalid source", entryInput{Amount: testMoney("250"), Type: "expense", Source: "import", Date: "2026-07-09", AccountID: 4}, false},
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
	entry := (entryInput{Amount: testMoney("250"), Type: "Expense", Date: "2026-07-09", AccountID: 7}).toModel(3)
	if entry.Currency != "INR" {
		t.Fatalf("expected INR default, got %q", entry.Currency)
	}
	if entry.AccountID != 7 {
		t.Fatalf("expected account 7, got %v", entry.AccountID)
	}
	if entry.UserID != 3 {
		t.Fatalf("expected user 3, got %d", entry.UserID)
	}
}
