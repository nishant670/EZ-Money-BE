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

func insightByKind(cards []InsightCard, kind string) *InsightCard {
	for index := range cards {
		if cards[index].Kind == kind {
			return &cards[index]
		}
	}
	return nil
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
	categoryInsight := insightByKind(dashboard.Insights, "category_increase")
	if categoryInsight == nil || categoryInsight.Category != "Food" || categoryInsight.Amount == nil || *categoryInsight.Amount != 1200 {
		t.Fatalf("missing structured category insight data: %#v", categoryInsight)
	}
	merchantInsight := insightByKind(dashboard.Insights, "top_merchant")
	if merchantInsight == nil || merchantInsight.Merchant != "Cafe" || merchantInsight.TransactionCount == nil || *merchantInsight.TransactionCount != 3 {
		t.Fatalf("missing structured merchant insight data: %#v", merchantInsight)
	}
	accountInsight := insightByKind(dashboard.Insights, "account_usage")
	if accountInsight == nil || accountInsight.AccountName != "Cash" || accountInsight.Percentage == nil || *accountInsight.Percentage != 100 {
		t.Fatalf("missing structured account insight data: %#v", accountInsight)
	}
	if dashboard.Summary.TotalSpent != 1200 || dashboard.Summary.DailyAverage != 400 {
		t.Fatalf("unexpected summary: %#v", dashboard.Summary)
	}
	if len(dashboard.DailySpending) != 3 {
		t.Fatalf("expected three daily spending rows, got %#v", dashboard.DailySpending)
	}
	expectedDaily := map[string]float64{
		"2026-07-01": 100,
		"2026-07-02": 100,
		"2026-07-03": 1000,
	}
	for _, daily := range dashboard.DailySpending {
		if expectedDaily[daily.Date] != daily.Amount {
			t.Fatalf("unexpected daily spending row: %#v", daily)
		}
	}
}

