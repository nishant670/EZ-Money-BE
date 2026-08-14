package http

import (
	"math"
	"strings"
	"testing"
	"time"
)

var comparisonZone = time.FixedZone("IST", 5*60*60+30*60)

func comparisonDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, comparisonZone)
}

func TestMonthToDateComparesAgainstTheSameDaysOfThePreviousMonth(t *testing.T) {
	// The audit case: 11 days of August. The old rule compared this with
	// Jul 21–31, the tail of the previous month, which on real data held six
	// transactions against thirty-four and produced a four-digit percentage.
	start, end, kind := previousComparisonWindow(
		comparisonDate(2026, time.August, 1),
		comparisonDate(2026, time.August, 11),
	)

	if kind != comparisonSameDaysPreviousMonth {
		t.Fatalf("expected a same-days comparison for month-to-date, got %q", kind)
	}
	if got := start.Format("2006-01-02"); got != "2026-07-01" {
		t.Fatalf("expected the previous month to start on the 1st, got %s", got)
	}
	if got := end.Format("2006-01-02"); got != "2026-07-11" {
		t.Fatalf("expected the same day count, got %s", got)
	}
}

func TestFullMonthComparesAgainstTheWholePreviousMonthClampedToItsLength(t *testing.T) {
	// March has 31 days and February does not. The comparison has to stop at
	// the end of February rather than reach for dates it does not have.
	start, end, kind := previousComparisonWindow(
		comparisonDate(2026, time.March, 1),
		comparisonDate(2026, time.March, 31),
	)

	if kind != comparisonSameDaysPreviousMonth {
		t.Fatalf("expected a same-days comparison, got %q", kind)
	}
	if got := start.Format("2006-01-02"); got != "2026-02-01" {
		t.Fatalf("unexpected previous start: %s", got)
	}
	if got := end.Format("2006-01-02"); got != "2026-02-28" {
		t.Fatalf("expected February to clamp at its last day, got %s", got)
	}
}

func TestMonthToDateComparisonCrossesTheYearBoundary(t *testing.T) {
	start, end, _ := previousComparisonWindow(
		comparisonDate(2026, time.January, 1),
		comparisonDate(2026, time.January, 9),
	)

	if got := start.Format("2006-01-02"); got != "2025-12-01" {
		t.Fatalf("expected December of the previous year, got %s", got)
	}
	if got := end.Format("2006-01-02"); got != "2025-12-09" {
		t.Fatalf("unexpected previous end: %s", got)
	}
}

func TestArbitraryRangesKeepThePrecedingEqualLengthWindow(t *testing.T) {
	// "Last 30 days" and other custom ranges have no calendar meaning, so the
	// window immediately before them stays the honest comparison.
	start, end, kind := previousComparisonWindow(
		comparisonDate(2026, time.August, 5),
		comparisonDate(2026, time.August, 14),
	)

	if kind != comparisonPrecedingPeriod {
		t.Fatalf("expected the preceding-period rule, got %q", kind)
	}
	if got := start.Format("2006-01-02"); got != "2026-07-26" {
		t.Fatalf("unexpected previous start: %s", got)
	}
	if got := end.Format("2006-01-02"); got != "2026-08-04" {
		t.Fatalf("unexpected previous end: %s", got)
	}
}

func TestComparisonFloorSuppressesThinBaselines(t *testing.T) {
	cases := []struct {
		name       string
		base       comparisonBase
		publishing bool
	}{
		{"a real habit", comparisonBase{Amount: 15644, Count: 15}, true},
		{"exactly at the floor", comparisonBase{Amount: 500, Count: 5}, true},
		{"one large payment", comparisonBase{Amount: 40000, Count: 1}, false},
		{"many tiny payments", comparisonBase{Amount: 120, Count: 12}, false},
		{"a quiet stretch", comparisonBase{Amount: 300, Count: 2}, false},
		{"nothing at all", comparisonBase{}, false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := test.base.publishable(); got != test.publishing {
				t.Fatalf("publishable() = %v, want %v for %#v", got, test.publishing, test.base)
			}
		})
	}
}

