package http

import (
	"bytes"
	"encoding/json"
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
	AccountID   *uint              `json:"account_id"`
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
	AccountID   optionalAccountID   `json:"account_id"`
}

type optionalAccountID struct {
	Set   bool
	Value *uint
}

func (field *optionalAccountID) UnmarshalJSON(data []byte) error {
	field.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Value = nil
		return nil
	}

	var value uint
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	field.Value = &value
	return nil
}

func validateEntryValues(amount models.Money, title, entryType, currency, source, mode, category, date string) map[string]string {
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
	if normalized := strings.ToUpper(strings.TrimSpace(currency)); normalized != "" && normalized != "INR" {
		fields["currency"] = "must be INR"
	}
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "manual", "text", "voice":
	default:
		fields["source"] = "must be manual, text, or voice"
	}
	if strings.TrimSpace(title) == "" {
		fields["title"] = "is required"
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "cash", "upi", "credit card", "wallets":
	default:
		fields["mode"] = "must be Cash, UPI, Credit Card, or Wallets"
	}
	if strings.TrimSpace(category) == "" {
		fields["category"] = "is required"
	}
	return fields
}

func (input entryInput) validate() map[string]string {
	return validateEntryValues(
		input.Amount, input.Title, input.Type, input.Currency,
		input.Source, input.Mode, input.Category, input.Date,
	)
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