func TestBuildDashboardReturnsFullPeriodReviewItems(t *testing.T) {
	location := time.FixedZone("IST", 5*60*60+30*60)
	dateRange := dashboardRange{
		Start:         time.Date(2026, 7, 1, 0, 0, 0, 0, location),
		End:           time.Date(2026, 7, 10, 0, 0, 0, 0, location),
		PreviousStart: time.Date(2026, 6, 21, 0, 0, 0, 0, location),
		PreviousEnd:   time.Date(2026, 6, 30, 0, 0, 0, 0, location),
		Days:          10,
	}
	entries := []models.Entry{
		dashboardEntry("2026-06-23", "expense", "90", "Food", "Older Cleanup", 1, "Cash"),
		dashboardEntry("2026-06-24", "expense", "110", "Food", "Older Cleanup", 1, "Cash"),
		dashboardEntry("2026-06-25", "expense", "200", "Shopping", "Older Cleanup", 1, "Cash"),
		dashboardEntry("2026-07-10", "expense", "100", "Food", "Cafe 1", 1, "Cash"),
		dashboardEntry("2026-07-09", "expense", "100", "Food", "Cafe 2", 1, "Cash"),
		dashboardEntry("2026-07-08", "expense", "100", "Food", "Cafe 3", 1, "Cash"),
		dashboardEntry("2026-07-07", "expense", "100", "Food", "Cafe 4", 1, "Cash"),
		dashboardEntry("2026-07-06", "expense", "100", "Food", "Cafe 5", 1, "Cash"),
		dashboardEntry("2026-07-05", "expense", "100", "Uncategorized", "Older Cleanup", 1, "Cash"),
	}
	missingAccount := dashboardEntry("2026-07-04", "expense", "120", "Travel", "Metro", 1, "Cash")
	missingAccount.AccountID = nil
	missingAccount.Account = nil
	entries = append(entries, missingAccount)

	dashboard := buildDashboard(entries, dateRange)
	if len(dashboard.RecentTransactions) != 5 {
		t.Fatalf("expected five recent transactions, got %#v", dashboard.RecentTransactions)
	}
	if len(dashboard.ReviewItems) != 2 {
		t.Fatalf("expected two review items from the full period, got %#v", dashboard.ReviewItems)
	}
	if dashboard.ReviewItems[0].Merchant != "Older Cleanup" || dashboard.ReviewItems[1].Merchant != "Metro" {
		t.Fatalf("unexpected review item ordering: %#v", dashboard.ReviewItems)
	}
	if len(dashboard.ReviewItems[0].CategorySuggestions) != 2 ||
		dashboard.ReviewItems[0].CategorySuggestions[0] != "Food" ||
		dashboard.ReviewItems[0].CategorySuggestions[1] != "Shopping" {
		t.Fatalf("expected category suggestions from matching history, got %#v", dashboard.ReviewItems[0].CategorySuggestions)
	}
	for _, entry := range dashboard.RecentTransactions {
		if entry.Merchant == "Older Cleanup" || entry.Merchant == "Metro" {
			t.Fatalf("review fixture should be outside recent preview: %#v", dashboard.RecentTransactions)
		}
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
	recurringInsight := insightByKind(dashboard.Insights, "recurring_candidate")
	if recurringInsight == nil || recurringInsight.Merchant != "Streamly" || recurringInsight.NextExpectedDate != "2026-07-15" || recurringInsight.Confidence == nil {
		t.Fatalf("missing structured recurring insight data: %#v", recurringInsight)
	}
}

func TestApplyBudgetStatusesAddsWatchAndExceededInsights(t *testing.T) {
	location := time.FixedZone("IST", 5*60*60+30*60)
	dateRange := dashboardRange{
		Start:         time.Date(2026, 7, 1, 0, 0, 0, 0, location),
		End:           time.Date(2026, 7, 20, 0, 0, 0, 0, location),
		PreviousStart: time.Date(2026, 6, 11, 0, 0, 0, 0, location),
		PreviousEnd:   time.Date(2026, 6, 30, 0, 0, 0, 0, location),
		Days:          20,
	}
	entries := []models.Entry{
		dashboardEntry("2026-07-05", "expense", "850", "Food", "Cafe", 1, "Cash"),
		dashboardEntry("2026-07-10", "expense", "1200", "Travel", "Metro", 1, "Cash"),
		dashboardEntry("2026-07-11", "expense", "100", "Shopping", "Store", 1, "Cash"),
	}
	dashboard := buildDashboard(entries, dateRange)
	applyBudgetStatuses(entries, []models.Budget{
		{ID: 1, Name: "Food", Period: budgetPeriodMonthly, Category: "Food", LimitAmount: testMoney("1000"), AlertThresholdPercent: 80, Active: true},
		{ID: 2, Name: "Travel", Period: budgetPeriodMonthly, Category: "Travel", LimitAmount: testMoney("1000"), AlertThresholdPercent: 80, Active: true},
		{ID: 3, Name: "Shopping", Period: budgetPeriodMonthly, Category: "Shopping", LimitAmount: testMoney("1000"), AlertThresholdPercent: 80, Active: true},
	}, dateRange, &dashboard)

	if len(dashboard.BudgetStatuses) != 3 {
		t.Fatalf("expected three budget statuses, got %#v", dashboard.BudgetStatuses)
	}
	kinds := map[string]int{}
	for _, insight := range dashboard.Insights {
		kinds[insight.Kind]++
	}
	if kinds["budget_watch"] != 1 || kinds["budget_exceeded"] != 1 {
		t.Fatalf("expected one watch and one exceeded budget insight, got %#v", dashboard.Insights)
	}
	if dashboard.BudgetStatuses[0].Status != "exceeded" || dashboard.BudgetStatuses[1].Status != "watch" {
		t.Fatalf("expected exceeded/watch statuses first, got %#v", dashboard.BudgetStatuses)
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
