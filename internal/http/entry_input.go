package http

import (
	"strings"
	"time"

	"finance-parser-go/internal/models"
)

type entryInput struct {
	Title       string             `json:"title"`
	Type        string             `json:"type"`
	Amount      models.Money       `json:"amount"`
	Currency    string             `json:"currency"`
	Source      string             `json:"source"`
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
	Amount      *models.Money       `json:"amount"`
	Currency    *string             `json:"currency"`
	Source      *string             `json:"source"`
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

func validateEntryValues(amount models.Money, entryType, currency, source, date string, accountID uint) map[string]string {
	fields := map[string]string{}
	if !amount.IsPositive() {
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
	if normalized := strings.ToUpper(strings.TrimSpace(currency)); normalized != "" && normalized != "INR" {
		fields["currency"] = "must be INR"
	}
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "manual", "text", "voice":
	default:
		fields["source"] = "must be manual, text, or voice"
	}
	return fields
}

func (input entryInput) validate() map[string]string {
	return validateEntryValues(input.Amount, input.Type, input.Currency, input.Source, input.Date, input.AccountID)
}

func (input entryInput) toModel(userID uint) models.Entry {
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "INR"
	}
	source := strings.ToLower(strings.TrimSpace(input.Source))
	if source == "" {
		source = "manual"
	}
	return models.Entry{
		Title: input.Title, Type: strings.ToLower(input.Type), Amount: input.Amount,
		Currency: currency, Source: source, Mode: input.Mode, CardNetwork: input.CardNetwork,
		Category: input.Category, Merchant: input.Merchant, PurposeType: input.PurposeType,
		Tag: input.Tag, Tags: input.Tags, Notes: input.Notes, Date: input.Date,
		Time: input.Time, SourceText: input.SourceText, Attachment: input.Attachment,
		AccountID: input.AccountID, UserID: userID,
	}
}
