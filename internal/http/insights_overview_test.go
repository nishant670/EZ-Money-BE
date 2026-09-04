package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

// The reported symptom, as a test: a month with nothing in it must not make an
// account with a year of history look empty.
//
// Everything on the Insights tab read off the selected window, so on the 1st of
// a month — before a single transaction had been captured — a rich account
// rendered ₹0 four times over "Waiting for data". The overview block is what
// the screen falls back on, and it has to be full while the period is empty.
func TestDashboardOverviewSurvivesAnEmptySelectedPeriod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	user, token := createPaidBillingTestUserSession(t)
	accounts := performJSONRequest[[]models.Account](
		t, router, http.MethodGet, "/v1/accounts", token, nil, http.StatusOK,
	)

	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)
	twoMonthsAgo := now.AddDate(0, -2, 0)
	seedOverviewEntry(t, user.ID, accounts[0].ID, lastMonth.Format("2006-01")+"-05", "4000.00")
	seedOverviewEntry(t, user.ID, accounts[0].ID, lastMonth.Format("2006-01")+"-19", "2000.00")
	seedOverviewEntry(t, user.ID, accounts[0].ID, twoMonthsAgo.Format("2006-01")+"-11", "9000.00")

	// Ask for a window that deliberately holds nothing: a fortnight far in the
	// future is the same shape as "the 1st of a new month".
	future := now.AddDate(0, 3, 0).Format("2006-01-02")
	dashboard := performJSONRequest[DashboardResponse](
		t, router, http.MethodGet,
		"/v1/dashboard?start_date="+future+"&end_date="+future, token, nil, http.StatusOK,
	)

	if dashboard.Summary.TransactionCount != 0 {
		t.Fatalf("expected the selected window to be empty, got %#v", dashboard.Summary)
	}
	if !dashboard.Overview.HasHistory {
		t.Fatal("an account with three transactions has history, whatever window is selected")
	}
	if dashboard.Overview.LifetimeSpent != 15000 {
		t.Fatalf("expected 15000 lifetime spend, got %v", dashboard.Overview.LifetimeSpent)
	}
	if dashboard.Overview.LifetimeTransactionCount != 3 {
		t.Fatalf("expected 3 lifetime transactions, got %d", dashboard.Overview.LifetimeTransactionCount)
	}

	// The one thing that turns a dead end into a next step.
	if dashboard.Overview.LastActiveMonth == nil {
		t.Fatal("expected the empty period to be told where the data actually is")
	}
	if dashboard.Overview.LastActiveMonth.Month != lastMonth.Format("2006-01") {
		t.Fatalf("expected last active month %s, got %s",
			lastMonth.Format("2006-01"), dashboard.Overview.LastActiveMonth.Month)
	}
	if dashboard.Overview.LastActiveMonth.Spent != 6000 {
		t.Fatalf("expected 6000 spent last month, got %v", dashboard.Overview.LastActiveMonth.Spent)
	}
}

// The strip keeps its quiet months. Dropping them compresses time, and a gap
// then draws as a decline.
func TestDashboardOverviewKeepsEmptyMonthsInTheStrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	user, token := createPaidBillingTestUserSession(t)
	accounts := performJSONRequest[[]models.Account](
		t, router, http.MethodGet, "/v1/accounts", token, nil, http.StatusOK,
	)
	seedOverviewEntry(t, user.ID, accounts[0].ID,
		time.Now().AddDate(0, -1, 0).Format("2006-01")+"-08", "1500.00")

	dashboard := performJSONRequest[DashboardResponse](
		t, router, http.MethodGet, "/v1/dashboard", token, nil, http.StatusOK,
	)
	if len(dashboard.Overview.RecentMonths) != overviewMonths {
		t.Fatalf("expected %d months in the strip, got %d",
			overviewMonths, len(dashboard.Overview.RecentMonths))
	}
	filled := 0
	for _, month := range dashboard.Overview.RecentMonths {
		if month.Count > 0 {
			filled++
		}
	}
	if filled != 1 {
		t.Fatalf("expected exactly one month with activity, got %d", filled)
	}
}

// A genuinely new account is the one case where an empty screen is honest, and
// it has to stay distinguishable from a quiet month.
func TestDashboardOverviewReportsNoHistoryForANewAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	_, token := createPaidBillingTestUserSession(t)

	dashboard := performJSONRequest[DashboardResponse](
		t, router, http.MethodGet, "/v1/dashboard", token, nil, http.StatusOK,
	)
	if dashboard.Overview.HasHistory {
		t.Fatalf("a new account has no history, got %#v", dashboard.Overview)
	}
	if len(dashboard.Overview.RecentMonths) != 0 || dashboard.Overview.LastActiveMonth != nil {
		t.Fatalf("expected nothing to point at, got %#v", dashboard.Overview)
	}
}

// The typical month is a median, because one holiday should not redefine what
// an ordinary month costs.
func TestTypicalMonthlySpendResistsAnOutlierMonth(t *testing.T) {
	months := []DashboardOverviewMonth{
		{Month: "2026-04", Spent: 20000, Count: 12},
		{Month: "2026-05", Spent: 21000, Count: 14},
		{Month: "2026-06", Spent: 19000, Count: 11},
		{Month: "2026-07", Spent: 120000, Count: 20}, // a wedding
		{Month: "2026-08", Spent: 20500, Count: 13},
	}
	mean, median := baselineMonthlySpend(months)
	if median > 21000 {
		t.Fatalf("the median must describe an ordinary month, got %v", median)
	}
	if mean <= median {
		t.Fatalf("this fixture exists because the mean is dragged up; mean=%v median=%v", mean, median)
	}
}

// Months with no activity are left out of the baseline entirely. Counting them
// as ₹0 measures when somebody started using the app, not what they spend.
func TestBaselineMonthlySpendIgnoresMonthsWithNoActivity(t *testing.T) {
	months := []DashboardOverviewMonth{
		{Month: "2026-06", Spent: 0, Count: 0},
		{Month: "2026-07", Spent: 0, Count: 0},
		{Month: "2026-08", Spent: 30000, Count: 18},
	}
	mean, median := baselineMonthlySpend(months)
	if mean != 30000 || median != 30000 {
		t.Fatalf("expected the one real month to stand alone, mean=%v median=%v", mean, median)
	}
}

func seedOverviewEntry(t *testing.T, userID, accountID uint, date, amount string) {
	t.Helper()
	money, err := models.ParseMoney(amount)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", amount, err)
	}
	entry := models.Entry{
		UserID:    userID,
		AccountID: &accountID,
		Title:     "Seeded",
		Type:      "expense",
		Amount:    money,
		Currency:  "INR",
		Source:    "manual",
		Mode:      "UPI",
		Category:  "Food & Drinks",
		Date:      date,
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		t.Fatalf("failed to seed entry: %v", err)
	}
}
