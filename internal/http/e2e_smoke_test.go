package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xeipuuv/gojsonschema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

func TestGuestCaptureParseConfirmSaveDashboardSmoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)

	authResponse := performJSONRequest[AuthResponse](
		t, router, http.MethodPost, "/v1/auth/guest", "", map[string]string{
			"device_id": "smoke-test-device",
		}, http.StatusOK,
	)
	if !strings.HasPrefix(authResponse.Token, "fnr_") {
		t.Fatalf("expected opaque guest session token, got %q", authResponse.Token)
	}

	accounts := performJSONRequest[[]models.Account](
		t, router, http.MethodGet, "/v1/accounts", authResponse.Token, nil, http.StatusOK,
	)
	if len(accounts) != 1 || !accounts[0].IsDefault || accounts[0].Name != "Cash" {
		t.Fatalf("expected guest default Cash account, got %#v", accounts)
	}
	accountID := accounts[0].ID

	form := url.Values{"hint_text": {"chai 80 cash"}}
	parseRequest := httptest.NewRequest(http.MethodPost, "/v1/parse", strings.NewReader(form.Encode()))
	parseRequest.Header.Set("Authorization", "Bearer "+authResponse.Token)
	parseRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	parseResponse := httptest.NewRecorder()
	router.ServeHTTP(parseResponse, parseRequest)
	if parseResponse.Code != http.StatusOK {
		t.Fatalf("parse status = %d, body = %s", parseResponse.Code, parseResponse.Body.String())
	}

	var draft map[string]any
	if err := json.Unmarshal(parseResponse.Body.Bytes(), &draft); err != nil {
		t.Fatalf("failed to decode parse response: %v", err)
	}
	if draft["stage"] != "draft" || draft["source_text"] != "chai 80 cash" {
		t.Fatalf("unexpected parse draft: %#v", draft)
	}
	if _, persisted := draft["id"]; persisted {
		t.Fatalf("parse response should be an unpersisted draft: %#v", draft)
	}

	confirmed := map[string]any{
		"title":       draft["title"],
		"type":        draft["type"],
		"amount":      draft["amount"],
		"currency":    "INR",
		"source":      "text",
		"account_id":  accountID,
		"mode":        draft["mode"],
		"category":    draft["category"],
		"merchant":    draft["merchant"],
		"date":        draft["date"],
		"time":        "09:15",
		"source_text": draft["source_text"],
	}
	savedEntry := performJSONRequest[models.Entry](
		t, router, http.MethodPost, "/v1/entries", authResponse.Token, confirmed, http.StatusCreated,
	)
	if savedEntry.ID == 0 || savedEntry.AccountID == nil || *savedEntry.AccountID != accountID {
		t.Fatalf("saved entry did not retain the confirmed account: %#v", savedEntry)
	}

	dashboard := performJSONRequest[DashboardResponse](
		t, router, http.MethodGet,
		"/v1/dashboard?start_date=2026-07-12&end_date=2026-07-12&tz=Asia/Kolkata",
		authResponse.Token, nil, http.StatusOK,
	)
	if dashboard.Summary.TransactionCount != 1 || dashboard.Summary.TotalSpent != 80 {
		t.Fatalf("dashboard summary did not include saved entry: %#v", dashboard.Summary)
	}
	if len(dashboard.RecentTransactions) != 1 || dashboard.RecentTransactions[0].ID != savedEntry.ID {
		t.Fatalf("dashboard recent transactions did not include saved entry: %#v", dashboard.RecentTransactions)
	}
	if len(dashboard.TopCategories) != 1 || dashboard.TopCategories[0].Category != "Food" {
		t.Fatalf("dashboard category rollup did not include parsed category: %#v", dashboard.TopCategories)
	}
}

func smokeRouter(t *testing.T) *gin.Engine {
	t.Helper()

	schemaPath := filepath.Join(projectRoot(t), "schemas", "expense_entry.schema.json")
	schema, err := gojsonschema.NewSchema(gojsonschema.NewReferenceLoader("file://" + schemaPath))
	if err != nil {
		t.Fatalf("failed to load parse schema: %v", err)
	}

	cfg := &config.Config{
		TZDefault:          "Asia/Kolkata",
		ReqTimeoutSec:      2,
		RateLimitRPS:       1000,
		RateLimitBurst:     1000,
		MaxJSONKB:          64,
		MaxUploadMB:        1,
		MaxTranscriptChars: 1000,
	}
	server := &Server{
		cfg:       cfg,
		validator: schema,
		parser: fixtureParser{result: []byte(`{
			"title":"Chai",
			"amount":80,
			"type":"expense",
			"currency":"INR",
			"mode":"Cash",
			"category":"Food",
			"merchant":"Tea Stall",
			"date":"2026-07-12"
		}`)},
	}

	router := gin.New()
	auth := router.Group("/v1/auth")
	auth.Use(jsonRequestLimits(cfg), rateLimit(cfg, "auth"))
	auth.POST("/guest", server.authGuest)

	authorized := router.Group("/v1")
	authorized.Use(AuthMiddleware())
	authorized.POST("/parse", uploadRequestLimits(cfg), rateLimit(cfg, "ai"), server.handleParse)
	authorized.GET("/accounts", server.listAccounts)
	authorized.POST("/entries", server.saveEntry)
	authorized.GET("/dashboard", server.getDashboard)
	authorized.POST("/split/friends", server.createSplitFriend)
	authorized.GET("/split/friends", server.listSplitFriends)
	authorized.POST("/split/bills", server.createSplitBill)
	authorized.GET("/split/bills", server.listSplitBills)
	authorized.POST("/split/settlements", server.createSplitSettlement)
	authorized.GET("/split/settlements", server.listSplitSettlements)
	authorized.GET("/split/balances", server.listSplitBalances)
	return router
}

func useSmokeDatabase(t *testing.T) {
	t.Helper()

	previous := database.DB
	db, err := gorm.Open(sqlite.Open("file:finnri_smoke?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open smoke database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to access smoke database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&models.User{},
		&models.AuthSession{},
		&models.AuthVerification{},
		&models.Account{},
		&models.Entry{},
		&models.QuickPrompt{},
		&models.Notification{},
		&models.SplitFriend{},
		&models.SplitBill{},
		&models.SplitParticipant{},
		&models.SplitSettlement{},
	); err != nil {
		t.Fatalf("failed to migrate smoke database: %v", err)
	}

	database.DB = db
	t.Cleanup(func() {
		database.DB = previous
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("failed to close smoke database: %v", err)
		}
	})
}

func performJSONRequest[T any](
	t *testing.T,
	router *gin.Engine,
	method string,
	target string,
	token string,
	body any,
	expectedStatus int,
) T {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to encode request body: %v", err)
		}
		requestBody = bytes.NewReader(encoded)
	}

	request := httptest.NewRequest(method, target, requestBody)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != expectedStatus {
		t.Fatalf("%s %s status = %d, body = %s", method, target, response.Code, response.Body.String())
	}

	var decoded T
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode %s %s response: %v; body = %s", method, target, err, response.Body.String())
	}
	return decoded
}
