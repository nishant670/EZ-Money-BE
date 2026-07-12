package http

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"

	"github.com/gin-gonic/gin"
)

type DashboardPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type DashboardSummary struct {
	TotalSpent       float64 `json:"total_spent"`
	TotalIncome      float64 `json:"total_income"`
	DailyAverage     float64 `json:"daily_average"`
	TransactionCount int     `json:"transaction_count"`
}

type DashboardCategory struct {
	Category   string  `json:"category"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
	Change     float64 `json:"change"`
}

type DashboardMerchant struct {
	Merchant         string  `json:"merchant"`
	Amount           float64 `json:"amount"`
	TransactionCount int     `json:"transaction_count"`
}

type DashboardAccount struct {
	AccountID   *uint   `json:"account_id"`
	AccountName string  `json:"account_name"`
	Amount      float64 `json:"amount"`
	Percentage  float64 `json:"percentage"`
}

type DashboardRecurringCandidate struct {
	Label            string  `json:"label"`
	Merchant         string  `json:"merchant"`
	Category         string  `json:"category"`
	AverageAmount    float64 `json:"average_amount"`
	IntervalGuess    string  `json:"interval_guess"`
	Confidence       float64 `json:"confidence"`
	Occurrences      int     `json:"occurrences"`
	LastSeenDate     string  `json:"last_seen_date"`
	NextExpectedDate string  `json:"next_expected_date"`
	ReviewDue        bool    `json:"review_due"`
}

type InsightCard struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

type DashboardResponse struct {
	Period              DashboardPeriod               `json:"period"`
	Summary             DashboardSummary              `json:"summary"`
	TopCategories       []DashboardCategory           `json:"top_categories"`
	TopMerchants        []DashboardMerchant           `json:"top_merchants"`
	AccountSpending     []DashboardAccount            `json:"account_spending"`
	RecentTransactions  []models.Entry                `json:"recent_transactions"`
	Insights            []InsightCard                 `json:"insights"`
	RecurringCandidates []DashboardRecurringCandidate `json:"recurring_candidates"`
}

type dashboardRange struct {
	Start         time.Time
	End           time.Time
	PreviousStart time.Time
	PreviousEnd   time.Time
	Days          int
}

func parseDashboardRange(startValue, endValue string, now time.Time) (dashboardRange, map[string]string) {
	fields := map[string]string{}
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if startValue != "" {
		parsed, err := time.ParseInLocation("2006-01-02", startValue, now.Location())
		if err != nil {
			fields["start_date"] = "must use YYYY-MM-DD"
		} else {
			start = parsed
		}
	}
	if endValue != "" {
		parsed, err := time.ParseInLocation("2006-01-02", endValue, now.Location())
		if err != nil {
			fields["end_date"] = "must use YYYY-MM-DD"
		} else {
			end = parsed
		}
	}
	if len(fields) == 0 && start.After(end) {
		fields["end_date"] = "must be on or after start_date"
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if len(fields) == 0 && days > 366 {
		fields["start_date"] = "range must not exceed 366 days"
	}
	previousEnd := start.AddDate(0, 0, -1)
	previousStart := previousEnd.AddDate(0, 0, -(days - 1))
	return dashboardRange{
		Start: start, End: end, PreviousStart: previousStart,
		PreviousEnd: previousEnd, Days: days,
	}, fields
}

func (s *Server) getDashboard(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	location := loadLocationOrIndia(c.Query("tz"), s.cfg.TZDefault)
	dateRange, fields := parseDashboardRange(c.Query("start_date"), c.Query("end_date"), time.Now().In(location))
	if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_range", "fields": fields})
		return
	}

	var entries []models.Entry
	if err := database.DB.Preload("Account").
		Where("user_id = ? AND date >= ? AND date <= ?", userID,
			dateRange.PreviousStart.Format("2006-01-02"), dateRange.End.Format("2006-01-02")).
		Order("date desc, created_at desc").
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_load_dashboard"})
		return
	}

	c.JSON(http.StatusOK, buildDashboard(entries, dateRange))
}

func (s *Server) getInsights(c *gin.Context) {
	s.getDashboard(c)
}

func buildDashboard(entries []models.Entry, dateRange dashboardRange) DashboardResponse {
	start := dateRange.Start.Format("2006-01-02")
	end := dateRange.End.Format("2006-01-02")
	previousStart := dateRange.PreviousStart.Format("2006-01-02")
	previousEnd := dateRange.PreviousEnd.Format("2006-01-02")

	current := make([]models.Entry, 0)
	previous := make([]models.Entry, 0)
	for _, entry := range entries {
		switch {
		case entry.Date >= start && entry.Date <= end:
			current = append(current, entry)
		case entry.Date >= previousStart && entry.Date <= previousEnd:
			previous = append(previous, entry)
		}
	}

	response := DashboardResponse{
		Period:        DashboardPeriod{Start: start, End: end},
		TopCategories: []DashboardCategory{}, TopMerchants: []DashboardMerchant{},
		AccountSpending: []DashboardAccount{}, RecentTransactions: []models.Entry{},
		Insights: []InsightCard{}, RecurringCandidates: []DashboardRecurringCandidate{},
	}

	categoryCurrent := map[string]float64{}
	categoryPrevious := map[string]float64{}
	merchantCurrent := map[string]*DashboardMerchant{}
	accountCurrent := map[string]*DashboardAccount{}
	expenseAmounts := []float64{}

	for _, entry := range previous {
		if strings.EqualFold(entry.Type, "expense") {
			categoryPrevious[normalizedLabel(entry.Category, "Uncategorized")] += entry.Amount.Float64()
		}
	}
	for _, entry := range current {
		response.Summary.TransactionCount++
		amount := entry.Amount.Float64()
		if strings.EqualFold(entry.Type, "income") {
			response.Summary.TotalIncome += amount
			continue
		}
		if !strings.EqualFold(entry.Type, "expense") {
			continue
		}
		response.Summary.TotalSpent += amount
		expenseAmounts = append(expenseAmounts, amount)

		category := normalizedLabel(entry.Category, "Uncategorized")
		categoryCurrent[category] += amount

		merchant := strings.TrimSpace(entry.Merchant)
		if merchant != "" {
			if merchantCurrent[merchant] == nil {
				merchantCurrent[merchant] = &DashboardMerchant{Merchant: merchant}
			}
			merchantCurrent[merchant].Amount += amount
			merchantCurrent[merchant].TransactionCount++
		}

		accountKey, accountName := "unassigned", "Unassigned"
		var accountID *uint
		if entry.AccountID != nil {
			accountID = entry.AccountID
			accountKey = fmt.Sprintf("%d", *entry.AccountID)
			accountName = entry.Mode
			if entry.Account != nil && strings.TrimSpace(entry.Account.Name) != "" {
				accountName = entry.Account.Name
			}
		}
		if accountCurrent[accountKey] == nil {
			accountCurrent[accountKey] = &DashboardAccount{AccountID: accountID, AccountName: accountName}
		}
		accountCurrent[accountKey].Amount += amount
	}

	if dateRange.Days > 0 {
		response.Summary.DailyAverage = response.Summary.TotalSpent / float64(dateRange.Days)
	}
	for category, amount := range categoryCurrent {
		change := percentageChange(amount, categoryPrevious[category])
		response.TopCategories = append(response.TopCategories, DashboardCategory{
			Category: category, Amount: amount,
			Percentage: safePercentage(amount, response.Summary.TotalSpent), Change: change,
		})
	}
	sort.Slice(response.TopCategories, func(i, j int) bool {
		return response.TopCategories[i].Amount > response.TopCategories[j].Amount
	})
	if len(response.TopCategories) > 5 {
		response.TopCategories = response.TopCategories[:5]
	}

	for _, merchant := range merchantCurrent {
		response.TopMerchants = append(response.TopMerchants, *merchant)
	}
	sort.Slice(response.TopMerchants, func(i, j int) bool {
		return response.TopMerchants[i].Amount > response.TopMerchants[j].Amount
	})
	if len(response.TopMerchants) > 5 {
		response.TopMerchants = response.TopMerchants[:5]
	}

	for _, account := range accountCurrent {
		account.Percentage = safePercentage(account.Amount, response.Summary.TotalSpent)
		response.AccountSpending = append(response.AccountSpending, *account)
	}
	sort.Slice(response.AccountSpending, func(i, j int) bool {
		return response.AccountSpending[i].Amount > response.AccountSpending[j].Amount
	})

	sort.Slice(current, func(i, j int) bool {
		if current[i].Date == current[j].Date {
			return current[i].CreatedAt.After(current[j].CreatedAt)
		}
		return current[i].Date > current[j].Date
	})
	if len(current) > 5 {
		current = current[:5]
	}
	response.RecentTransactions = current
	response.RecurringCandidates = detectRecurringCandidates(entries, dateRange)
	response.Insights = buildInsightCards(
		response, previousExpenseTotal(previous), categoryPrevious, expenseAmounts,
	)
	return response
}

func buildInsightCards(
	dashboard DashboardResponse,
	previousSpent float64,
	categoryPrevious map[string]float64,
	expenseAmounts []float64,
) []InsightCard {
	cards := []InsightCard{}
	if dashboard.Summary.TotalSpent > 0 || previousSpent > 0 {
		change := percentageChange(dashboard.Summary.TotalSpent, previousSpent)
		direction := "higher"
		if change < 0 {
			direction = "lower"
		}
		cards = append(cards, InsightCard{
			Kind: "period_comparison", Severity: "info", Title: "Period comparison",
			Body: fmt.Sprintf("Spending is %.0f%% %s than the previous period.", math.Abs(change), direction),
		})
	}

	for _, category := range dashboard.TopCategories {
		if previous := categoryPrevious[category.Category]; previous > 0 && category.Change >= 20 {
			cards = append(cards, InsightCard{
				Kind: "category_increase", Severity: "warning", Title: category.Category + " increased",
				Body: fmt.Sprintf("%s spending is %.0f%% higher than the previous period.", category.Category, category.Change),
			})
			break
		}
	}
	if len(dashboard.TopMerchants) > 0 {
		top := dashboard.TopMerchants[0]
		cards = append(cards, InsightCard{
			Kind: "top_merchant", Severity: "info", Title: "Top merchant",
			Body: fmt.Sprintf("%s accounted for ₹%.2f across %d transaction(s).", top.Merchant, top.Amount, top.TransactionCount),
		})
	}
	if len(dashboard.AccountSpending) > 0 {
		top := dashboard.AccountSpending[0]
		cards = append(cards, InsightCard{
			Kind: "account_usage", Severity: "info", Title: "Most-used account",
			Body: fmt.Sprintf("%s handled %.0f%% of spending.", top.AccountName, top.Percentage),
		})
	}
	if unusual := unusualExpense(expenseAmounts); unusual > 0 {
		cards = append(cards, InsightCard{
			Kind: "unusual_spending", Severity: "warning", Title: "Unusual spending",
			Body: fmt.Sprintf("A ₹%.2f expense was substantially above this period's average.", unusual),
		})
	}
	if len(dashboard.RecurringCandidates) > 0 {
		candidate := dashboard.RecurringCandidates[0]
		cards = append(cards, InsightCard{
			Kind: "recurring_candidate", Severity: "info", Title: "Recurring spend to review",
			Body: fmt.Sprintf("Review %d likely recurring expense(s), including %s around ₹%.2f.", len(dashboard.RecurringCandidates), candidate.Label, candidate.AverageAmount),
		})
	}
	return cards
}

type recurringGroup struct {
	label    string
	merchant string
	category string
	entries  []models.Entry
}

type recurringInterval struct {
	name string
	days int
}

func detectRecurringCandidates(entries []models.Entry, dateRange dashboardRange) []DashboardRecurringCandidate {
	groups := map[string]*recurringGroup{}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Type, "expense") {
			continue
		}
		if _, err := time.ParseInLocation("2006-01-02", entry.Date, dateRange.Start.Location()); err != nil {
			continue
		}
		label := recurringLabel(entry)
		if label == "" {
			continue
		}
		category := normalizedLabel(entry.Category, "Uncategorized")
		key := strings.ToLower(label) + "|" + strings.ToLower(category)
		if groups[key] == nil {
			groups[key] = &recurringGroup{
				label: label, merchant: strings.TrimSpace(entry.Merchant), category: category,
			}
		}
		groups[key].entries = append(groups[key].entries, entry)
	}

	candidates := []DashboardRecurringCandidate{}
	for _, group := range groups {
		if candidate, ok := recurringCandidateFromGroup(group, dateRange); ok {
			candidates = append(candidates, candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ReviewDue != candidates[j].ReviewDue {
			return candidates[i].ReviewDue
		}
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return candidates[i].NextExpectedDate < candidates[j].NextExpectedDate
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates
}

func recurringCandidateFromGroup(group *recurringGroup, dateRange dashboardRange) (DashboardRecurringCandidate, bool) {
	entries := append([]models.Entry(nil), group.entries...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})
	if len(entries) < 2 {
		return DashboardRecurringCandidate{}, false
	}

	amounts := make([]float64, 0, len(entries))
	dates := make([]time.Time, 0, len(entries))
	for _, entry := range entries {
		date, err := time.ParseInLocation("2006-01-02", entry.Date, dateRange.Start.Location())
		if err != nil {
			return DashboardRecurringCandidate{}, false
		}
		dates = append(dates, date)
		amounts = append(amounts, entry.Amount.Float64())
	}
	if !recurringAmountsAreStable(amounts) {
		return DashboardRecurringCandidate{}, false
	}

	interval, ok := guessRecurringInterval(dates)
	if !ok {
		return DashboardRecurringCandidate{}, false
	}

	average := averageFloat(amounts)
	lastSeen := dates[len(dates)-1]
	nextExpected := nextRecurringDate(lastSeen, interval)
	confidence := recurringConfidence(len(entries), amounts)
	reviewDue := !nextExpected.Before(dateRange.Start) && !nextExpected.After(dateRange.End.AddDate(0, 0, 7))

	return DashboardRecurringCandidate{
		Label:            group.label,
		Merchant:         group.merchant,
		Category:         group.category,
		AverageAmount:    average,
		IntervalGuess:    interval.name,
		Confidence:       confidence,
		Occurrences:      len(entries),
		LastSeenDate:     lastSeen.Format("2006-01-02"),
		NextExpectedDate: nextExpected.Format("2006-01-02"),
		ReviewDue:        reviewDue,
	}, true
}

func recurringLabel(entry models.Entry) string {
	for _, value := range []string{entry.Merchant, entry.Title} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func guessRecurringInterval(dates []time.Time) (recurringInterval, bool) {
	if len(dates) < 2 {
		return recurringInterval{}, false
	}
	intervals := make([]int, 0, len(dates)-1)
	for i := 1; i < len(dates); i++ {
		days := int(dates[i].Sub(dates[i-1]).Hours() / 24)
		if days <= 0 {
			return recurringInterval{}, false
		}
		intervals = append(intervals, days)
	}
	average := averageInt(intervals)
	switch {
	case average >= 6 && average <= 8 && intervalSpreadWithin(intervals, 2):
		return recurringInterval{name: "weekly", days: 7}, true
	case average >= 27 && average <= 35 && intervalSpreadWithin(intervals, 5):
		return recurringInterval{name: "monthly", days: 30}, true
	default:
		return recurringInterval{}, false
	}
}

func nextRecurringDate(lastSeen time.Time, interval recurringInterval) time.Time {
	if interval.name == "monthly" {
		return lastSeen.AddDate(0, 1, 0)
	}
	return lastSeen.AddDate(0, 0, interval.days)
}

func recurringAmountsAreStable(amounts []float64) bool {
	if len(amounts) < 2 {
		return false
	}
	minimum, maximum := amounts[0], amounts[0]
	for _, amount := range amounts[1:] {
		if amount < minimum {
			minimum = amount
		}
		if amount > maximum {
			maximum = amount
		}
	}
	average := averageFloat(amounts)
	if average <= 0 {
		return false
	}
	return (maximum - minimum) <= math.Max(20, average*0.15)
}

func recurringConfidence(occurrences int, amounts []float64) float64 {
	confidence := 0.65
	if occurrences >= 3 {
		confidence = 0.80
	}
	if recurringAmountSpread(amounts) <= 0.05 {
		confidence += 0.10
	}
	if occurrences >= 4 {
		confidence += 0.05
	}
	return math.Min(confidence, 0.95)
}

func recurringAmountSpread(amounts []float64) float64 {
	average := averageFloat(amounts)
	if average <= 0 {
		return 1
	}
	minimum, maximum := amounts[0], amounts[0]
	for _, amount := range amounts[1:] {
		if amount < minimum {
			minimum = amount
		}
		if amount > maximum {
			maximum = amount
		}
	}
	return (maximum - minimum) / average
}

func averageFloat(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func averageInt(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return int(math.Round(float64(total) / float64(len(values))))
}

func intervalSpreadWithin(values []int, tolerance int) bool {
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	return maximum-minimum <= tolerance
}

func normalizedLabel(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func previousExpenseTotal(entries []models.Entry) float64 {
	total := 0.0
	for _, entry := range entries {
		if strings.EqualFold(entry.Type, "expense") {
			total += entry.Amount.Float64()
		}
	}
	return total
}

func safePercentage(value, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return value / total * 100
}

func percentageChange(current, previous float64) float64 {
	if previous <= 0 {
		if current > 0 {
			return 100
		}
		return 0
	}
	return (current - previous) / previous * 100
}

func unusualExpense(amounts []float64) float64 {
	if len(amounts) < 3 {
		return 0
	}
	total := 0.0
	maximum := 0.0
	for _, amount := range amounts {
		total += amount
		if amount > maximum {
			maximum = amount
		}
	}
	average := total / float64(len(amounts))
	if maximum >= average*2 {
		return maximum
	}
	return 0
}
