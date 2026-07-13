package http

import (
	"testing"
	"time"

	"finance-parser-go/internal/models"
)

func dashboardEntry(
	date, entryType, amount, category, merchant string,
	accountID uint, accountName string,
) models.Entry {
	parsedAmount, err := models.ParseMoney(amount)
	if err != nil {
		panic(err)
	}
	id := accountID
	return models.Entry{
		Date: date, Type: entryType, Amount: parsedAmount, Category: category,
		Merchant: merchant, AccountID: &id,
		Account: &models.Account{ID: id, Name: accountName},
	}
}

func TestBuildDashboardProducesFiveDeterministicTemplates(t *testing.T) {
	location := time.FixedZone("IST", 5*60*60+30*60)
	dateRange := dashboardRange{
		Start:         time.Date(2026, 7, 1, 0, 0, 0, 0, location),
		End:           time.Date(2026, 7, 3, 0, 0, 0, 0, location),
		PreviousStart: time.Date(2026, 6, 28, 0, 0, 0, 0, location),
		PreviousEnd:   time.Date(2026, 6, 30, 0, 0, 0, 0, location),
		Days:          3,
	}
	entries := []models.Entry{
		dashboardEntry("2026-06-29", "expense", "100", "Food", "Cafe", 1, "Cash"),
		dashboardEntry("2026-06-30", "expense", "100", "Travel", "Metro", 2, "UPI"),
		dashboardEntry("2026-07-01", "expense", "100", "Food", "Cafe", 1, "Cash"),
		dashboardEntry("2026-07-02", "expense", "100", "Food", "Cafe", 1, "Cash"),
		dashboardEntry("2026-07-03", "expense", "1000", "Food", "Cafe", 1, "Cash"),
	}

	dashboard := buildDashboard(entries, dateRange)
	kinds := map[string]bool{}
	for _, card := range dashboard.Insights {
		kinds[card.Kind] = true
	}
	for _, expected := range []string{
		"period_comparison", "category_increase", "top_merchant",
		"account_usage", "unusual_spending",
	} {
		if !kinds[expected] {
			t.Fatalf("missing %s insight: %#v", expected, dashboard.Insights)
		}
	}
	if dashboard.Summary.TotalSpent != 1200 || dashboard.Summary.DailyAverage != 400 {
		t.Fatalf("unexpected summary: %#v", dashboard.Summary)
	}
}

func TestBuildDashboardDetectsRecurringCandidatesForWeeklyReview(t *testing.T) {
	location := time.FixedZone("IST", 5*60*60+30*60)
	dateRange := dashboardRange{
		Start:         time.Date(2026, 7, 8, 0, 0, 0, 0, location),
		End:           time.Date(2026, 7, 14, 0, 0, 0, 0, location),
		PreviousStart: time.Date(2026, 7, 1, 0, 0, 0, 0, location),
		PreviousEnd:   time.Date(2026, 7, 7, 0, 0, 0, 0, location),
		Days:          7,
	}
	entries := []models.Entry{
		dashboardEntry("2026-07-01", "expense", "499", "Entertainment", "Streamly", 1, "UPI"),
		dashboardEntry("2026-07-08", "expense", "499", "Entertainment", "Streamly", 1, "UPI"),
		dashboardEntry("2026-07-10", "expense", "120", "Food", "Cafe", 1, "UPI"),
		dashboardEntry("2026-07-12", "expense", "220", "Food", "Cafe", 1, "UPI"),
	}

	dashboard := buildDashboard(entries, dateRange)
	if len(dashboard.RecurringCandidates) != 1 {
		t.Fatalf("expected one recurring candidate, got %#v", dashboard.RecurringCandidates)
	}
	candidate := dashboard.RecurringCandidates[0]
	if candidate.Label != "Streamly" || candidate.IntervalGuess != "weekly" || !candidate.ReviewDue {
		t.Fatalf("unexpected recurring candidate: %#v", candidate)
	}
	if candidate.NextExpectedDate != "2026-07-15" || candidate.AverageAmount != 499 {
		t.Fatalf("unexpected recurring timing/amount: %#v", candidate)
	}

	kinds := map[string]bool{}
	for _, card := range dashboard.Insights {
		kinds[card.Kind] = true
	}
	if !kinds["recurring_candidate"] {
		t.Fatalf("missing recurring insight: %#v", dashboard.Insights)
	}
}

func TestDetectRecurringCandidatesRejectsUnstableRepeats(t *testing.T) {
	location := time.FixedZone("IST", 5*60*60+30*60)
	dateRange := dashboardRange{
		Start:         time.Date(2026, 7, 1, 0, 0, 0, 0, location),
		End:           time.Date(2026, 7, 31, 0, 0, 0, 0, location),
		PreviousStart: time.Date(2026, 6, 1, 0, 0, 0, 0, location),
		PreviousEnd:   time.Date(2026, 6, 30, 0, 0, 0, 0, location),
		Days:          31,
	}
	entries := []models.Entry{
		dashboardEntry("2026-06-01", "expense", "100", "Food", "Cafe", 1, "Cash"),
		dashboardEntry("2026-06-12", "expense", "250", "Food", "Cafe", 1, "Cash"),
		dashboardEntry("2026-07-03", "expense", "600", "Food", "Cafe", 1, "Cash"),
	}

	candidates := detectRecurringCandidates(entries, dateRange)
	if len(candidates) != 0 {
		t.Fatalf("expected no unstable recurring candidates, got %#v", candidates)
	}
}

func TestParseDashboardRangeRejectsInvalidAndOversizedRanges(t *testing.T) {
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	_, fields := parseDashboardRange("2026-07-10", "2026-07-01", now)
	if fields["end_date"] == "" {
		t.Fatalf("expected inverted range error, got %v", fields)
	}
	_, fields = parseDashboardRange("2025-01-01", "2026-07-09", now)
	if fields["start_date"] == "" {
		t.Fatalf("expected maximum range error, got %v", fields)
	}
}
