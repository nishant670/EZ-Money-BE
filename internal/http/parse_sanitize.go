package http

import (
	"math"
	"strconv"
	"strings"
	"time"
)

/*
Coercion for everything the model hands back, applied before the draft meets
the JSON schema.

The parse call runs in json_object mode, not against a strict schema, so what
comes back is *shaped like* a draft rather than guaranteed to be one. The schema
is the last gate, and it is an all-or-nothing one: anything it rejects becomes a
422 that has already cost the user a credit and returns no draft at all — no
amount, no merchant, no way forward but retyping the sentence.

That trade was wrong on the nested blocks in particular. A single
`"direction": null` inside a split participant, or a `missing_fields` entry the
model called "participant_shares" instead of "share_amount", threw away a
perfectly readable ₹10,000 rent capture over a hint about the split.

So the rule here is: never reject what can be understood. A value that can be
coerced is coerced, a value that cannot is dropped to null and left for the
confirm-first pass to flag, and the schema only ever sees a draft that already
conforms. The schema keeps its job — it is the assertion that this file did its
own — but it stops being the thing that decides whether a capture survives.
*/

// parseNumberCleaner strips the decoration a model puts on money — currency
// symbols, thousands separators, stray spaces — so "₹10,000" reads as 10000.
var parseNumberCleaner = strings.NewReplacer(
	",", "", " ", "", " ", "", "₹", "", "$", "", "€", "", "£", "",
	"rs.", "", "rs", "", "inr", "", "/-", "",
)

func coerceParseNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return typed, true
	case int:
		return float64(typed), true
	case string:
		cleaned := parseNumberCleaner.Replace(strings.ToLower(strings.TrimSpace(typed)))
		if cleaned == "" {
			return 0, false
		}
		number, err := strconv.ParseFloat(cleaned, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}

// coerceParseAmount folds a signed amount to its magnitude. Direction is carried
// by "type", so a model that writes an expense as -10000 means the same 10000,
// and the schema's `minimum: 0` would otherwise reject the whole draft over it.
func coerceParseAmount(value any) (float64, bool) {
	amount, ok := coerceParseNumber(value)
	if !ok {
		return 0, false
	}
	if amount < 0 {
		amount = -amount
	}
	if amount == 0 {
		return 0, false
	}
	return amount, true
}

func coerceParseBool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "yes", "y", "1":
			return true, true
		case "false", "no", "n", "0":
			return false, true
		}
	case float64:
		return typed != 0, true
	}
	return false, false
}

// normalizeOptionalString trims a text field to nil or a non-empty string.
// Absent keys stay absent — the schema does not require them, and inventing
// nulls would change what every existing response looks like.
func normalizeOptionalString(entry map[string]any, field string) {
	value, present := entry[field]
	if !present {
		return
	}
	text, ok := value.(string)
	if !ok {
		entry[field] = nil
		return
	}
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		entry[field] = trimmed
	} else {
		entry[field] = nil
	}
}

func normalizeOptionalBool(entry map[string]any, field string) {
	value, present := entry[field]
	if !present {
		return
	}
	if coerced, ok := coerceParseBool(value); ok {
		entry[field] = coerced
		return
	}
	entry[field] = nil
}

func normalizeOptionalAmount(entry map[string]any, field string) {
	value, present := entry[field]
	if !present {
		return
	}
	if amount, ok := coerceParseAmount(value); ok {
		entry[field] = amount
		return
	}
	entry[field] = nil
}

// parseDateLayouts are read day-first. The app is INR-denominated and
// India-first, so "02/01/2026" is 2 January, and these only ever rescue a value
// the canonical layout already failed to read.
var parseDateLayouts = []string{
	"2006-01-02",
	"2006/01/02",
	"02-01-2006",
	"02/01/2006",
	"02.01.2006",
	"2 January 2006",
	"2 Jan 2006",
	"January 2, 2006",
	"Jan 2, 2006",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
}

func canonicalParseDate(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}
	for _, layout := range parseDateLayouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.Format("2006-01-02"), true
		}
	}
	return "", false
}

func normalizeOptionalDate(entry map[string]any, field string) {
	value, present := entry[field]
	if !present {
		return
	}
	if date, ok := canonicalParseDate(value); ok {
		entry[field] = date
		return
	}
	entry[field] = nil
}

// parseTimeLayouts cover the 24-hour form the schema wants plus the clock
// wordings a model reaches for on its own ("12:33 AM"), which the schema's
// pattern rejects outright.
var parseTimeLayouts = []string{
	"15:04:05",
	"15:04",
	"3:04:05 PM",
	"3:04 PM",
	"3:04PM",
	"3PM",
	"3:04",
}

