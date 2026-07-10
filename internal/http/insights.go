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

type InsightCard struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

type DashboardResponse struct {
	Period             DashboardPeriod     `json:"period"`
	Summary            DashboardSummary    `json:"summary"`
	TopCategories      []DashboardCategory `json:"top_categories"`
	TopMerchants       []DashboardMerchant `json:"top_merchants"`
	AccountSpending    []DashboardAccount  `json:"account_spending"`
	RecentTransactions []models.Entry      `json:"recent_transactions"`
	Insights           []InsightCard       `json:"insights"`
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
		Insights: []InsightCard{},
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
	return cards
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
