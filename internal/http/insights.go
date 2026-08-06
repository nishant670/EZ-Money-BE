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

type DashboardBudgetStatus struct {
	BudgetID              uint    `json:"budget_id"`
	Name                  string  `json:"name"`
	Category              string  `json:"category"`
	LimitAmount           float64 `json:"limit_amount"`
	SpentAmount           float64 `json:"spent_amount"`
	RemainingAmount       float64 `json:"remaining_amount"`
	Percentage            float64 `json:"percentage"`
	AlertThresholdPercent int     `json:"alert_threshold_percent"`
	DaysLeft              int     `json:"days_left"`
	Status                string  `json:"status"`
}

type DashboardDailySpend struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

type DashboardReviewItem struct {
	models.Entry
	CategorySuggestions []string `json:"category_suggestions"`
}

type DashboardRecurringCandidate struct {
	CandidateKey     string  `json:"candidate_key"`
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
	Kind             string   `json:"kind"`
	Severity         string   `json:"severity"`
	Title            string   `json:"title"`
	Body             string   `json:"body"`
	Explanation      string   `json:"explanation,omitempty"`
	ActionLabel      string   `json:"action_label,omitempty"`
	Category         string   `json:"category,omitempty"`
	Merchant         string   `json:"merchant,omitempty"`
	BudgetID         *uint    `json:"budget_id,omitempty"`
	AccountID        *uint    `json:"account_id,omitempty"`
	AccountName      string   `json:"account_name,omitempty"`
	Amount           *float64 `json:"amount,omitempty"`
	LimitAmount      *float64 `json:"limit_amount,omitempty"`
	RemainingAmount  *float64 `json:"remaining_amount,omitempty"`
	Status           string   `json:"status,omitempty"`
	Percentage       *float64 `json:"percentage,omitempty"`
	ChangePercentage *float64 `json:"change_percentage,omitempty"`
	TransactionCount *int     `json:"transaction_count,omitempty"`
	NextExpectedDate string   `json:"next_expected_date,omitempty"`
	Confidence       *float64 `json:"confidence,omitempty"`
}

type DashboardResponse struct {
	Period              DashboardPeriod               `json:"period"`
	Summary             DashboardSummary              `json:"summary"`
	TopCategories       []DashboardCategory           `json:"top_categories"`
	TopMerchants        []DashboardMerchant           `json:"top_merchants"`
	AccountSpending     []DashboardAccount            `json:"account_spending"`
	BudgetStatuses      []DashboardBudgetStatus       `json:"budget_statuses"`
	DailySpending       []DashboardDailySpend         `json:"daily_spending"`
	RecentTransactions  []models.Entry                `json:"recent_transactions"`
	ReviewItems         []DashboardReviewItem         `json:"review_items"`
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
	var budgets []models.Budget
	if err := database.DB.
		Where("user_id = ? AND active = ? AND period = ?", userID, true, budgetPeriodMonthly).
		Order("category asc, name asc").
		Find(&budgets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_load_dashboard"})
		return
	}

	dashboard := buildDashboard(entries, dateRange)
	applyBudgetStatuses(entries, budgets, dateRange, &dashboard)
	if err := suppressReviewedRecurringCandidates(userID, &dashboard, dateRange.End); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_load_dashboard"})
		return
	}
	c.JSON(http.StatusOK, dashboard)
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
		AccountSpending: []DashboardAccount{}, BudgetStatuses: []DashboardBudgetStatus{},
		DailySpending:      []DashboardDailySpend{},
		RecentTransactions: []models.Entry{},
		ReviewItems:        []DashboardReviewItem{},
		Insights:           []InsightCard{}, RecurringCandidates: []DashboardRecurringCandidate{},
	}

	categoryCurrent := map[string]float64{}
	categoryPrevious := map[string]float64{}
	merchantCurrent := map[string]*DashboardMerchant{}
	accountCurrent := map[string]*DashboardAccount{}
	dailyCurrent := map[string]*DashboardDailySpend{}
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
		if dailyCurrent[entry.Date] == nil {
			dailyCurrent[entry.Date] = &DashboardDailySpend{Date: entry.Date}
		}
		dailyCurrent[entry.Date].Amount += amount
		dailyCurrent[entry.Date].Count++

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
	for day := dateRange.Start; !day.After(dateRange.End); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		if dailyCurrent[date] != nil {
			response.DailySpending = append(response.DailySpending, *dailyCurrent[date])
			continue
		}
		response.DailySpending = append(response.DailySpending, DashboardDailySpend{Date: date})
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
	response.ReviewItems = dashboardReviewItems(current, entries)
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