func TestLargeChangesRenderAsAMultiplierNotFourDigits(t *testing.T) {
	cases := []struct {
		change float64
		want   string
	}{
		{change: 0, want: "0%"},
		{change: 45.4, want: "45%"},
		{change: -62, want: "62%"},
		{change: 159, want: "159%"},
		{change: 300, want: "300%"}, // the boundary stays a percentage
		{change: 301, want: "4×"},   // 1 + 3.01, rendered without a stray .0
		{change: 350, want: "4.5×"},
		{change: 1218, want: "13×"}, // the real Aug-vs-Jul-tail figure
		{change: 2127, want: "22×"},
	}

	for _, test := range cases {
		if got := formatChangeMagnitude(test.change); got != test.want {
			t.Fatalf("formatChangeMagnitude(%v) = %q, want %q", test.change, got, test.want)
		}
	}
}

func TestComparisonLabelNamesBothWindows(t *testing.T) {
	monthToDate := dashboardRange{
		Start:          comparisonDate(2026, time.August, 1),
		End:            comparisonDate(2026, time.August, 11),
		PreviousStart:  comparisonDate(2026, time.July, 1),
		PreviousEnd:    comparisonDate(2026, time.July, 11),
		Days:           11,
		ComparisonKind: comparisonSameDaysPreviousMonth,
	}
	if got := comparisonLabel(monthToDate); got != "Aug 1–11 vs Jul 1–11" {
		t.Fatalf("unexpected label: %q", got)
	}
	if explanation := comparisonExplanation(monthToDate); !strings.Contains(explanation, "same days of the previous month") {
		t.Fatalf("explanation should state the rule, got %q", explanation)
	}

	straddling := dashboardRange{
		Start:          comparisonDate(2026, time.August, 5),
		End:            comparisonDate(2026, time.August, 14),
		PreviousStart:  comparisonDate(2026, time.July, 26),
		PreviousEnd:    comparisonDate(2026, time.August, 4),
		Days:           10,
		ComparisonKind: comparisonPrecedingPeriod,
	}
	if got := comparisonLabel(straddling); got != "Aug 5–14 vs Jul 26 – Aug 4" {
		t.Fatalf("unexpected straddling label: %q", got)
	}

	singleDay := dashboardRange{
		Start:          comparisonDate(2026, time.August, 5),
		End:            comparisonDate(2026, time.August, 5),
		PreviousStart:  comparisonDate(2026, time.August, 4),
		PreviousEnd:    comparisonDate(2026, time.August, 4),
		Days:           1,
		ComparisonKind: comparisonPrecedingPeriod,
	}
	if got := comparisonLabel(singleDay); got != "Aug 5 vs Aug 4" {
		t.Fatalf("unexpected single-day label: %q", got)
	}
}

func TestParseDashboardRangeDefaultsToMonthToDateComparison(t *testing.T) {
	// The default range — no query params — is the one the Insights tab opens
	// on, so it is the one that has to be right.
	dateRange, fields := parseDashboardRange("", "", comparisonDate(2026, time.August, 11))
	if len(fields) > 0 {
		t.Fatalf("unexpected validation errors: %v", fields)
	}
	if dateRange.ComparisonKind != comparisonSameDaysPreviousMonth {
		t.Fatalf("expected same-days comparison by default, got %q", dateRange.ComparisonKind)
	}
	period := dateRange.period()
	if period.PreviousStart != "2026-07-01" || period.PreviousEnd != "2026-07-11" {
		t.Fatalf("unexpected previous window: %s to %s", period.PreviousStart, period.PreviousEnd)
	}
	if period.ComparisonLabel != "Aug 1–11 vs Jul 1–11" {
		t.Fatalf("unexpected comparison label: %q", period.ComparisonLabel)
	}
}