func canonicalParseTime(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	trimmed := strings.ToUpper(strings.TrimSpace(text))
	if trimmed == "" {
		return "", false
	}
	for _, layout := range parseTimeLayouts {
		parsed, err := time.Parse(layout, trimmed)
		if err != nil {
			continue
		}
		if parsed.Second() != 0 {
			return parsed.Format("15:04:05"), true
		}
		return parsed.Format("15:04"), true
	}
	return "", false
}

func normalizeParseTime(entry map[string]any) {
	value, present := entry["time"]
	if !present {
		return
	}
	if clock, ok := canonicalParseTime(value); ok {
		entry["time"] = clock
		return
	}
	entry["time"] = nil
}

// filterAliasedList maps a model-authored list of field names onto the enum the
// schema allows, dropping anything unrecognised. These lists are hints about
// what to ask the user next; a hint nobody can act on is worth less than the
// capture it would otherwise take down with it.
func filterAliasedList(value any, aliases map[string]string) []any {
	values, _ := value.([]any)
	filtered := make([]any, 0, len(values))
	seen := map[string]bool{}
	for _, item := range values {
		text, ok := item.(string)
		if !ok {
			continue
		}
		canonical, ok := aliases[strings.ToLower(strings.TrimSpace(text))]
		if !ok || seen[canonical] {
			continue
		}
		seen[canonical] = true
		filtered = append(filtered, canonical)
	}
	return filtered
}

func removeAnyString(value any, unwanted string) []any {
	values, _ := value.([]any)
	filtered := make([]any, 0, len(values))
	for _, item := range values {
		if text, ok := item.(string); ok && text == unwanted {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

var splitMissingFieldAliases = map[string]string{
	"group_name":         "group_name",
	"group":              "group_name",
	"groupname":          "group_name",
	"group_id":           "group_name",
	"participants":       "participants",
	"participant":        "participants",
	"friends":            "participants",
	"members":            "participants",
	"group_members":      "participants",
	"friend_name":        "friend_name",
	"friend":             "friend_name",
	"name":               "friend_name",
	"person":             "friend_name",
	"friend_or_group":    "friend_or_group",
	"group_or_friend":    "friend_or_group",
	"friend_or_group_id": "friend_or_group",
	"share_amount":       "share_amount",
	"share_amounts":      "share_amount",
	"share":              "share_amount",
	"shares":             "share_amount",
	"participant_shares": "share_amount",
	"split_amount":       "share_amount",
	"amount":             "share_amount",
	"direction":          "direction",
	"who_owes":           "direction",
	"owes":               "direction",
}

var subscriptionMissingFieldAliases = map[string]string{
	"name":              "name",
	"service_name":      "name",
	"service":           "name",
	"plan":              "name",
	"merchant":          "merchant",
	"provider":          "merchant",
	"category":          "category",
	"amount":            "amount",
	"price":             "amount",
	"billing_interval":  "billing_interval",
	"interval":          "billing_interval",
	"frequency":         "billing_interval",
	"billing_cycle":     "billing_interval",
	"cycle":             "billing_interval",
	"next_due_date":     "next_due_date",
	"next_payment_date": "next_due_date",
	"next_due":          "next_due_date",
	"due_date":          "next_due_date",
	"last_charged_date": "last_charged_date",
	"last_charged":      "last_charged_date",
	"last_paid_date":    "last_charged_date",
	"last_payment_date": "last_charged_date",
	"reminder_days":     "reminder_days",
	"reminder":          "reminder_days",
	"reminder_day":      "reminder_days",
	"cancel_before_due": "cancel_before_due",
	"cancel":            "cancel_before_due",
	"cancel_on_date":    "cancel_on_date",
	"cancel_date":       "cancel_on_date",
	"autopay":           "autopay",
	"auto_pay":          "autopay",
	"auto_debit":        "autopay",
	"payment_mode":      "payment_mode",
	"mode":              "payment_mode",
	"payment_method":    "payment_mode",
	"notes":             "notes",
	"note":              "notes",
}

var subscriptionIntervalAliases = map[string]string{
	"daily":          "daily",
	"every_day":      "daily",
	"everyday":       "daily",
	"business_daily": "business_daily",
	"businessdaily":  "business_daily",
	"weekday":        "business_daily",
	"weekdays":       "business_daily",
	"market_days":    "business_daily",
	"weekly":         "weekly",
	"every_week":     "weekly",
	"biweekly":       "biweekly",
	"bi_weekly":      "biweekly",
	"fortnightly":    "biweekly",
	"monthly":        "monthly",
	"every_month":    "monthly",
	"quarterly":      "quarterly",
	"yearly":         "yearly",
	"annual":         "yearly",
	"annually":       "yearly",
}

func canonicalSubscriptionInterval(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(text)), " ", "_")
	interval, ok := subscriptionIntervalAliases[key]
	return interval, ok
}

var splitDirectionAliases = map[string]string{
	"friend_owes_user": "friend_owes_user",
	"friend_owes_me":   "friend_owes_user",
	"friend_owes":      "friend_owes_user",
	"they_owe_me":      "friend_owes_user",
	"owed_to_user":     "friend_owes_user",
	"owed_to_me":       "friend_owes_user",
	"user_owes_friend": "user_owes_friend",
	"i_owe_friend":     "user_owes_friend",
	"i_owe_them":       "user_owes_friend",
	"i_owe":            "user_owes_friend",
	"user_owes":        "user_owes_friend",
	"owed_by_user":     "user_owes_friend",
}

// defaultSplitDirection is the shape of nearly every capture that reaches this
// code: the user paid the bill, so the other side owes them. It is the fallback
// for an unreadable direction because the alternative — refusing the draft — is
// strictly worse than a value the user can flip in the review sheet.
const defaultSplitDirection = "friend_owes_user"

func canonicalSplitDirection(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(text)), " ", "_")
	direction, ok := splitDirectionAliases[key]
	return direction, ok
}

