package http

import (
	"strings"
	"time"
)

var confirmableParseFields = []string{"title", "amount", "type", "mode", "category", "date"}

func normalizeParsedDraft(entry map[string]any, transcript string) {
	entry["stage"] = "draft"
	entry["source_text"] = transcript

	if currency, ok := entry["currency"].(string); !ok || strings.TrimSpace(currency) == "" {
		entry["currency"] = "INR"
	}

	needsConfirmation, _ := entry["needs_confirmation"].(map[string]any)
	if needsConfirmation == nil {
		needsConfirmation = map[string]any{}
	}

	missingSet := map[string]bool{}
	if values, ok := entry["missing_fields"].([]any); ok {
		for _, value := range values {
			if field, ok := value.(string); ok && field != "" {
				missingSet[field] = true
			}
		}
	}

	for _, field := range confirmableParseFields {
		if parseFieldMissing(field, entry[field]) {
			entry[field] = nil
			missingSet[field] = true
			needsConfirmation[field] = true
		}
	}

	missing := make([]string, 0, len(missingSet))
	for _, field := range confirmableParseFields {
		if missingSet[field] {
			missing = append(missing, field)
		}
	}
	entry["missing_fields"] = missing
	entry["needs_confirmation"] = needsConfirmation

	if _, ok := entry["confidence"].(map[string]any); !ok {
		entry["confidence"] = map[string]any{}
	}
	if _, ok := entry["clarifications"].([]any); !ok {
		entry["clarifications"] = []any{}
	}
}

func parseFieldMissing(field string, value any) bool {
	if value == nil {
		return true
	}
	if field == "amount" {
		amount, ok := value.(float64)
		return !ok || amount <= 0
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return true
	}
	if field == "date" {
		_, err := time.Parse("2006-01-02", text)
		return err != nil
	}
	return false
}
