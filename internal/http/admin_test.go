package http

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"finance-parser-go/internal/billing"
	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

func TestAdminEndpointsRequireConfiguredBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)
	router := smokeRouter(t)

	response := performJSONRequest[map[string]any](t, router, http.MethodGet, "/v1/admin/ai/metrics?date=2026-07-19", "", nil, http.StatusUnauthorized)
	if response["error"] != "admin_unauthorized" {
		t.Fatalf("unexpected admin auth response: %#v", response)
	}
}

func TestAdminMetricsAndCreditAdjustment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)
	router := smokeRouter(t)
	user, _ := createBillingTestUserSession(t)

	adjustment := performJSONRequest[struct {
		Grant   models.CreditGrant `json:"grant"`
		Created bool               `json:"created"`
	}](t, router, http.MethodPost, "/v1/admin/credits/adjustments", "admin-test-token", map[string]any{
		"user_id":     user.ID,
		"credits":     25,
		"reason_code": "support_refund",
	}, http.StatusCreated)
	if !adjustment.Created || adjustment.Grant.CreditsRemaining != 25 {
		t.Fatalf("unexpected adjustment: %#v", adjustment)
	}

	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	event := models.AIUsageEvent{
		UserID:                 &user.ID,
		RequestID:              "ai_admin_metrics_1",
		ActionCode:             "transaction_parse_text",
		InputKind:              "text",
		Status:                 billing.UsageStatusSucceeded,
		EstimatedCredits:       5,
		ReservedCredits:        5,
		FinalCredits:           5,
		EstimatedCostUSDMicros: 500,
		StartedAt:              now,
	}
	if err := database.DB.Create(&event).Error; err != nil {
		t.Fatal(err)
	}

	metrics := performJSONRequest[billing.AIMetrics](t, router, http.MethodGet, "/v1/admin/ai/metrics?date=2026-07-19", "admin-test-token", nil, http.StatusOK)
	if metrics.TotalEvents != 1 || metrics.EstimatedCostUSDMicros != 500 {
		t.Fatalf("unexpected admin metrics: %#v", metrics)
	}
}

func TestAdminModelPricingAndLifetimeQuoteList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)
	router := smokeRouter(t)
	user, _ := createBillingTestUserSession(t)

	invalid := performJSONRequest[map[string]any](t, router, http.MethodPut, "/v1/admin/ai/model-pricing", "admin-test-token", map[string]any{
		"provider":                "openai",
		"model":                   "gpt-4o-mini",
		"operation":               "unsupported",
		"input_token_usd_micros":  -1,
		"output_token_usd_micros": 6,
	}, http.StatusUnprocessableEntity)
	if invalid["error"] != "invalid_model_pricing" {
		t.Fatalf("unexpected invalid pricing response: %#v", invalid)
	}

	upserted := performJSONRequest[models.AIModelPricing](t, router, http.MethodPut, "/v1/admin/ai/model-pricing", "admin-test-token", map[string]any{
		"provider":                "openai",
		"model":                   "gpt-4o-mini",
		"operation":               "llm",
		"input_token_usd_micros":  2,
		"output_token_usd_micros": 6,
		"request_usd_micros":      10,
		"credit_usd_micros":       100,
	}, http.StatusOK)
	if upserted.ID == 0 || upserted.InputTokenUSDMicros != 2 {
		t.Fatalf("unexpected upserted pricing: %#v", upserted)
	}
	pricing := performJSONRequest[struct {
		Pricing []models.AIModelPricing `json:"pricing"`
	}](t, router, http.MethodGet, "/v1/admin/ai/model-pricing", "admin-test-token", nil, http.StatusOK)
	if len(pricing.Pricing) != 1 {
		t.Fatalf("expected one pricing row, got %#v", pricing)
	}

	quote := models.LifetimeQuoteRequest{
		UserID:              user.ID,
		Status:              "requested",
		PaidMonthsCompleted: 3,
		UsageWindowStart:    time.Now().UTC().AddDate(0, 0, -90),
		UsageWindowEnd:      time.Now().UTC(),
	}
	if err := database.DB.Create(&quote).Error; err != nil {
		t.Fatal(err)
	}
	quotes := performJSONRequest[struct {
		Requests []models.LifetimeQuoteRequest `json:"requests"`
		Total    int64                         `json:"total"`
	}](t, router, http.MethodGet, "/v1/admin/billing/lifetime-quotes?status=requested", "admin-test-token", nil, http.StatusOK)
	if quotes.Total != 1 || len(quotes.Requests) != 1 {
		t.Fatalf("unexpected lifetime quote list: %#v", quotes)
	}
}

func TestAdminAIAbuseBlockLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)
	router := smokeRouter(t)

	created := performJSONRequest[models.AIAbuseBlock](t, router, http.MethodPost, "/v1/admin/ai/abuse-blocks", "admin-test-token", map[string]any{
		"guest_device_id_hash": "admin-block-device-123",
		"scope":                "ai_parse",
		"reason_code":          "abuse_review",
		"notes":                "support requested temporary block",
		"created_by":           "support",
	}, http.StatusCreated)
	if created.ID == 0 || !created.Active || created.GuestDeviceIDHash == "admin-block-device-123" || len(created.GuestDeviceIDHash) != 64 {
		t.Fatalf("unexpected abuse block: %#v", created)
	}

	listed := performJSONRequest[struct {
		Blocks []models.AIAbuseBlock `json:"blocks"`
		Total  int64                 `json:"total"`
	}](t, router, http.MethodGet, "/v1/admin/ai/abuse-blocks?active=true", "admin-test-token", nil, http.StatusOK)
	if listed.Total != 1 || len(listed.Blocks) != 1 {
		t.Fatalf("unexpected abuse block list: %#v", listed)
	}

	updated := performJSONRequest[models.AIAbuseBlock](t, router, http.MethodPatch, "/v1/admin/ai/abuse-blocks/"+strconv.Itoa(int(created.ID)), "admin-test-token", map[string]any{
		"active": false,
		"notes":  "review complete",
	}, http.StatusOK)
	if updated.Active {
		t.Fatalf("expected block to be inactive: %#v", updated)
	}
}
