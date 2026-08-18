package http

import "testing"

func spendLine(date, description string, amount int64) statementLine {
	return statementLine{Date: date, Description: description, Amount: rupees(amount), Type: "expense"}
}

func creditLine(date, description string, amount int64) statementLine {
	return statementLine{Date: date, Description: description, Amount: rupees(amount), Type: "income"}
}

func ledgerSpend(id uint, date, title string, amount int64) ledgerLine {
	return ledgerLine{EntryID: id, Date: date, Title: title, Amount: rupees(amount), Type: "expense"}
}

func TestClassifyLine(t *testing.T) {
	cases := []struct {
		name string
		line statementLine
		want string
	}{
		{"an ordinary purchase", spendLine("2026-07-10", "SWIGGY BANGALORE IN", 480), lineKindSpend},
		{"a refund", creditLine("2026-07-18", "AMAZON REFUND", 800), lineKindRefund},
		{"a bill payment", creditLine("2026-07-25", "PAYMENT RECEIVED - THANK YOU", 12400), lineKindPayment},
		{"an autopay debit of the bill", creditLine("2026-07-25", "AUTOPAY SETTLEMENT", 12400), lineKindPayment},
		{"interest", spendLine("2026-08-05", "FINANCE CHARGE", 620), lineKindInterest},
		{"a late fee", spendLine("2026-08-05", "LATE PAYMENT FEE", 500), lineKindFee},
		{"GST on a fee", spendLine("2026-08-05", "IGST @18%", 90), lineKindFee},
		{"an EMI instalment", spendLine("2026-07-20", "EMI INSTALMENT 3/12 CROMA", 5000), lineKindEMI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyLine(tc.line); got != tc.want {
				t.Fatalf("kind = %q, want %q", got, tc.want)
			}
		})
	}
}

// The everyday case: some of the bill is already logged, some is not, and one
// thing in Finnri was never billed.
func TestDiffSortsLinesIntoThreeBuckets(t *testing.T) {
	lines := []statementLine{
		spendLine("2026-07-10", "SWIGGY BANGALORE IN", 480),
		spendLine("2026-07-14", "UBER INDIA SYSTEMS", 260),
		spendLine("2026-07-20", "CROMA RETAIL MUMBAI", 5000),
	}
	entries := []ledgerLine{
		ledgerSpend(1, "2026-07-10", "Dinner", 480),
		ledgerSpend(2, "2026-07-28", "Groceries", 1200),
	}

	diff := diffStatementLines(lines, entries)

	if len(diff.Matched) != 1 || diff.Matched[0].Entry.EntryID != 1 {
		t.Fatalf("matched = %+v, want the ₹480 pair", diff.Matched)
	}
	if len(diff.Missing) != 2 {
		t.Fatalf("missing = %d, want 2", len(diff.Missing))
	}
	if diff.Summary.MissingAmount != rupees(5260) {
		t.Fatalf("missing amount = %s, want %s", diff.Summary.MissingAmount, rupees(5260))
	}
	if len(diff.Extra) != 1 || diff.Extra[0].EntryID != 2 {
		t.Fatalf("extra = %+v, want the ₹1,200 entry", diff.Extra)
	}
}

// A purchase posts to the statement a day or two after it happened, and the
// user logged it when it happened. Refusing to match across that gap would
// report almost everything as missing.
func TestDiffMatchesAcrossAPostingDelay(t *testing.T) {
	lines := []statementLine{spendLine("2026-07-13", "BIGBASKET", 1499)}
	entries := []ledgerLine{ledgerSpend(1, "2026-07-11", "Weekly veg", 1499)}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 1 {
		t.Fatalf("matched = %d, want 1 across a two-day delay", len(diff.Matched))
	}
	if diff.Matched[0].DayGap != 2 {
		t.Fatalf("day gap = %d, want 2", diff.Matched[0].DayGap)
	}
}

