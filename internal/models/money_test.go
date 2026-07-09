package models

import (
	"encoding/json"
	"testing"
)

func TestMoneyRejectsExcessPrecision(t *testing.T) {
	var amount Money
	if err := json.Unmarshal([]byte(`12.345`), &amount); err == nil {
		t.Fatal("expected amount with more than two decimal places to fail")
	}
}

func TestMoneyJSONRoundTrip(t *testing.T) {
	var amount Money
	if err := json.Unmarshal([]byte(`"1234.50"`), &amount); err != nil {
		t.Fatal(err)
	}
	if amount.String() != "1234.50" {
		t.Fatalf("got %s", amount.String())
	}
	encoded, err := json.Marshal(amount)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "1234.50" {
		t.Fatalf("got %s", encoded)
	}
}
