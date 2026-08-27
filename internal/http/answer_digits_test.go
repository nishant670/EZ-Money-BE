package http

import "testing"

func floatPointer(value float64) *float64 { return &value }

// The answer the rest of these tests quote from, and the shape the plan's own
// worked example uses: ₹4,820 on Food & Drinks last month.
func foodAnswer() ledgerAnswer {
	return ledgerAnswer{
		Status:           "answered",
		Metric:           "spend_total",
		Amount:           floatPointer(4820),
		Currency:         "INR",
		TransactionCount: 12,
		Subject:          "Food & Drinks",
		Period:           ledgerAnswerPeriod{Kind: "last_month", Label: "July 2026", StartDate: "2026-07-01", EndDate: "2026-07-31"},
	}
}

func TestProseSurvivesWhenEveryNumberIsTheAnswers(t *testing.T) {
	prose := "You spent ₹4,820 on Food & Drinks across 12 transactions in July 2026."

	got, ok := validateAnswerProse(prose, foodAnswer(), "fallback")
	if !ok {
		t.Fatalf("prose was rejected: %q", got)
	}
	if got != prose {
		t.Fatalf("got %q, want the prose unchanged", got)
	}
}

// The failure the whole mechanism exists for: a figure that is close, plausible
// and not what the database said.
func TestProseIsDiscardedWhenItRoundsTheNumberItself(t *testing.T) {
	got, ok := validateAnswerProse("You spent about ₹5,000 on food last month.", foodAnswer(), "Here is what your ledger shows.")
	if ok {
		t.Fatal("an invented figure was allowed through")
	}
	if got != "Here is what your ledger shows." {
		t.Fatalf("got %q, want the server sentence", got)
	}
}

// Discarded, not patched. A sentence with one number swapped is a sentence whose
// grammar no longer matches its claim.
func TestRejectedProseIsReplacedWholeRatherThanEdited(t *testing.T) {
	got, _ := validateAnswerProse("You spent ₹9,999 on Food & Drinks.", foodAnswer(), "Here is what your ledger shows.")
	if got == "You spent ₹4,820 on Food & Drinks." {
		t.Fatal("the prose was corrected in place instead of dropped")
	}
	if got != "Here is what your ledger shows." {
		t.Fatalf("got %q", got)
	}
}

// The user reads the rendered figure, so quoting it must not be punished.
func TestTheAppsOwnRoundingIsAccepted(t *testing.T) {
	answer := foodAnswer()
	answer.Amount = floatPointer(4819.6)

	if _, ok := validateAnswerProse("You spent ₹4,820 last month.", answer, "fallback"); !ok {
		t.Fatal("the figure the card displays was rejected")
	}
	// Anything further out than the app's own rounding still fails.
	if _, ok := validateAnswerProse("You spent ₹4,900 last month.", answer, "fallback"); ok {
		t.Fatal("a figure the card never showed was allowed")
	}
}

// Indian grouping is one number with two separators, not three numbers.
func TestLakhGroupingReadsAsASingleNumber(t *testing.T) {
	numbers := extractNumbers("₹1,20,450.50 spent")
	if len(numbers) != 1 || numbers[0] != "120450.5" {
		t.Fatalf("extractNumbers = %#v, want one canonical 120450.5", numbers)
	}

	answer := foodAnswer()
	answer.Amount = floatPointer(120450.5)
	if _, ok := validateAnswerProse("You spent ₹1,20,450.50 last month.", answer, "fallback"); !ok {
		t.Fatal("a correctly grouped lakh figure was rejected")
	}
}

// A full stop ending a sentence is not part of the number before it.
func TestSentenceEndingPunctuationIsNotSwallowed(t *testing.T) {
	numbers := extractNumbers("You spent 4820. That is a lot.")
	if len(numbers) != 1 || numbers[0] != "4820" {
		t.Fatalf("extractNumbers = %#v, want just 4820", numbers)
	}
}

