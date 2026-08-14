package http

import "strings"

// Canonical payment modes. The single source of truth for what may be saved on
// an entry and what may be filtered on.
//
// This drifted exactly the way the category list did, and stayed hidden longer
// because the two halves lived in different files. validateEntryValues accepted
// "bank account", so entries saved with it happily — 9 of them in the audit
// database. filteredEntriesQuery's whitelist did not, so filtering by Bank
// Account returned 422 invalid_filters, which the app renders as "Unable to
// load entries right now". A mode you can record but cannot search for is worse
// than one you cannot record at all: the data is there and the app denies it.
//
// Meanwhile the filter sheet offered "Debit Card", which appears in neither
// list and in no row, so that chip could only ever produce the same error.
//
// TestCanonicalModesAcceptedOnSaveAndFilter fails if the two ever diverge again.
var canonicalModes = []string{
	"Cash",
	"Bank Account",
	"UPI",
	"Credit Card",
	"Wallets",
}

// modeAliases maps loose or legacy input onto the canonical list. "Debit Card"
// is deliberately absent: the app has no debit-card mode and no row uses one,
// so mapping it to Bank Account would invent a meaning rather than recover one.
var modeAliases = map[string]string{
	"bank":            "Bank Account",
	"savings account": "Bank Account",
	"saving account":  "Bank Account",
	"netbanking":      "Bank Account",
	"net banking":     "Bank Account",
	"wallet":          "Wallets",
	"credit":          "Credit Card",
	"card":            "Credit Card",
}

// canonicalMode resolves a raw value to its canonical form. The second return is
// false when the value is not a payment mode at all.
func canonicalMode(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", false
	}
	for _, mode := range canonicalModes {
		if strings.ToLower(mode) == normalized {
			return mode, true
		}
	}
	if alias, ok := modeAliases[normalized]; ok {
		return alias, true
	}
	return "", false
}

// modeMessage names the accepted values, so a 422 tells the caller what to send.
func modeMessage() string {
	return "must be " + strings.Join(canonicalModes[:len(canonicalModes)-1], ", ") +
		", or " + canonicalModes[len(canonicalModes)-1]
}