// normalizeSplitParticipants rewrites each row into the three keys the schema
// requires, and drops the rows that say nothing.
//
// The empty row is the common case worth naming: told to split across a named
// group whose members it cannot know, the model tends to emit one placeholder
// participant with every field null. That row is not a person. Kept, it fails
// the schema on `direction`; kept and coerced, it would reach the review sheet
// as a nameless participant holding an equal share and quietly displace the
// group's real members. Dropping it lets the app expand the group itself.
func normalizeSplitParticipants(value any) []any {
	rows, _ := value.([]any)
	participants := make([]any, 0, len(rows))
	for _, row := range rows {
		participant, ok := row.(map[string]any)
		if !ok {
			continue
		}
		pruneKeys(participant, allowedSplitParticipantFields)
		normalizeOptionalString(participant, "friend_name")
		name, _ := participant["friend_name"].(string)
		share, hasShare := coerceParseAmount(participant["share_amount"])
		if name == "" && !hasShare {
			continue
		}
		participant["friend_name"] = nil
		if name != "" {
			participant["friend_name"] = name
		}
		participant["share_amount"] = nil
		if hasShare {
			participant["share_amount"] = share
		}
		direction, ok := canonicalSplitDirection(participant["direction"])
		if !ok {
			direction = defaultSplitDirection
		}
		participant["direction"] = direction
		participants = append(participants, participant)
	}
	return participants
}

// normalizeSubscriptionCandidate coerces every field of the recurring block.
// Same reasoning as the split: a subscription hint the model got slightly wrong
// should cost the hint, never the transaction it was attached to.
func normalizeSubscriptionCandidate(candidate map[string]any) {
	for _, field := range []string{"name", "merchant", "notes"} {
		normalizeOptionalString(candidate, field)
	}
	if category, ok := candidate["category"].(string); ok {
		if canonical, ok := canonicalCategory(category); ok {
			candidate["category"] = canonical
		} else {
			normalizeOptionalString(candidate, "category")
		}
	} else if _, present := candidate["category"]; present {
		candidate["category"] = nil
	}
	normalizeOptionalAmount(candidate, "amount")
	for _, field := range []string{"next_due_date", "last_charged_date", "cancel_on_date"} {
		normalizeOptionalDate(candidate, field)
	}
	for _, field := range []string{"cancel_before_due", "autopay"} {
		normalizeOptionalBool(candidate, field)
	}
	if _, present := candidate["billing_interval"]; present {
		if interval, ok := canonicalSubscriptionInterval(candidate["billing_interval"]); ok {
			candidate["billing_interval"] = interval
		} else {
			candidate["billing_interval"] = nil
		}
	}
	if _, present := candidate["payment_mode"]; present {
		mode, _ := candidate["payment_mode"].(string)
		if canonical, ok := canonicalParseMode(mode); ok {
			candidate["payment_mode"] = canonical
		} else {
			candidate["payment_mode"] = nil
		}
	}
	if _, present := candidate["reminder_days"]; present {
		if days, ok := coerceParseNumber(candidate["reminder_days"]); ok {
			rounded := math.Round(days)
			if rounded < 0 {
				rounded = 0
			}
			if rounded > 30 {
				rounded = 30
			}
			candidate["reminder_days"] = rounded
		} else {
			candidate["reminder_days"] = nil
		}
	}
	candidate["missing_fields"] = filterAliasedList(candidate["missing_fields"], subscriptionMissingFieldAliases)
}
