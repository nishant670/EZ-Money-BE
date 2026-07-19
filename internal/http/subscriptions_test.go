package http

import (
	"testing"
	"time"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

func TestSubscriptionResponseDueStates(t *testing.T) {
	now := mustDate(t, "2026-07-13")
	tests := []struct {
		name         string
		subscription models.Subscription
		wantState    string
		wantDays     int
	}{
		{"scheduled", models.Subscription{Status: "active", NextDueDate: "2026-07-20", ReminderDays: 3}, "scheduled", 7},
		{"due soon", models.Subscription{Status: "active", NextDueDate: "2026-07-15", ReminderDays: 3}, "due_soon", 2},
		{"overdue", models.Subscription{Status: "active", NextDueDate: "2026-07-12", ReminderDays: 3}, "overdue", -1},
		{"paused", models.Subscription{Status: "paused", NextDueDate: "2026-07-12", ReminderDays: 3}, "paused", -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := buildSubscriptionResponse(test.subscription, now)
			if response.DueState != test.wantState || response.DaysUntilDue != test.wantDays {
				t.Fatalf("response = %#v", response)
			}
		})
	}
}

func TestAdvanceSubscriptionDueDate(t *testing.T) {
	paid := mustDate(t, "2026-07-13")
	tests := []struct {
		name     string
		dueDate  string
		interval string
		want     string
	}{
		{"monthly after due", "2026-07-10", "monthly", "2026-08-10"},
		{"daily catches up", "2026-07-10", "daily", "2026-07-14"},
		{"weekly catches up", "2026-06-20", "weekly", "2026-07-18"},
		{"biweekly catches up", "2026-06-20", "biweekly", "2026-07-18"},
		{"early payment advances one cycle", "2026-07-20", "monthly", "2026-08-20"},
		{"quarterly", "2026-07-01", "quarterly", "2026-10-01"},
		{"yearly", "2026-07-01", "yearly", "2027-07-01"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := advanceSubscriptionDueDate(test.dueDate, test.interval, paid)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("next due = %s, want %s", got, test.want)
			}
		})
	}
}

func TestSubscriptionRemindersDeduplicateByDueDateAndKind(t *testing.T) {
	useSmokeDatabase(t)
	user := createBudgetTestUser(t)
	subscription := models.Subscription{
		UserID: user.ID, Name: "Streamly", Amount: testMoney("499"), Currency: "INR",
		BillingInterval: "monthly", NextDueDate: "2026-07-15", Status: "active", ReminderDays: 3,
	}
	if err := database.DB.Create(&subscription).Error; err != nil {
		t.Fatal(err)
	}

	created, err := syncSubscriptionReminders(user.ID, mustDate(t, "2026-07-13"))
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("created = %d, want 1", created)
	}
	created, err = syncSubscriptionReminders(user.ID, mustDate(t, "2026-07-13"))
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("duplicate created = %d, want 0", created)
	}

	assertBudgetNotificationCount(t, user.ID, "subscription.due", 1)
}

func TestSubscriptionCancelBeforeDueReminderCopy(t *testing.T) {
	subscription := models.Subscription{
		Name: "Streamly", Amount: testMoney("499"), CancelBeforeDue: true,
	}
	title, body := subscriptionReminderCopy(subscription, "2026-07-20", subscriptionReminderDueKind)
	if title != "Cancel before renewal" || body == "" {
		t.Fatalf("copy = %q %q", title, body)
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