// The same quantity written three ways is one quantity.
func TestFormattingDifferencesCompareEqual(t *testing.T) {
	for _, written := range []string{"4820", "4,820", "4820.00"} {
		if got := numberToken(written); got != "4820" {
			t.Fatalf("numberToken(%q) = %q, want 4820", written, got)
		}
	}
}

// Labels and dates are part of the computed answer, so numbers inside them are
// quotable — the period is what the question was *about*.
func TestNumbersInsideLabelsAndDatesAreQuotable(t *testing.T) {
	answer := foodAnswer()
	answer.LargestEntry = &ledgerAnswerEntry{
		Title:    "Swiggy",
		Merchant: "Swiggy",
		Category: "Food & Drinks",
		Amount:   980,
		Date:     "2026-07-15",
	}

	if _, ok := validateAnswerProse("In July 2026 your largest was ₹980 on 2026-07-15.", answer, "fallback"); !ok {
		t.Fatal("numbers carried by the answer's own labels were rejected")
	}
}

// Breakdown slices are computed too.
func TestBreakdownFiguresAreQuotable(t *testing.T) {
	answer := foodAnswer()
	answer.GroupBy = "category"
	answer.Breakdown = []ledgerAnswerSlice{
		{Label: "Food & Drinks", Amount: 3200, TransactionCount: 8, Percentage: 66.4},
		{Label: "Transport", Amount: 1620, TransactionCount: 4, Percentage: 33.6},
	}

	if _, ok := validateAnswerProse("Food & Drinks took ₹3,200, or 66.4% of it.", answer, "fallback"); !ok {
		t.Fatal("a breakdown figure was rejected")
	}
	if _, ok := validateAnswerProse("Food & Drinks took ₹3,200, or 70% of it.", answer, "fallback"); ok {
		t.Fatal("an invented percentage was allowed")
	}
}

// Nothing to check means nothing to get wrong.
func TestProseWithoutNumbersPasses(t *testing.T) {
	prose := "Your spending was concentrated in a single category."
	got, ok := validateAnswerProse(prose, foodAnswer(), "fallback")
	if !ok || got != prose {
		t.Fatalf("got %q, ok = %v", got, ok)
	}
}

// The strict bias, stated as a test so nobody loosens it by accident: a number
// that is merely *reasonable* is still rejected when the answer does not hold
// it. Prose that wants to survive can decline to quote figures.
func TestPlausibleButUncomputedNumbersAreStillRejected(t *testing.T) {
	if _, ok := validateAnswerProse("That is roughly 3 meals a week.", foodAnswer(), "fallback"); ok {
		t.Fatal("a number the answer never computed was allowed through")
	}
}

// An empty model response is not a pass. There is no sentence to show, so the
// caller's own sentence is what the user gets.
func TestEmptyProseFallsBack(t *testing.T) {
	got, ok := validateAnswerProse("   ", foodAnswer(), "Here is what your ledger shows.")
	if ok {
		t.Fatal("empty prose reported as validated")
	}
	if got != "Here is what your ledger shows." {
		t.Fatalf("got %q", got)
	}
}

// A caller that supplies no fallback still must not surface rejected prose.
func TestRejectionWithoutAFallbackUsesTheDefaultSentence(t *testing.T) {
	got, ok := validateAnswerProse("You spent ₹7,777.", foodAnswer(), "")
	if ok {
		t.Fatal("an invented figure was allowed")
	}
	if got != answerProseFallback {
		t.Fatalf("got %q, want the default sentence", got)
	}
}

// The transaction count is a number the answer holds, and a wrong one is the
// same class of defect as a wrong amount.
func TestTransactionCountsAreCheckedToo(t *testing.T) {
	if _, ok := validateAnswerProse("Across 12 transactions.", foodAnswer(), "fallback"); !ok {
		t.Fatal("the real count was rejected")
	}
	if _, ok := validateAnswerProse("Across 20 transactions.", foodAnswer(), "fallback"); ok {
		t.Fatal("an invented count was allowed")
	}
}
