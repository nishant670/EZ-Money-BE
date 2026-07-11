package http

import (
	"strings"
	"testing"

	"finance-parser-go/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNotificationPaginationValidation(t *testing.T) {
	page, pageSize, fields := parseNotificationPagination("2", "50")
	if len(fields) != 0 {
		t.Fatalf("expected valid pagination, got %v", fields)
	}
	if page != 2 || pageSize != 50 {
		t.Fatalf("unexpected pagination: page=%d pageSize=%d", page, pageSize)
	}

	_, _, fields = parseNotificationPagination("0", "101")
	if fields["page"] == nil || fields["page_size"] == nil {
		t.Fatalf("expected page and page_size validation errors, got %v", fields)
	}
}

func TestNotificationsSkipStaticBearerGate(t *testing.T) {
	if !skipsStaticBearer("/v1/notifications/unread-count") {
		t.Fatal("notifications should use user-session auth, not the static bearer gate")
	}
}

func TestEntryNotificationBodyUsesBestAvailableLabel(t *testing.T) {
	entry := models.Entry{
		Merchant: "Starbucks",
		Type:     "expense",
		Amount:   testMoney("275.50"),
	}
	body := entryNotificationBody("Added", entry)
	if !strings.Contains(body, "Starbucks") || !strings.Contains(body, "expense") || !strings.Contains(body, "₹275.50") {
		t.Fatalf("unexpected notification body: %q", body)
	}
}

func TestNotificationScopeAlwaysScopesUser(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=test dbname=test sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}

	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return notificationScope(tx, 42).Where("id = ?", 9).Delete(&models.Notification{})
	})
	if !strings.Contains(sql, "user_id") || !strings.Contains(sql, "id") ||
		!strings.Contains(sql, "42") || !strings.Contains(sql, "9") {
		t.Fatalf("ownership predicate missing from notification mutation: %s", sql)
	}
}
