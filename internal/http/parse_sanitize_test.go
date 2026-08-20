package http

import (
	"encoding/json"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

// The sentence that started this: a real capture with a group split attached.
const groupSplitTranscript = "I paid 10000 as an advance payment to landlord for rented house, " +
	"paid by UPI. split this expense in the group bubu-dudu"

func normalizeAndValidate(t *testing.T, schema *gojsonschema.Schema, raw, transcript string) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatal(err)
	}
	normalizeParsedDraft(entry, transcript)
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	result, err := schema.Validate(gojsonschema.NewBytesLoader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid() {
		for _, e := range result.Errors() {
			t.Errorf("schema error: %s", e.String())
		}
		t.Fatalf("normalized draft failed the schema: %s", encoded)
	}
	return entry
}

// A model told to split across a group it cannot enumerate writes one
// participant of nulls. That row used to fail the schema on `direction` and
// take the whole ₹10,000 capture down with it.
func TestNormalizeDropsPlaceholderSplitParticipant(t *testing.T) {
	schema := parseDraftSchema(t)
	entry := normalizeAndValidate(t, schema, `{
		"type":"expense","title":"Rent advance","amount":10000,"mode":"UPI",
		"category":"Bills","merchant":"Landlord","date":"2026-08-19",
		"split_candidate":true,
		"split_candidate_details":{
			"group_name":"bubu-dudu",
			"participants":[{"friend_name":null,"share_amount":null,"direction":null}],
			"missing_fields":["share_amount"]
		}
	}`, groupSplitTranscript)

	if entry["amount"] != float64(10000) {
		t.Fatalf("amount = %#v, want 10000", entry["amount"])
	}
	candidate := entry["split_candidate_details"].(map[string]any)
	if participants := candidate["participants"].([]any); len(participants) != 0 {
		t.Fatalf("placeholder participant survived: %#v", participants)
	}
	if candidate["group_name"] != "bubu-dudu" || entry["split_candidate"] != true {
		t.Fatalf("group split was lost: %#v", candidate)
	}
}

func TestNormalizeSplitParticipantCoercesShareAndDirection(t *testing.T) {
	schema := parseDraftSchema(t)
	entry := normalizeAndValidate(t, schema, `{
		"type":"expense","title":"Dinner","amount":2500,"mode":"UPI",
		"category":"Food & Drinks","date":"2026-08-19",
		"split_candidate":true,
		"split_candidate_details":{
			"group_name":null,
			"participants":[{"friend_name":" Ria ","share_amount":"1,250","direction":"they owe me"}],
			"missing_fields":["participant_shares","who_pays"]
		}
	}`, "dinner 2500 split with Ria")

	candidate := entry["split_candidate_details"].(map[string]any)
	participant := candidate["participants"].([]any)[0].(map[string]any)
	if participant["friend_name"] != "Ria" || participant["share_amount"] != float64(1250) {
		t.Fatalf("participant was not coerced: %#v", participant)
	}
	if participant["direction"] != "friend_owes_user" {
		t.Fatalf("direction = %#v, want friend_owes_user", participant["direction"])
	}
	missing := candidate["missing_fields"].([]any)
	if len(missing) != 1 || missing[0] != "share_amount" {
		t.Fatalf("missing_fields = %#v, want [share_amount]", missing)
	}
}

func TestNormalizeSplitIntentWithoutDetailsAsksWhoToSplitWith(t *testing.T) {
	schema := parseDraftSchema(t)
	entry := normalizeAndValidate(t, schema, `{
		"type":"expense","title":"Cab","amount":400,"mode":"UPI",
		"category":"Transport","date":"2026-08-19","split_candidate":true
	}`, "cab 400, split it")

	candidate, ok := entry["split_candidate_details"].(map[string]any)
	if !ok {
		t.Fatalf("split intent lost its detail block: %#v", entry)
	}
	missing := candidate["missing_fields"].([]any)
	if len(missing) != 1 || missing[0] != "friend_or_group" {
		t.Fatalf("missing_fields = %#v, want [friend_or_group]", missing)
	}
}

func TestNormalizeSubscriptionMissingFieldsMapOntoTheEnum(t *testing.T) {
	schema := parseDraftSchema(t)
	entry := normalizeAndValidate(t, schema, `{
		"type":"expense","title":"House rent","amount":10000,"mode":"UPI",
		"category":"Bills","merchant":"Landlord","date":"2026-08-19",
		"recurring_candidate":true,
		"subscription_candidate":{
			"name":"House rent","amount":"10000","billing_interval":"every month",
			"reminder_days":"3","autopay":"no","payment_mode":"upi",
			"next_due_date":"19-09-2026","cancel_before_due":false,
			"missing_fields":["service_name","next_payment_date"]
		}
	}`, groupSplitTranscript)

	candidate := entry["subscription_candidate"].(map[string]any)
	if candidate["billing_interval"] != "monthly" {
		t.Fatalf("billing_interval = %#v, want monthly", candidate["billing_interval"])
	}
	if candidate["amount"] != float64(10000) || candidate["reminder_days"] != float64(3) {
		t.Fatalf("subscription numbers were not coerced: %#v", candidate)
	}
	if candidate["next_due_date"] != "2026-09-19" {
		t.Fatalf("next_due_date = %#v, want 2026-09-19", candidate["next_due_date"])
	}
	missing := candidate["missing_fields"].([]any)
	if len(missing) != 1 || missing[0] != "name" {
		t.Fatalf("missing_fields = %#v, want [name]", missing)
	}
}

func TestNormalizeCoercesLooseRootFields(t *testing.T) {
	schema := parseDraftSchema(t)
	entry := normalizeAndValidate(t, schema, `{
		"type":"expense","title":"  Rent advance  ","amount":"₹10,000","mode":"UPI",
		"category":"Bills","merchant":"Landlord","date":"19/08/2026","time":"12:33 AM",
		"split_candidate":"true",
		"split_candidate_details":{"group_name":"bubu-dudu","participants":[],"missing_fields":[]}
	}`, groupSplitTranscript)

	if entry["amount"] != float64(10000) {
		t.Fatalf("amount = %#v, want 10000", entry["amount"])
	}
	if entry["title"] != "Rent advance" {
		t.Fatalf("title = %#v, want trimmed", entry["title"])
	}
	if entry["date"] != "2026-08-19" || entry["time"] != "00:33" {
		t.Fatalf("date/time were not canonicalised: %#v %#v", entry["date"], entry["time"])
	}
	if entry["split_candidate"] != true {
		t.Fatalf("split_candidate = %#v, want true", entry["split_candidate"])
	}
}

func TestNormalizeFoldsNegativeAmountToItsMagnitude(t *testing.T) {
	schema := parseDraftSchema(t)
	entry := normalizeAndValidate(t, schema, `{
		"type":"expense","title":"Rent","amount":-10000,"mode":"UPI",
		"category":"Bills","date":"2026-08-19"
	}`, "paid 10000 rent")

	if entry["amount"] != float64(10000) {
		t.Fatalf("amount = %#v, want 10000", entry["amount"])
	}
	if missing := entry["missing_fields"].([]string); len(missing) != 0 {
		t.Fatalf("a readable amount should not be missing: %v", missing)
	}
}
