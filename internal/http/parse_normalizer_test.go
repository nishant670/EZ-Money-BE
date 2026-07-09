package http

import (
	"reflect"
	"testing"
)

func TestNormalizeParsedDraftDoesNotGuessMissingFields(t *testing.T) {
	entry := map[string]any{
		"type":     "expense",
		"title":    "Lunch",
		"amount":   float64(250),
		"mode":     "UPI",
		"category": nil,
		"date":     "not-a-date",
	}

	normalizeParsedDraft(entry, "paid 250 for lunch")

	if entry["date"] != nil || entry["category"] != nil {
		t.Fatalf("normalizer guessed missing values: %#v", entry)
	}
	wantMissing := []string{"category", "date"}
	if !reflect.DeepEqual(entry["missing_fields"], wantMissing) {
		t.Fatalf("missing_fields = %#v, want %#v", entry["missing_fields"], wantMissing)
	}
	if entry["source_text"] != "paid 250 for lunch" {
		t.Fatal("source_text must be set from the original transcript")
	}
}

func TestNormalizeParsedDraftPreservesValidValues(t *testing.T) {
	entry := map[string]any{
		"title": "Metro", "amount": float64(45), "type": "expense",
		"mode": "UPI", "category": "Travel", "date": "2026-07-09",
	}
	normalizeParsedDraft(entry, "metro 45 via upi today")

	if missing := entry["missing_fields"].([]string); len(missing) != 0 {
		t.Fatalf("unexpected missing fields: %v", missing)
	}
	if entry["currency"] != "INR" || entry["stage"] != "draft" {
		t.Fatalf("normalization defaults missing: %#v", entry)
	}
}

func TestNormalizeIndianPaymentFixtures(t *testing.T) {
	fixtures := []struct {
		name  string
		entry map[string]any
	}{
		{"upi", map[string]any{"title": "Chai", "amount": float64(80), "type": "expense", "mode": "UPI", "category": "Food", "date": "2026-07-09"}},
		{"rupay card", map[string]any{"title": "Fuel", "amount": float64(2500), "type": "expense", "mode": "Credit Card", "card_network": "Rupay", "category": "Travel", "date": "2026-07-08"}},
		{"cash", map[string]any{"title": "Auto", "amount": float64(120), "type": "expense", "mode": "Cash", "category": "Travel", "date": "2026-07-09"}},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			normalizeParsedDraft(fixture.entry, fixture.name)
			if missing := fixture.entry["missing_fields"].([]string); len(missing) != 0 {
				t.Fatalf("unexpected missing fields: %v", missing)
			}
		})
	}
}
