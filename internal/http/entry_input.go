package http

import (
	"strings"
	"time"

	"finance-parser-go/internal/models"
)

type entryInput struct {
	Title       string             `json:"title"`
	Type        string             `json:"type"`
	Amount      float64            `json:"amount"`
	Currency    string             `json:"currency"`
	Mode        string             `json:"mode"`
	CardNetwork string             `json:"card_network"`
	Category    string             `json:"category"`
	Merchant    string             `json:"merchant"`
	PurposeType string             `json:"purpose_type"`
	Tag         string             `json:"tag"`
	Tags        models.StringArray `json:"tags"`
	Notes       string             `json:"notes"`
	Date        string             `json:"date"`
	Time        string             `json:"time"`
	SourceText  string             `json:"source_text"`
	Attachment  string             `json:"attachment"`
	AccountID   uint               `json:"account_id"`
}

type updateEntryInput struct {
	Title       *string             `json:"title"`
	Type        *string             `json:"type"`
	Amount      *float64            `json:"amount"`
	Currency    *string             `json:"currency"`
	Mode        *string             `json:"mode"`
	CardNetwork *string             `json:"card_network"`
	Category    *string             `json:"category"`
	Merchant    *string             `json:"merchant"`
	PurposeType *string             `json:"purpose_type"`
	Tag         *string             `json:"tag"`
	Tags        *models.StringArray `json:"tags"`
	Notes       *string             `json:"notes"`
	Date        *string             `json:"date"`
	Time        *string             `json:"time"`
	SourceText  *string             `json:"source_text"`
	Attachment  *string             `json:"attachment"`
	AccountID   *uint               `json:"account_id"`
}

func validateEntryValues(amount float64, entryType, date string, accountID uint) map[string]string {
	fields := map[string]string{}
	if amount <= 0 {
		fields["amount"] = "must be greater than zero"
	}
	switch strings.ToLower(entryType) {
	case "expense", "income":
	default:
		fields["type"] = "must be expense or income"
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		fields["date"] = "must use YYYY-MM-DD"
	}
	if accountID == 0 {
		fields["account_id"] = "is required"
	}
	return fields
}

func (input entryInput) validate() map[string]string {
	return validateEntryValues(input.Amount, input.Type, input.Date, input.AccountID)
}

func (input entryInput) toModel(userID uint) models.Entry {
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "INR"
	}
	accountID := input.AccountID
	return models.Entry{
		Title: input.Title, Type: strings.ToLower(input.Type), Amount: input.Amount,
		Currency: currency, Mode: input.Mode, CardNetwork: input.CardNetwork,
		Category: input.Category, Merchant: input.Merchant, PurposeType: input.PurposeType,
		Tag: input.Tag, Tags: input.Tags, Notes: input.Notes, Date: input.Date,
		Time: input.Time, SourceText: input.SourceText, Attachment: input.Attachment,
		AccountID: &accountID, UserID: userID,
	}
}