// Beyond the window it is a different transaction — most obviously a monthly
// subscription of the same amount.
func TestDiffWillNotMatchBeyondTheDateWindow(t *testing.T) {
	lines := []statementLine{spendLine("2026-07-20", "NETFLIX", 649)}
	entries := []ledgerLine{ledgerSpend(1, "2026-06-20", "Netflix", 649)}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 0 {
		t.Fatalf("matched a month apart: %+v", diff.Matched)
	}
	if len(diff.Missing) != 1 || len(diff.Extra) != 1 {
		t.Fatalf("want one missing and one extra, got %d and %d", len(diff.Missing), len(diff.Extra))
	}
}

// Two identical amounts on the same day must pair off one-to-one rather than
// both matching the first entry.
func TestDiffPairsDuplicateAmountsOneToOne(t *testing.T) {
	lines := []statementLine{
		spendLine("2026-07-10", "STARBUCKS", 250),
		spendLine("2026-07-10", "STARBUCKS", 250),
	}
	entries := []ledgerLine{
		ledgerSpend(1, "2026-07-10", "Coffee", 250),
		ledgerSpend(2, "2026-07-10", "Coffee", 250),
	}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 2 {
		t.Fatalf("matched = %d, want 2", len(diff.Matched))
	}
	if diff.Matched[0].Entry.EntryID == diff.Matched[1].Entry.EntryID {
		t.Fatal("the same entry was matched twice")
	}
	if len(diff.Missing) != 0 || len(diff.Extra) != 0 {
		t.Fatalf("nothing should be left over: missing %d, extra %d",
			len(diff.Missing), len(diff.Extra))
	}
}

// Three ₹250 lines against two ₹250 entries leaves exactly one missing.
func TestDiffLeavesTheSurplusDuplicateMissing(t *testing.T) {
	lines := []statementLine{
		spendLine("2026-07-10", "STARBUCKS", 250),
		spendLine("2026-07-10", "STARBUCKS", 250),
		spendLine("2026-07-11", "STARBUCKS", 250),
	}
	entries := []ledgerLine{
		ledgerSpend(1, "2026-07-10", "Coffee", 250),
		ledgerSpend(2, "2026-07-10", "Coffee", 250),
	}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 2 || len(diff.Missing) != 1 {
		t.Fatalf("matched %d and missing %d, want 2 and 1", len(diff.Matched), len(diff.Missing))
	}
}

// A ₹2,000 refund is not a ₹2,000 purchase.
func TestDiffWillNotMatchACreditToADebit(t *testing.T) {
	lines := []statementLine{creditLine("2026-07-18", "AMAZON REFUND", 2000)}
	entries := []ledgerLine{ledgerSpend(1, "2026-07-18", "Amazon order", 2000)}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 0 {
		t.Fatalf("a refund matched a purchase: %+v", diff.Matched)
	}
}

// Refunds do match refunds.
func TestDiffMatchesARefundToAnIncomeEntry(t *testing.T) {
	lines := []statementLine{creditLine("2026-07-18", "AMAZON REFUND", 800)}
	entries := []ledgerLine{{
		EntryID: 1, Date: "2026-07-18", Title: "Returned order",
		Amount: rupees(800), Type: "income",
	}}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 1 {
		t.Fatalf("refund did not match its income entry: %+v", diff)
	}
}

// The important safety property. Finnri tracks bill payments on the statement,
// never as card entries — outstanding comes from the statement, not from
// ledger arithmetic. Offering a payment for import would reduce the card's
// outstanding a second time.
func TestBillPaymentsAreNeverOfferedForImport(t *testing.T) {
	lines := []statementLine{
		creditLine("2026-07-25", "PAYMENT RECEIVED - THANK YOU", 12400),
		spendLine("2026-07-10", "SWIGGY", 480),
	}

	diff := diffStatementLines(lines, nil)

	for _, line := range diff.Missing {
		if line.Kind == lineKindPayment {
			t.Fatalf("a bill payment was offered for import: %+v", line)
		}
	}
	if len(diff.Ignored) != 1 || diff.Ignored[0].Kind != lineKindPayment {
		t.Fatalf("the payment should be surfaced as ignored, got %+v", diff.Ignored)
	}
	if diff.Summary.MissingAmount != rupees(480) {
		t.Fatalf("missing amount = %s, want just the ₹480 spend", diff.Summary.MissingAmount)
	}
}

