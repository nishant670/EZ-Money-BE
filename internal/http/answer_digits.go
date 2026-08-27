package http

import (
	"strconv"
	"strings"
	"unicode"
)

// The digit validator.
//
// `parse_query.go` states the invariant this file defends: *the model decides
// what to look up and the database decides what the number is.* A one-shot
// answer card can hold that absolutely, because every figure on it is rendered
// from `ledgerAnswer` and the model never writes a character of it.
//
// A conversation cannot. It needs some model-authored sentences, and prose is
// where "you spent about ₹5,000 on food" comes from — almost right, occasionally
// very wrong, and completely unfalsifiable. So the absolute rule becomes an
// enforceable one: **any number in the model's prose that the computed answer
// does not contain causes the whole prose to be discarded** and replaced with a
// sentence the server wrote.
//
// Discarding rather than correcting is deliberate. A sentence with one number
// swapped out is a sentence whose grammar no longer matches its claim, and the
// failure it produces is a confident, fluent falsehood — the exact thing this
// mechanism exists to make impossible.
//
// **The bias is strict, and that is the point.** A false rejection costs a
// plainer sentence that is still true. A false acceptance puts a wrong number in
// front of someone reading it as their own money. So anything ambiguous is
// rejected: "Q4" is treated as containing 4, and "your top 3 categories" is
// rejected unless 3 is genuinely in the answer. Prose that wants to survive can
// simply not quote numbers the answer does not hold.

// answerProseFallback is what the user sees when the model's prose is thrown
// away and the caller offered nothing better. It is deliberately dull: the card
// beside it already carries the figures, so this only has to not lie.
const answerProseFallback = "Here is what your ledger shows."

// numberToken is a canonical form for one number, so that the same quantity
// written two different ways compares equal. "4,820", "4820" and "4820.00" all
// become "4820"; 30.2 stays "30.2".
func numberToken(value string) string {
	cleaned := strings.ReplaceAll(value, ",", "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}
	parsed, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		// Not a number we can reason about — keep the digits as written so an
		// unparseable run still has to match something the answer contains,
		// rather than being waved through.
		return cleaned
	}
	return strconv.FormatFloat(parsed, 'f', -1, 64)
}

// extractNumbers pulls every run of digits out of text, keeping grouping commas
// and a single decimal point attached so "₹1,20,450.50" reads as one number
// rather than four.
//
// Indian grouping is why the comma has to be consumed greedily: 1,20,450 is one
// figure with two separators in it, and splitting on them would produce 1, 20
// and 450 — three numbers the answer will not contain, so every correctly
// formatted lakh would be rejected.
func extractNumbers(text string) []string {
	var found []string
	runes := []rune(text)
	for index := 0; index < len(runes); {
		if !unicode.IsDigit(runes[index]) {
			index++
			continue
		}
		start := index
		seenDot := false
		for index < len(runes) {
			current := runes[index]
			if unicode.IsDigit(current) {
				index++
				continue
			}
			// A separator only continues the number when a digit follows it.
			// "₹4,820." at the end of a sentence must not swallow the full stop.
			if (current == ',' || current == '.') && index+1 < len(runes) && unicode.IsDigit(runes[index+1]) {
				if current == '.' {
					if seenDot {
						break
					}
					seenDot = true
				}
				index++
				continue
			}
			break
		}
		token := numberToken(string(runes[start:index]))
		if token != "" {
			found = append(found, token)
		}
	}
	return found
}

// addNumber records one numeric value, plus the rounded form the app would
// actually display.
//
// The rounded form is not leniency, it is correctness: `formatMoney` renders
// 4819.6 as ₹4,820, so that is the figure on the card and the only figure the
// user can read. Prose quoting what the screen says must not be rejected for
// agreeing with it. Anything further from the truth than the app's own rounding
// still fails, which is the case worth catching — "about ₹5,000" for 4,820 is
// rejected exactly as it should be.
func addNumber(set map[string]struct{}, value float64) {
	set[strconv.FormatFloat(value, 'f', -1, 64)] = struct{}{}
	set[strconv.FormatFloat(roundHalfAwayFromZero(value), 'f', -1, 64)] = struct{}{}
}

func roundHalfAwayFromZero(value float64) float64 {
	if value < 0 {
		return -float64(int64(-value + 0.5))
	}
	return float64(int64(value + 0.5))
}

// addText records every number embedded in a string the answer carries.
//
// Labels and dates count as part of the answer: a period label of "July 2026"
// is what makes "in July 2026" quotable, and an entry dated 2026-07-15 is what
// makes "on the 15th" quotable. These strings came out of the database in the
// same query the figures did, so a number inside one is no less computed than
// the headline is.
func addText(set map[string]struct{}, text string) {
	for _, token := range extractNumbers(text) {
		set[token] = struct{}{}
	}
}

// answerNumbers is every number the computed answer contains, in canonical form.
func answerNumbers(answer ledgerAnswer) map[string]struct{} {
	set := map[string]struct{}{}

	if answer.Amount != nil {
		addNumber(set, *answer.Amount)
	}
	addNumber(set, float64(answer.TransactionCount))

	addText(set, answer.Subject)
	addText(set, answer.Period.Label)
	addText(set, answer.Period.StartDate)
	addText(set, answer.Period.EndDate)
	addText(set, answer.Message)

	for _, slice := range answer.Breakdown {
		addNumber(set, slice.Amount)
		addNumber(set, float64(slice.TransactionCount))
		addNumber(set, slice.Percentage)
		addText(set, slice.Label)
	}

	if entry := answer.LargestEntry; entry != nil {
		addNumber(set, entry.Amount)
		addText(set, entry.Title)
		addText(set, entry.Merchant)
		addText(set, entry.Category)
		addText(set, entry.Date)
	}

	return set
}

// validateAnswerProse decides whether the model's sentence may be shown.
//
// It returns the prose when every number in it is one the answer holds, and the
// fallback otherwise. The boolean says which happened, so a caller can count
// rejections — a rising rejection rate is the signal that a prompt has started
// inventing figures, and it is worth seeing before a user reports it.
//
// Prose with no numbers in it at all passes. There is nothing to check, and the
// sentence cannot be wrong about a figure it never states.
func validateAnswerProse(prose string, answer ledgerAnswer, fallback string) (string, bool) {
	trimmed := strings.TrimSpace(prose)
	if trimmed == "" {
		return strings.TrimSpace(fallback), false
	}
	if strings.TrimSpace(fallback) == "" {
		fallback = answerProseFallback
	}

	allowed := answerNumbers(answer)
	for _, token := range extractNumbers(trimmed) {
		if _, ok := allowed[token]; !ok {
			return strings.TrimSpace(fallback), false
		}
	}
	return trimmed, true
}
