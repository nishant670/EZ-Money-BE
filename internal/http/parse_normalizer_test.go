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

func TestNormalizeParsedDraftDefaultsMissingPaymentMode(t *testing.T) {
	entry := map[string]any{
		"title": "Uber", "amount": float64(500), "type": "expense",
		"category": "Travel", "date": "2026-07-17",
		"missing_fields":     []any{"mode"},
		"needs_confirmation": map[string]any{"mode": true},
	}

	normalizeParsedDraft(entry, "Today I paid 500 rupees for the Uber cab for Ria")

	if entry["mode"] != defaultParseMode {
		t.Fatalf("mode = %#v, want %s", entry["mode"], defaultParseMode)
	}
	if missing := entry["missing_fields"].([]string); len(missing) != 0 {
		t.Fatalf("mode should not be a missing field: %v", missing)
	}
	if _, ok := entry["needs_confirmation"].(map[string]any)["mode"]; ok {
		t.Fatalf("mode should not need confirmation: %#v", entry["needs_confirmation"])
	}
}

func TestNormalizeParsedDraftDefaultsInvalidPaymentMode(t *testing.T) {
	entry := map[string]any{
		"title": "Uber", "amount": float64(500), "type": "expense",
		"mode": "unknown", "category": "Travel", "date": "2026-07-17",
	}

	normalizeParsedDraft(entry, "Today I paid 500 rupees for the Uber cab")

	if entry["mode"] != defaultParseMode {
		t.Fatalf("mode = %#v, want %s", entry["mode"], defaultParseMode)
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

func TestNormalizeSubscriptionCandidateLabelsDraft(t *testing.T) {
	entry := map[string]any{
		"title":    "Sub",
		"amount":   float64(1500),
		"type":     "expense",
		"mode":     "Cash",
		"category": "Bills",
		"date":     "2026-07-17",
		"subscription_candidate": map[string]any{
			"name":             nil,
			"amount":           float64(1500),
			"billing_interval": nil,
			"next_due_date":    nil,
		},
	}

	normalizeParsedDraft(entry, "paid 1500 rupees for my subscription by cash")

	if entry["recurring_candidate"] != true || entry["tag"] != "Subscription" {
		t.Fatalf("subscription label was not normalized: %#v", entry)
	}
	candidate := entry["subscription_candidate"].(map[string]any)
	missing := candidate["missing_fields"].([]any)
	if !reflect.DeepEqual(missing, []any{"name", "billing_interval", "next_due_date"}) {
		t.Fatalf("subscription missing_fields = %#v", missing)
	}
}

func TestNormalizeSplitCandidateDetailsSetsLegacyFlag(t *testing.T) {
	entry := map[string]any{
		"title":    "Dinner",
		"amount":   float64(2500),
		"type":     "expense",
		"mode":     "UPI",
		"category": "Food",
		"date":     "2026-07-17",
		"split_candidate_details": map[string]any{
			"participants": []any{
				map[string]any{
					"friend_name":  "Ria",
					"share_amount": float64(1250),
					"direction":    "friend_owes_user",
				},
			},
			"missing_fields": []any{},
		},
	}

	normalizeParsedDraft(entry, "paid 2500 rupees for dinner split with Ria")

	if entry["split_candidate"] != true {
		t.Fatalf("split candidate flag was not normalized: %#v", entry)
	}
}
