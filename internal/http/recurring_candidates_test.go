package http

import (
	"net/http"
	"testing"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

func TestRecurringCandidateDecisionSuppressesDashboardCandidate(t *testing.T) {
	useSmokeDatabase(t)
	router := smokeRouter(t)
	authResponse := performJSONRequest[AuthResponse](
		t, router, http.MethodPost, "/v1/auth/guest", "", map[string]string{
			"device_id": "recurring-decision-device",
		}, http.StatusOK,
	)
	accounts := performJSONRequest[[]models.Account](
		t, router, http.MethodGet, "/v1/accounts", authResponse.Token, nil, http.StatusOK,
	)
	if len(accounts) == 0 {
		t.Fatal("expected guest account")
	}
	accountID := accounts[0].ID
	amount, err := models.ParseMoney("499")
	if err != nil {
		t.Fatal(err)
	}
	entries := []models.Entry{
		{UserID: authResponse.User.ID, AccountID: &accountID, Amount: amount, Type: "expense", Category: "Entertainment", Merchant: "Streamly", Mode: "UPI", Date: "2026-07-01", Currency: "INR", Source: "manual"},
		{UserID: authResponse.User.ID, AccountID: &accountID, Amount: amount, Type: "expense", Category: "Entertainment", Merchant: "Streamly", Mode: "UPI", Date: "2026-07-08", Currency: "INR", Source: "manual"},
	}
	if err := database.DB.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}

	dashboard := performJSONRequest[DashboardResponse](
		t, router, http.MethodGet,
		"/v1/dashboard?start_date=2026-07-08&end_date=2026-07-14&tz=Asia/Kolkata",
		authResponse.Token, nil, http.StatusOK,
	)
	if len(dashboard.RecurringCandidates) != 1 || dashboard.RecurringCandidates[0].CandidateKey == "" {
		t.Fatalf("expected recurring candidate before decision, got %#v", dashboard.RecurringCandidates)
	}

	decision := performJSONRequest[models.RecurringCandidateDecision](
		t, router, http.MethodPost, "/v1/recurring-candidates/decision", authResponse.Token, map[string]string{
			"candidate_key": dashboard.RecurringCandidates[0].CandidateKey,
			"merchant":      "Streamly",
			"category":      "Entertainment",
			"decision":      "dismissed",
		}, http.StatusOK,
	)
	if decision.Decision != "dismissed" || decision.CandidateKey != dashboard.RecurringCandidates[0].CandidateKey {
		t.Fatalf("unexpected decision response: %#v", decision)
	}

	dashboard = performJSONRequest[DashboardResponse](
		t, router, http.MethodGet,
		"/v1/dashboard?start_date=2026-07-08&end_date=2026-07-14&tz=Asia/Kolkata",
		authResponse.Token, nil, http.StatusOK,
	)
	if len(dashboard.RecurringCandidates) != 0 {
		t.Fatalf("expected dismissed recurring candidate to be suppressed, got %#v", dashboard.RecurringCandidates)
	}
	for _, insight := range dashboard.Insights {
		if insight.Kind == "recurring_candidate" {
			t.Fatalf("expected recurring insight to be suppressed, got %#v", dashboard.Insights)
		}
	}
}