func TestInsightCardsStateTheComparisonWindowAndCapTheMagnitude(t *testing.T) {
	dateRange := dashboardRange{
		Start:          comparisonDate(2026, time.August, 1),
		End:            comparisonDate(2026, time.August, 11),
		PreviousStart:  comparisonDate(2026, time.July, 1),
		PreviousEnd:    comparisonDate(2026, time.July, 11),
		Days:           11,
		ComparisonKind: comparisonSameDaysPreviousMonth,
	}
	dashboard := DashboardResponse{
		Summary: DashboardSummary{TotalSpent: 40486},
		TopCategories: []DashboardCategory{
			{Category: "Food & Drinks", Amount: 12114, Percentage: 30, Change: 620, ChangeComparable: true},
		},
	}
	base := comparisonBase{Amount: 15644, Count: 15}

	cards := buildInsightCards(dashboard, dateRange, base, map[string]comparisonBase{
		"Food & Drinks": {Amount: 1683, Count: 9},
	}, nil)

	period := insightByKind(cards, "period_comparison")
	if period == nil {
		t.Fatalf("expected a period comparison off a solid baseline: %#v", cards)
	}
	if period.Body != "Spending is 159% higher than Jul 1–11." {
		t.Fatalf("period card should name the window it compared: %q", period.Body)
	}

	category := insightByKind(cards, "category_increase")
	if category == nil {
		t.Fatalf("expected a category increase: %#v", cards)
	}
	// 620% is exactly the sort of figure that used to ship as four digits.
	if category.Body != "Food & Drinks spending is 7.2× higher than Jul 1–11." {
		t.Fatalf("category card should read as a multiplier: %q", category.Body)
	}
}

func TestInsightCardsWithholdComparisonsOnAThinBaseline(t *testing.T) {
	dateRange := dashboardRange{
		Start:          comparisonDate(2026, time.August, 1),
		End:            comparisonDate(2026, time.August, 11),
		PreviousStart:  comparisonDate(2026, time.July, 1),
		PreviousEnd:    comparisonDate(2026, time.July, 11),
		Days:           11,
		ComparisonKind: comparisonSameDaysPreviousMonth,
	}
	dashboard := DashboardResponse{
		Summary:       DashboardSummary{TotalSpent: 40486},
		TopCategories: []DashboardCategory{{Category: "Bills", Amount: 19821, Percentage: 49}},
	}

	cards := buildInsightCards(dashboard, dateRange, comparisonBase{Amount: 300, Count: 2}, nil, nil)

	if insightByKind(cards, "period_comparison") != nil {
		t.Fatalf("a ₹300 two-entry baseline must not produce a percentage: %#v", cards)
	}
	if insightByKind(cards, "category_increase") != nil {
		t.Fatalf("a category with no comparable base must not produce a percentage: %#v", cards)
	}
}

// The strip on Home reads the headline comparison straight off the summary.
// Before W2 it existed only inside a `period_comparison` insight card, which is
// ranked and deduped against the hero — so a caller wanting the plain number
// could lose it for reasons that have nothing to do with the data.
func TestApplySpendComparisonPublishesOnlyAboveTheFloor(t *testing.T) {
	summary := DashboardSummary{TotalSpent: 40486}
	applySpendComparison(&summary, comparisonBase{Amount: 15644, Count: 15})

	if !summary.SpendChangeComparable {
		t.Fatal("a 15-transaction, ₹15,644 base is well above the floor and must publish")
	}
	if got := math.Round(summary.SpendChange); got != 159 {
		t.Fatalf("spend change = %v, want 159", got)
	}
	if summary.PreviousTotalSpent != 15644 {
		t.Fatalf("previous total = %v, want 15644", summary.PreviousTotalSpent)
	}
}

func TestApplySpendComparisonWithholdsChangeOnAThinBase(t *testing.T) {
	// One ₹40,000 rent payment: a big number and a terrible baseline.
	summary := DashboardSummary{TotalSpent: 40486}
	applySpendComparison(&summary, comparisonBase{Amount: 40000, Count: 1})

	if summary.SpendChangeComparable {
		t.Fatal("a single-transaction base must not publish a percentage")
	}
	if summary.SpendChange != 0 {
		t.Fatalf("spend change = %v, want 0 when not comparable", summary.SpendChange)
	}
	// The base itself is still reported — it is the change that is withheld.
	if summary.PreviousTotalSpent != 40000 {
		t.Fatalf("previous total = %v, want 40000", summary.PreviousTotalSpent)
	}
}

// A summary reused across calls must not keep a stale comparison.
func TestApplySpendComparisonClearsAPreviousResult(t *testing.T) {
	summary := DashboardSummary{TotalSpent: 100, SpendChange: 159, SpendChangeComparable: true}
	applySpendComparison(&summary, comparisonBase{Amount: 20, Count: 1})

	if summary.SpendChangeComparable || summary.SpendChange != 0 {
		t.Fatalf("stale comparison survived: %+v", summary)
	}
}
