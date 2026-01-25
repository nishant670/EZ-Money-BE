package http

import (
	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type MonthlyHealth struct {
	Income      float64 `json:"income"`
	Spent       float64 `json:"spent"`
	Savings     float64 `json:"savings"`
	SavingsRate float64 `json:"savings_rate"` // Percentage
	BurnRate    string  `json:"burn_rate"`    // Days remaining
}

type CategoryBreakdown struct {
	Category   string  `json:"category"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
	Change     float64 `json:"change"` // vs last month
}

type MerchantInfo struct {
	Merchant         string  `json:"merchant"`
	Amount           float64 `json:"amount"`
	TransactionCount int     `json:"transaction_count"`
	Icon             string  `json:"icon"`
}

type AIInsightCard struct {
	Type        string `json:"type"` // info, warning, success
	Title       string `json:"title"`
	Description string `json:"description"`
	ActionLabel string `json:"action_label"`
	ActionType  string `json:"action_type"`
}

type AccountSpending struct {
	Type       string  `json:"type"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type CreditUtilization struct {
	AccountName string  `json:"account_name"`
	Used        float64 `json:"used"`
	Limit       float64 `json:"limit"`
	Percentage  float64 `json:"percentage"`
	DueDate     string  `json:"due_date"`
	Warning     bool    `json:"warning"`
}

type EMISummary struct {
	TotalMonthlyEMI float64 `json:"total_monthly_emi"`
	TotalLent       float64 `json:"total_lent"`
	LentCount       int     `json:"lent_count"`
}

type BehavioralInsight struct {
	AverageDailySpend float64 `json:"average_daily_spend"`
	HighestSpendDay   string  `json:"highest_spend_day"`
}

type ReviewItem struct {
	Type  string `json:"type"` // uncategorized, missing_account, duplicates
	Count int    `json:"count"`
	Title string `json:"title"`
}

type InsightsResponse struct {
	MonthlyHealth      MonthlyHealth       `json:"monthly_health"`
	CategoryBreakdown  []CategoryBreakdown `json:"category_breakdown"`
	TopMerchants       []MerchantInfo      `json:"top_merchants"`
	AIInsights         []AIInsightCard     `json:"ai_insights"`
	AccountSpending    []AccountSpending   `json:"account_spending"`
	CreditUtilization  []CreditUtilization `json:"credit_utilization"`
	EMISummary         EMISummary          `json:"emi_summary"`
	BehavioralInsights BehavioralInsight   `json:"behavioral_insights"`
	ReviewItems        []ReviewItem        `json:"review_items"`
}

func (s *Server) getInsights(c *gin.Context) {
	userId := c.MustGet("userID").(uint)
	now := time.Now()
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	lastMonthStart := now.AddDate(0, -1, 0)
	lastMonthStartStr := time.Date(lastMonthStart.Year(), lastMonthStart.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	lastMonthEndStr := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -1).Format("2006-01-02")

	var entries []models.Entry
	database.DB.Where("user_id = ? AND date >= ?", userId, lastMonthStartStr).Find(&entries)

	var accounts []models.Account
	database.DB.Where("user_id = ?", userId).Find(&accounts)

	res := InsightsResponse{
		CategoryBreakdown: []CategoryBreakdown{},
		TopMerchants:      []MerchantInfo{},
		AIInsights:        []AIInsightCard{},
		AccountSpending:   []AccountSpending{},
		CreditUtilization: []CreditUtilization{},
		ReviewItems:       []ReviewItem{},
	}

	// 1. Monthly Health
	var thisMonthIncome, thisMonthSpent float64
	var lastMonthSpent float64
	categorySpendThis := make(map[string]float64)
	categorySpendLast := make(map[string]float64)
	merchantSpend := make(map[string]*MerchantInfo)
	accountSpend := make(map[string]float64)
	dailySpend := make(map[string]float64)

	for _, e := range entries {
		if e.Date >= thisMonthStart {
			if strings.ToLower(e.Type) == "income" {
				thisMonthIncome += e.Amount
			} else if strings.ToLower(e.Type) == "expense" {
				thisMonthSpent += e.Amount
				categorySpendThis[e.Category] += e.Amount
				if e.Merchant != "" {
					if _, ok := merchantSpend[e.Merchant]; !ok {
						merchantSpend[e.Merchant] = &MerchantInfo{Merchant: e.Merchant}
					}
					merchantSpend[e.Merchant].Amount += e.Amount
					merchantSpend[e.Merchant].TransactionCount++
				}
				accountSpend[e.Mode] += e.Amount
				dailySpend[e.Date] += e.Amount
			}
		} else if e.Date >= lastMonthStartStr && e.Date <= lastMonthEndStr {
			if strings.ToLower(e.Type) == "expense" {
				lastMonthSpent += e.Amount
				categorySpendLast[e.Category] += e.Amount
			}
		}
	}

	res.MonthlyHealth.Income = thisMonthIncome
	res.MonthlyHealth.Spent = thisMonthSpent
	res.MonthlyHealth.Savings = thisMonthIncome - thisMonthSpent
	if thisMonthIncome > 0 {
		res.MonthlyHealth.SavingsRate = math.Max(0, (res.MonthlyHealth.Savings/thisMonthIncome)*100)
	}

	// Burn Rate Calculation (Very simplified: based on this month's spending and current day)
	currentDay := now.Day()
	if currentDay > 0 && thisMonthSpent > 0 {
		avgDaily := thisMonthSpent / float64(currentDay)
		// Assume user has some balance, or just use income - spent
		balance := thisMonthIncome - thisMonthSpent
		if balance > 0 && avgDaily > 0 {
			daysLeft := balance / avgDaily
			res.MonthlyHealth.BurnRate = strings.Split(time.Duration(daysLeft*24*float64(time.Hour)).String(), "h")[0] // Rough estimate
			res.MonthlyHealth.BurnRate = fmt.Sprintf("At your current spending, your balance will last ~%.0f days.", daysLeft)
		} else {
			res.MonthlyHealth.BurnRate = "Your spending is currently exceeding your income."
		}
	}

	// 2. Category Breakdown
	for cat, amt := range categorySpendThis {
		percentage := 0.0
		if thisMonthSpent > 0 {
			percentage = (amt / thisMonthSpent) * 100
		}
		lastAmt := categorySpendLast[cat]
		change := 0.0
		if lastAmt > 0 {
			change = ((amt - lastAmt) / lastAmt) * 100
		}
		res.CategoryBreakdown = append(res.CategoryBreakdown, CategoryBreakdown{
			Category:   cat,
			Amount:     amt,
			Percentage: percentage,
			Change:     change,
		})
	}
	sort.Slice(res.CategoryBreakdown, func(i, j int) bool {
		return res.CategoryBreakdown[i].Amount > res.CategoryBreakdown[j].Amount
	})

	// 3. Top Merchants
	for _, info := range merchantSpend {
		res.TopMerchants = append(res.TopMerchants, *info)
	}
	sort.Slice(res.TopMerchants, func(i, j int) bool {
		return res.TopMerchants[i].Amount > res.TopMerchants[j].Amount
	})
	if len(res.TopMerchants) > 5 {
		res.TopMerchants = res.TopMerchants[:5]
	}

	// 4. AI Insights (Mocked logic)
	if res.MonthlyHealth.SavingsRate < 10 {
		res.AIInsights = append(res.AIInsights, AIInsightCard{
			Type:        "warning",
			Title:       "Low Savings Rate",
			Description: "Your savings rate is below 10%. Consider reviewing non-essential expenses.",
			ActionLabel: "Review Expenses",
			ActionType:  "navigate_transactions",
		})
	}

	if len(res.TopMerchants) > 0 && res.TopMerchants[0].Amount > thisMonthIncome*0.2 {
		res.AIInsights = append(res.AIInsights, AIInsightCard{
			Type:        "info",
			Title:       "High Merchant Spend",
			Description: fmt.Sprintf("You've spent %.0f%% of your income at %s alone.", (res.TopMerchants[0].Amount/thisMonthIncome)*100, res.TopMerchants[0].Merchant),
			ActionLabel: "View Details",
			ActionType:  "view_merchant",
		})
	}

	// 5. Account Intelligence
	for mode, amt := range accountSpend {
		percentage := 0.0
		if thisMonthSpent > 0 {
			percentage = (amt / thisMonthSpent) * 100
		}
		res.AccountSpending = append(res.AccountSpending, AccountSpending{
			Type:       mode,
			Amount:     amt,
			Percentage: percentage,
		})
	}

	for _, acc := range accounts {
		if strings.EqualFold(acc.Type, "credit") && acc.CreditLimit > 0 {
			used := 0.0 // Needs careful logic to track specific card spend. For now using mode match.
			for _, e := range entries {
				if e.Date >= thisMonthStart && strings.EqualFold(e.Mode, acc.Name) {
					used += e.Amount
				}
			}
			utilization := (used / acc.CreditLimit) * 100
			res.CreditUtilization = append(res.CreditUtilization, CreditUtilization{
				AccountName: acc.Name,
				Used:        used,
				Limit:       acc.CreditLimit,
				Percentage:  utilization,
				DueDate:     fmt.Sprintf("%d", acc.DueDay),
				Warning:     utilization > 60,
			})
		}
	}

	// 6. EMI Summary (Mocked logic based on tags)
	var emiTotal float64
	var lentTotal float64
	var lentCount int
	for _, e := range entries {
		if e.Date >= thisMonthStart {
			if strings.Contains(strings.ToLower(e.Tag), "emi") {
				emiTotal += e.Amount
			}
			if strings.Contains(strings.ToLower(e.Tag), "lent") {
				lentTotal += e.Amount
				lentCount++
			}
		}
	}
	res.EMISummary.TotalMonthlyEMI = emiTotal
	res.EMISummary.TotalLent = lentTotal
	res.EMISummary.LentCount = lentCount

	// 7. Behavioral Insights
	if currentDay > 0 {
		res.BehavioralInsights.AverageDailySpend = thisMonthSpent / float64(currentDay)
	}
	// Highest Spend Day
	weekdaySpends := make(map[string]float64)
	for d, amt := range dailySpend {
		t, _ := time.Parse("2006-01-02", d)
		weekdaySpends[t.Weekday().String()] += amt
	}
	highestAmt := 0.0
	highestDay := ""
	for day, amt := range weekdaySpends {
		if amt > highestAmt {
			highestAmt = amt
			highestDay = day
		}
	}
	res.BehavioralInsights.HighestSpendDay = highestDay

	// 8. Review Items
	uncategorized := 0
	for _, e := range entries {
		if e.Date >= thisMonthStart && (e.Category == "" || strings.ToLower(e.Category) == "uncategorized" || strings.ToLower(e.Category) == "other") {
			uncategorized++
		}
	}
	if uncategorized > 0 {
		res.ReviewItems = append(res.ReviewItems, ReviewItem{
			Type:  "uncategorized",
			Count: uncategorized,
			Title: "Uncategorized Transactions",
		})
	}

	c.JSON(http.StatusOK, res)
}