// Fees and interest are real spending the user almost never logs, so they
// should come through as importable rather than being filtered out with
// payments.
func TestFeesAndInterestAreImportable(t *testing.T) {
	lines := []statementLine{
		spendLine("2026-08-05", "FINANCE CHARGE", 620),
		spendLine("2026-08-05", "LATE PAYMENT FEE", 500),
	}

	diff := diffStatementLines(lines, nil)
	if len(diff.Missing) != 2 {
		t.Fatalf("missing = %d, want both the charge and the fee", len(diff.Missing))
	}
	if diff.Summary.MissingAmount != rupees(1120) {
		t.Fatalf("missing amount = %s, want %s", diff.Summary.MissingAmount, rupees(1120))
	}
}

func TestDescriptionBreaksTiesBetweenEqualDates(t *testing.T) {
	lines := []statementLine{spendLine("2026-07-10", "SWIGGY BANGALORE IN", 500)}
	entries := []ledgerLine{
		ledgerSpend(1, "2026-07-10", "Cab home", 500),
		{EntryID: 2, Date: "2026-07-10", Title: "Dinner", Merchant: "Swiggy",
			Amount: rupees(500), Type: "expense"},
	}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 1 {
		t.Fatalf("matched = %d, want 1", len(diff.Matched))
	}
	if diff.Matched[0].Entry.EntryID != 2 {
		t.Fatalf("matched entry %d, want the Swiggy one", diff.Matched[0].Entry.EntryID)
	}
}

// A true match usually scores low on words, and must still match.
func TestLowDescriptionSimilarityStillMatches(t *testing.T) {
	lines := []statementLine{spendLine("2026-07-10", "POS 4523 IND MUMBAI", 3400)}
	entries := []ledgerLine{ledgerSpend(1, "2026-07-10", "New shoes", 3400)}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 1 {
		t.Fatal("an unrecognisable bank description blocked a real match")
	}
	if diff.Matched[0].Similarity != 0 {
		t.Fatalf("similarity = %v, want 0 for entirely different words",
			diff.Matched[0].Similarity)
	}
}

func TestDescriptionSimilarityIgnoresProcessorNoise(t *testing.T) {
	// Sharing only "upi", "pvt" and "ltd" is not evidence of anything.
	if score := describeSimilarity("UPI PVT LTD INDIA", "UPI PVT LTD INDIA COM"); score != 0 {
		t.Fatalf("noise-only similarity = %v, want 0", score)
	}
	if score := describeSimilarity("SWIGGY BANGALORE", "Swiggy dinner"); score <= 0 {
		t.Fatalf("shared merchant similarity = %v, want > 0", score)
	}
}

// An empty statement means everything Finnri holds is unbilled, not that
// everything matched.
func TestDiffWithNoStatementLines(t *testing.T) {
	entries := []ledgerLine{ledgerSpend(1, "2026-07-10", "Dinner", 480)}
	diff := diffStatementLines(nil, entries)

	if len(diff.Extra) != 1 || len(diff.Matched) != 0 || len(diff.Missing) != 0 {
		t.Fatalf("unexpected diff: %+v", diff.Summary)
	}
}

// Buckets are always arrays so the app never has to guard against null.
func TestDiffBucketsAreNeverNil(t *testing.T) {
	diff := diffStatementLines(nil, nil)
	if diff.Matched == nil || diff.Missing == nil || diff.Extra == nil || diff.Ignored == nil {
		t.Fatalf("a bucket came back nil: %+v", diff)
	}
}

// A line the parser could not date cannot be matched on date, and must not
// silently match everything.
func TestUnparseableDatesDoNotMatch(t *testing.T) {
	lines := []statementLine{{Date: "not-a-date", Description: "SWIGGY", Amount: rupees(480), Type: "expense"}}
	entries := []ledgerLine{ledgerSpend(1, "2026-07-10", "Dinner", 480)}

	diff := diffStatementLines(lines, entries)
	if len(diff.Matched) != 0 {
		t.Fatalf("a line with a broken date matched: %+v", diff.Matched)
	}
	if len(diff.Missing) != 1 || len(diff.Extra) != 1 {
		t.Fatal("the line and the entry should each stand alone")
	}
}