func dashboardReviewItems(currentEntries, allEntries []models.Entry) []DashboardReviewItem {
	items := []DashboardReviewItem{}
	for _, entry := range currentEntries {
		category := strings.TrimSpace(entry.Category)
		if category == "" || strings.EqualFold(category, "uncategorized") || entry.AccountID == nil {
			items = append(items, DashboardReviewItem{
				Entry:               entry,
				CategorySuggestions: categorySuggestionsForReviewItem(entry, allEntries),
			})
			if len(items) == 10 {
				return items
			}
		}
	}
	return items
}

func categorySuggestionsForReviewItem(target models.Entry, entries []models.Entry) []string {
	type scoredCategory struct {
		category string
		score    int
	}
	categoryScores := map[string]int{}
	targetMerchant := normalizeInsightMatch(target.Merchant)
	targetTitle := normalizeInsightMatch(target.Title)

	for _, entry := range entries {
		if entry.ID != 0 && target.ID != 0 && entry.ID == target.ID {
			continue
		}
		category := normalizedLabel(entry.Category, "")
		if category == "" || strings.EqualFold(category, "uncategorized") {
			continue
		}

		score := 0
		if targetMerchant != "" && normalizeInsightMatch(entry.Merchant) == targetMerchant {
			score += 3
		}
		if targetTitle != "" && normalizeInsightMatch(entry.Title) == targetTitle {
			score += 2
		}
		if score == 0 {
			continue
		}
		categoryScores[category] += score
	}

	scored := make([]scoredCategory, 0, len(categoryScores))
	for category, score := range categoryScores {
		scored = append(scored, scoredCategory{category: category, score: score})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].category < scored[j].category
	})

	suggestions := []string{}
	for _, item := range scored {
		suggestions = append(suggestions, item.category)
		if len(suggestions) == 3 {
			break
		}
	}
	return suggestions
}

func normalizeInsightMatch(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
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
		amount := dashboard.Summary.TotalSpent
		cards = append(cards, InsightCard{
			Kind: "period_comparison", Severity: "info", Title: "Period comparison",
			Body:             fmt.Sprintf("Spending is %.0f%% %s than the previous period.", math.Abs(change), direction),
			Explanation:      "Compares confirmed expense totals against the immediately preceding equal-length period.",
			ActionLabel:      "Review period transactions",
			Amount:           &amount,
			ChangePercentage: &change,
		})
	}

	for _, category := range dashboard.TopCategories {
		if previous := categoryPrevious[category.Category]; previous > 0 && category.Change >= 20 {
			amount := category.Amount
			percentage := category.Percentage
			change := category.Change
			cards = append(cards, InsightCard{
				Kind: "category_increase", Severity: "warning", Title: category.Category + " increased",
				Body:             fmt.Sprintf("%s spending is %.0f%% higher than the previous period.", category.Category, category.Change),
				Explanation:      "Flags a category that had previous-period activity and increased by at least 20%.",
				ActionLabel:      "Open category",
				Category:         category.Category,
				Amount:           &amount,
				Percentage:       &percentage,
				ChangePercentage: &change,
			})
			break
		}
	}
	if len(dashboard.TopMerchants) > 0 {
		top := dashboard.TopMerchants[0]
		amount := top.Amount
		count := top.TransactionCount
		cards = append(cards, InsightCard{
			Kind: "top_merchant", Severity: "info", Title: "Top merchant",
			Body:             fmt.Sprintf("%s accounted for ₹%.2f across %d transaction(s).", top.Merchant, top.Amount, top.TransactionCount),
			Explanation:      "Finds the merchant with the highest confirmed expense total in the selected period.",
			ActionLabel:      "Open merchant",
			Merchant:         top.Merchant,
			Amount:           &amount,
			TransactionCount: &count,
		})
	}
	if len(dashboard.AccountSpending) > 0 {
		top := dashboard.AccountSpending[0]
		amount := top.Amount
		percentage := top.Percentage
		cards = append(cards, InsightCard{
			Kind: "account_usage", Severity: "info", Title: "Most-used account",
			Body:        fmt.Sprintf("%s handled %.0f%% of spending.", top.AccountName, top.Percentage),
			Explanation: "Ranks linked accounts and payment sources by confirmed expense total.",
			ActionLabel: "Review account spend",
			AccountID:   top.AccountID,
			AccountName: top.AccountName,
			Amount:      &amount,
			Percentage:  &percentage,
		})
	}
	if unusual := unusualExpense(expenseAmounts); unusual > 0 {
		amount := unusual
		cards = append(cards, InsightCard{
			Kind: "unusual_spending", Severity: "warning", Title: "Unusual spending",
			Body:        fmt.Sprintf("A ₹%.2f expense was substantially above this period's average.", unusual),
			Explanation: "Flags an expense that is substantially above this period's average expense size.",
			ActionLabel: "Find large expenses",
			Amount:      &amount,
		})
	}
	if len(dashboard.RecurringCandidates) > 0 {
		candidate := dashboard.RecurringCandidates[0]
		amount := candidate.AverageAmount
		confidence := candidate.Confidence
		cards = append(cards, InsightCard{
			Kind: "recurring_candidate", Severity: "info", Title: "Recurring spend to review",
			Body:             fmt.Sprintf("Review %d likely recurring expense(s), including %s around ₹%.2f.", len(dashboard.RecurringCandidates), candidate.Label, candidate.AverageAmount),
			Explanation:      "Detects repeated merchant or category patterns that look weekly or monthly.",
			ActionLabel:      "Review recurring pattern",
			Category:         candidate.Category,
			Merchant:         candidate.Merchant,
			Amount:           &amount,
			NextExpectedDate: candidate.NextExpectedDate,
			Confidence:       &confidence,
		})
	}
	return cards
}

