package http

import (
	"strings"
	"time"

	"finance-parser-go/internal/models"
)

const (
	subscriptionIntervalDaily         = "daily"
	subscriptionIntervalBusinessDaily = "business_daily"
	subscriptionIntervalWeekly        = "weekly"
	subscriptionIntervalBiweekly      = "biweekly"
	subscriptionIntervalMonthly       = "monthly"
	subscriptionIntervalQuarterly     = "quarterly"
	subscriptionIntervalYearly        = "yearly"

	subscriptionStatusActive    = "active"
	subscriptionStatusPaused    = "paused"
	subscriptionStatusCancelled = "cancelled"

	defaultSubscriptionReminderDays = 3
	maxSubscriptionReminderDays     = 30
	maxSubscriptionNameLength       = 120
)

type subscriptionInput struct {
	AccountID       *uint        `json:"account_id"`
	Name            string       `json:"name"`
	Merchant        string       `json:"merchant"`
	Category        string       `json:"category"`
	Amount          models.Money `json:"amount"`
	Currency        string       `json:"currency"`
	BillingInterval string       `json:"billing_interval"`
	NextDueDate     string       `json:"next_due_date"`
	LastChargedDate string       `json:"last_charged_date"`
	Status          string       `json:"status"`
	ReminderDays    *int         `json:"reminder_days"`
	CancelBeforeDue bool         `json:"cancel_before_due"`
	Notes           string       `json:"notes"`
}

type markSubscriptionPaidInput struct {
	PaidDate string `json:"paid_date"`
}

func (input subscriptionInput) validate() map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = "is required"
	} else if len([]rune(strings.TrimSpace(input.Name))) > maxSubscriptionNameLength {
		fields["name"] = "must not exceed 120 characters"
	}
	if !input.Amount.IsPositive() {
		fields["amount"] = "must be greater than zero"
	}
	if currency := normalizeSubscriptionCurrency(input.Currency); currency != "" && currency != "INR" {
		fields["currency"] = "must be INR"
	}
	if normalizeSubscriptionInterval(input.BillingInterval) == "" {
		fields["billing_interval"] = "must be daily, business_daily, weekly, biweekly, monthly, quarterly, or yearly"
	}
	if _, err := parseStrictAPIDate(input.NextDueDate); err != nil {
		fields["next_due_date"] = "must use YYYY-MM-DD"
	}
	if strings.TrimSpace(input.LastChargedDate) != "" {
		if _, err := parseStrictAPIDate(input.LastChargedDate); err != nil {
			fields["last_charged_date"] = "must use YYYY-MM-DD"
		}
	}
	if normalizeSubscriptionStatus(input.Status) == "" {
		fields["status"] = "must be active, paused, or cancelled"
	}
	if input.ReminderDays != nil && (*input.ReminderDays < 0 || *input.ReminderDays > maxSubscriptionReminderDays) {
		fields["reminder_days"] = "must be between 0 and 30"
	}
	if input.AccountID != nil && *input.AccountID == 0 {
		fields["account_id"] = "must be a positive integer"
	}
	return fields
}

func (input subscriptionInput) apply(subscription *models.Subscription) {
	subscription.AccountID = input.AccountID
	subscription.Name = strings.TrimSpace(input.Name)
	subscription.Merchant = strings.TrimSpace(input.Merchant)
	subscription.Category = strings.TrimSpace(input.Category)
	subscription.Amount = input.Amount
	subscription.Currency = normalizeSubscriptionCurrency(input.Currency)
	if subscription.Currency == "" {
		subscription.Currency = "INR"
	}
	subscription.BillingInterval = normalizeSubscriptionInterval(input.BillingInterval)
	if subscription.BillingInterval == "" {
		subscription.BillingInterval = subscriptionIntervalMonthly
	}
	subscription.NextDueDate = strings.TrimSpace(input.NextDueDate)
	subscription.LastChargedDate = strings.TrimSpace(input.LastChargedDate)
	subscription.Status = normalizeSubscriptionStatus(input.Status)
	if subscription.Status == "" {
		subscription.Status = subscriptionStatusActive
	}
	subscription.ReminderDays = defaultSubscriptionReminderDays
	if input.ReminderDays != nil {
		subscription.ReminderDays = *input.ReminderDays
	}
	subscription.CancelBeforeDue = input.CancelBeforeDue
	subscription.Notes = strings.TrimSpace(input.Notes)
}

func normalizeSubscriptionInterval(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", subscriptionIntervalMonthly:
		return subscriptionIntervalMonthly
	case subscriptionIntervalDaily:
		return subscriptionIntervalDaily
	case subscriptionIntervalBusinessDaily:
		return subscriptionIntervalBusinessDaily
	case subscriptionIntervalWeekly:
		return subscriptionIntervalWeekly
	case subscriptionIntervalBiweekly:
		return subscriptionIntervalBiweekly
	case subscriptionIntervalQuarterly:
		return subscriptionIntervalQuarterly
	case subscriptionIntervalYearly:
		return subscriptionIntervalYearly
	default:
		return ""
	}
}

func normalizeSubscriptionStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", subscriptionStatusActive:
		return subscriptionStatusActive
	case subscriptionStatusPaused:
		return subscriptionStatusPaused
	case subscriptionStatusCancelled:
		return subscriptionStatusCancelled
	default:
		return ""
	}
}

func normalizeSubscriptionCurrency(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func parseAPIDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if len(value) >= len("2006-01-02") {
		value = value[:len("2006-01-02")]
	}
	return time.Parse("2006-01-02", value)
}

func parseStrictAPIDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}
