package http

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// How the previous window was chosen. Sent to the app so a comparison can
// always explain itself.
const (
	comparisonSameDaysPreviousMonth = "same_days_previous_month"
	comparisonPrecedingPeriod       = "preceding_period"
)

// A comparison needs a base worth dividing by. Below these, a percentage says
// more about how little was logged last period than about spending: ten days
// of August against a quiet stretch of July produced "2127% higher", which
// teaches users the analysis is decorative.
const (
	comparisonMinBaseAmount  = 500.0
	comparisonMinBaseCount   = 5
	comparisonMultiplierFrom = 300.0
)

// comparisonBase is the previous-period figure a change is measured against —
// the total, and how many transactions produced it.
type comparisonBase struct {
	Amount float64
	Count  int
}

// publishable reports whether this base can carry a percentage the user should
// be shown. Both tests matter: one ₹40,000 rent payment is a big number and a
// terrible baseline, and five ₹20 chai runs are a real habit but too small to
// divide by.
func (b comparisonBase) publishable() bool {
	return b.Amount >= comparisonMinBaseAmount && b.Count >= comparisonMinBaseCount
}

func daysInMonth(moment time.Time) int {
	return time.Date(moment.Year(), moment.Month()+1, 0, 0, 0, 0, 0, moment.Location()).Day()
}

// previousComparisonWindow picks what a range should be measured against.
//
// A range that starts on the 1st and stays inside one calendar month is a
// month-to-date view, and the honest comparison is the same days of the
// previous month. The old rule — the equal-length window immediately before
// the range — put Aug 1–11 against Jul 21–31, the tail of a month, which is
// both arbitrary and usually near-empty. Any other range is genuinely
// arbitrary, so the preceding equal-length window remains the right answer.
func previousComparisonWindow(start, end time.Time) (time.Time, time.Time, string) {
	if start.Day() == 1 && start.Year() == end.Year() && start.Month() == end.Month() {
		previousStart := start.AddDate(0, -1, 0)
		span := end.Day()
		// A short month cannot supply the same days: 1–31 March compares
		// against all of February, not a date February does not have.
		if limit := daysInMonth(previousStart); span > limit {
			span = limit
		}
		return previousStart, previousStart.AddDate(0, 0, span-1), comparisonSameDaysPreviousMonth
	}

	days := int(end.Sub(start).Hours()/24) + 1
	previousEnd := start.AddDate(0, 0, -1)
	return previousEnd.AddDate(0, 0, -(days - 1)), previousEnd, comparisonPrecedingPeriod
}

// formatComparisonWindow renders one window the way a person would say it:
// "Aug 1–11", or "Jul 28 – Aug 3" when it straddles a month boundary.
func formatComparisonWindow(start, end time.Time) string {
	if start.Year() == end.Year() && start.Month() == end.Month() {
		if start.Day() == end.Day() {
			return fmt.Sprintf("%s %d", start.Format("Jan"), start.Day())
		}
		return fmt.Sprintf("%s %d–%d", start.Format("Jan"), start.Day(), end.Day())
	}
	return fmt.Sprintf("%s %d – %s %d",
		start.Format("Jan"), start.Day(), end.Format("Jan"), end.Day())
}

// comparisonLabel is the whole comparison in one phrase: "Aug 1–11 vs Jul 1–11".
func comparisonLabel(dateRange dashboardRange) string {
	return fmt.Sprintf("%s vs %s",
		formatComparisonWindow(dateRange.Start, dateRange.End),
		formatComparisonWindow(dateRange.PreviousStart, dateRange.PreviousEnd))
}

// comparisonExplanation states the rule that produced the window, so the card
// can be checked rather than believed.
func comparisonExplanation(dateRange dashboardRange) string {
	previous := formatComparisonWindow(dateRange.PreviousStart, dateRange.PreviousEnd)
	if dateRange.ComparisonKind == comparisonSameDaysPreviousMonth {
		return fmt.Sprintf(
			"Compares %s with the same days of the previous month, %s.",
			formatComparisonWindow(dateRange.Start, dateRange.End), previous,
		)
	}
	return fmt.Sprintf(
		"Compares %s with the %d days immediately before it, %s.",
		formatComparisonWindow(dateRange.Start, dateRange.End), dateRange.Days, previous,
	)
}

// formatChangeMagnitude renders how big a change is.
//
// Past roughly 300%, a percentage stops being read as a quantity — "1218%
// higher" reads as a broken calculation, while "13.2× higher" is a number
// someone can picture. Only increases can get there; a decrease bottoms out at
// -100%.
func formatChangeMagnitude(change float64) string {
	if change > comparisonMultiplierFrom {
		multiple := 1 + change/100
		if multiple >= 10 {
			return fmt.Sprintf("%.0f×", multiple)
		}
		return strings.TrimSuffix(fmt.Sprintf("%.1f", multiple), ".0") + "×"
	}
	return fmt.Sprintf("%.0f%%", math.Abs(change))
}

func changeDirection(change float64) string {
	if change < 0 {
		return "lower"
	}
	return "higher"
}

// applySpendComparison stamps the headline spend comparison onto a summary.
//
// Both dashboard builders call this rather than each doing the arithmetic, for
// the same reason the window rule lives in this file: the DB-backed and
// in-memory builders have to answer identically, and the floor has to be the
// one floor. A base below it publishes no change at all — never 0%, which a
// reader would take to mean "spending is flat".
func applySpendComparison(summary *DashboardSummary, base comparisonBase) {
	summary.PreviousTotalSpent = base.Amount
	summary.SpendChange = 0
	summary.SpendChangeComparable = false
	if base.publishable() {
		summary.SpendChange = percentageChange(summary.TotalSpent, base.Amount)
		summary.SpendChangeComparable = true
	}
}