func applyBudgetStatuses(entries []models.Entry, budgets []models.Budget, dateRange dashboardRange, dashboard *DashboardResponse) {
	if dashboard == nil || len(budgets) == 0 {
		return
	}
	currentStart := dateRange.Start.Format("2006-01-02")
	currentEnd := dateRange.End.Format("2006-01-02")
	daysLeft := budgetDaysLeft(dateRange.End)
	statuses := make([]DashboardBudgetStatus, 0, len(budgets))
	for _, budget := range budgets {
		spent := budgetSpendFromEntries(entries, budget, currentStart, currentEnd)
		limit := budget.LimitAmount.Float64()
		spentValue := spent.Float64()
		remaining := math.Max(0, limit-spentValue)
		status := "safe"
		if spent >= budget.LimitAmount {
			status = "exceeded"
		} else {
			threshold := budget.AlertThresholdPercent
			if threshold == 0 {
				threshold = defaultBudgetAlertThreshold
			}
			if safePercentage(spentValue, limit) >= float64(threshold) {
				status = "watch"
			}
		}
		threshold := budget.AlertThresholdPercent
		if threshold == 0 {
			threshold = defaultBudgetAlertThreshold
		}
		statuses = append(statuses, DashboardBudgetStatus{
			BudgetID: budget.ID, Name: budget.Name, Category: budget.Category,
			LimitAmount: limit, SpentAmount: spentValue, RemainingAmount: remaining,
			Percentage: safePercentage(spentValue, limit), AlertThresholdPercent: threshold,
			DaysLeft: daysLeft, Status: status,
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		if budgetStatusRank(statuses[i].Status) != budgetStatusRank(statuses[j].Status) {
			return budgetStatusRank(statuses[i].Status) > budgetStatusRank(statuses[j].Status)
		}
		return statuses[i].Percentage > statuses[j].Percentage
	})
	dashboard.BudgetStatuses = statuses
	dashboard.Insights = append(dashboard.Insights, buildBudgetInsightCards(statuses)...)
}

func budgetSpendFromEntries(entries []models.Entry, budget models.Budget, start, end string) models.Money {
	total := models.Money(0)
	category := strings.TrimSpace(budget.Category)
	for _, entry := range entries {
		if entry.Date < start || entry.Date > end || !strings.EqualFold(entry.Type, "expense") {
			continue
		}
		if category != "" && !strings.EqualFold(strings.TrimSpace(entry.Category), category) {
			continue
		}
		total += entry.Amount
	}
	return total
}

func budgetDaysLeft(end time.Time) int {
	monthEnd := time.Date(end.Year(), end.Month()+1, 0, 0, 0, 0, 0, end.Location())
	left := int(monthEnd.Sub(end).Hours() / 24)
	if left < 0 {
		return 0
	}
	return left
}

func budgetStatusRank(status string) int {
	switch status {
	case "exceeded":
		return 3
	case "watch":
		return 2
	default:
		return 1
	}
}

func buildBudgetInsightCards(statuses []DashboardBudgetStatus) []InsightCard {
	cards := []InsightCard{}
	for _, status := range statuses {
		if status.Status != "watch" && status.Status != "exceeded" {
			continue
		}
		budgetID := status.BudgetID
		spent := status.SpentAmount
		limit := status.LimitAmount
		remaining := status.RemainingAmount
		percentage := status.Percentage
		target := status.Category
		if strings.TrimSpace(target) == "" {
			target = status.Name
		}
		kind := "budget_watch"
		severity := "warning"
		title := target + " budget nearing limit"
		body := fmt.Sprintf("%s spend is %.0f%% of the ₹%.2f monthly budget with %d day(s) left.", target, status.Percentage, status.LimitAmount, status.DaysLeft)
		if status.Status == "exceeded" {
			kind = "budget_exceeded"
			title = target + " budget exceeded"
			body = fmt.Sprintf("%s spend is %.0f%% of the ₹%.2f monthly budget.", target, status.Percentage, status.LimitAmount)
		}
		cards = append(cards, InsightCard{
			Kind:            kind,
			Severity:        severity,
			Title:           title,
			Body:            body,
			Explanation:     "Compares confirmed expenses in the selected period against your active monthly budget.",
			ActionLabel:     "Review budget",
			Category:        status.Category,
			BudgetID:        &budgetID,
			Amount:          &spent,
			LimitAmount:     &limit,
			RemainingAmount: &remaining,
			Percentage:      &percentage,
			Status:          status.Status,
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
			candidate.CandidateKey = recurringCandidateKey(candidate.Label, candidate.Category)
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

func suppressReviewedRecurringCandidates(userID uint, dashboard *DashboardResponse, rangeEnd time.Time) error {
	if dashboard == nil || len(dashboard.RecurringCandidates) == 0 {
		return nil
	}

	var decisions []models.RecurringCandidateDecision
	if err := database.DB.Where("user_id = ?", userID).Find(&decisions).Error; err != nil {
		return err
	}
	decisionByKey := map[string]models.RecurringCandidateDecision{}
	for _, decision := range decisions {
		decisionByKey[decision.CandidateKey] = decision
	}

	var subscriptions []models.Subscription
	if err := database.DB.Where("user_id = ? AND status IN ?", userID, []string{"active", "paused"}).Find(&subscriptions).Error; err != nil {
		return err
	}

	filtered := make([]DashboardRecurringCandidate, 0, len(dashboard.RecurringCandidates))
	for _, candidate := range dashboard.RecurringCandidates {
		if candidate.CandidateKey == "" {
			candidate.CandidateKey = recurringCandidateKey(candidate.Label, candidate.Category)
		}
		if recurringCandidateHasSubscription(candidate, subscriptions) {
			continue
		}
		decision, exists := decisionByKey[candidate.CandidateKey]
		if exists {
			switch decision.Decision {
			case "dismissed", "tracked":
				continue
			case "snoozed":
				if decision.SnoozedUntil == nil || !decision.SnoozedUntil.Before(rangeEnd) {
					continue
				}
			}
		}
		filtered = append(filtered, candidate)
	}
	dashboard.RecurringCandidates = filtered
	dashboard.Insights = replaceRecurringInsight(dashboard.Insights, filtered)
	return nil
}

func replaceRecurringInsight(cards []InsightCard, candidates []DashboardRecurringCandidate) []InsightCard {
	filtered := make([]InsightCard, 0, len(cards))
	for _, card := range cards {
		if card.Kind != "recurring_candidate" {
			filtered = append(filtered, card)
		}
	}
	if len(candidates) == 0 {
		return filtered
	}
	candidate := candidates[0]
	amount := candidate.AverageAmount
	confidence := candidate.Confidence
	return append(filtered, InsightCard{
		Kind:             "recurring_candidate",
		Severity:         "info",
		Title:            "Recurring spend to review",
		Body:             fmt.Sprintf("Review %d likely recurring expense(s), including %s around ₹%.2f.", len(candidates), candidate.Label, candidate.AverageAmount),
		Explanation:      "Detects repeated merchant or category patterns that look weekly or monthly.",
		ActionLabel:      "Review recurring pattern",
		Category:         candidate.Category,
		Merchant:         candidate.Merchant,
		Amount:           &amount,
		NextExpectedDate: candidate.NextExpectedDate,
		Confidence:       &confidence,
	})
}

func recurringCandidateHasSubscription(candidate DashboardRecurringCandidate, subscriptions []models.Subscription) bool {
	candidateMerchant := normalizeRecurringKeyPart(candidate.Merchant)
	candidateLabel := normalizeRecurringKeyPart(candidate.Label)
	candidateCategory := normalizeRecurringKeyPart(candidate.Category)
	for _, subscription := range subscriptions {
		subMerchant := normalizeRecurringKeyPart(subscription.Merchant)
		subName := normalizeRecurringKeyPart(subscription.Name)
		subCategory := normalizeRecurringKeyPart(subscription.Category)
		if candidateMerchant != "" && (candidateMerchant == subMerchant || candidateMerchant == subName) {
			return true
		}
		if candidateLabel != "" && (candidateLabel == subMerchant || candidateLabel == subName) {
			return true
		}
		if candidateCategory != "" && candidateCategory == subCategory && candidateMerchant == "" {
			return true
		}
	}
	return false
}

func recurringCandidateKey(label, category string) string {
	return normalizeRecurringKeyPart(label) + "|" + normalizeRecurringKeyPart(category)
}

func normalizeRecurringKeyPart(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
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
