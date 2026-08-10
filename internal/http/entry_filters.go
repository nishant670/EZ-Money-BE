package http

import (
	"encoding/json"
	"strconv"
	"strings"
	timepkg "time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

func filteredEntriesQuery(userID uint, c *gin.Context) (*gorm.DB, map[string]string) {
	fields := map[string]string{}
	query := database.DB.Model(&models.Entry{}).Where("entries.user_id = ?", userID)

	if t := strings.TrimSpace(c.Query("type")); t != "" && !strings.EqualFold(t, "all") {
		if !strings.EqualFold(t, "expense") && !strings.EqualFold(t, "income") {
			fields["type"] = "must be expense or income"
		} else {
			query = query.Where("LOWER(entries.type) = LOWER(?)", t)
		}
	}

	if cat := strings.TrimSpace(c.Query("category")); cat != "" {
		query = query.Where("LOWER(entries.category) = LOWER(?)", cat)
	}

	if mode := strings.TrimSpace(c.Query("mode")); mode != "" {
		switch strings.ToLower(mode) {
		case "cash", "upi", "credit card", "wallets":
			query = query.Where("LOWER(entries.mode) = LOWER(?)", mode)
		default:
			fields["mode"] = "is invalid"
		}
	}
	if accountID := strings.TrimSpace(c.Query("account_id")); accountID != "" {
		parsed, err := strconv.ParseUint(accountID, 10, 32)
		if err != nil || parsed == 0 {
			fields["account_id"] = "must be a positive integer"
		} else {
			query = query.Where("entries.account_id = ?", parsed)
		}
	}

	var minAmount, maxAmount *models.Money
	if minStr := c.Query("min_amount"); minStr != "" {
		min, err := models.ParseMoney(minStr)
		if err != nil {
			fields["min_amount"] = err.Error()
		} else {
			minAmount = &min
			query = query.Where("entries.amount >= ?", min)
		}
	}

	if maxStr := c.Query("max_amount"); maxStr != "" {
		max, err := models.ParseMoney(maxStr)
		if err != nil {
			fields["max_amount"] = err.Error()
		} else {
			maxAmount = &max
			query = query.Where("entries.amount <= ?", max)
		}
	}
	if minAmount != nil && maxAmount != nil && *minAmount > *maxAmount {
		fields["max_amount"] = "must be greater than or equal to min_amount"
	}

	startDate := c.Query("start_date")
	if startDate != "" {
		if _, err := timepkg.Parse("2006-01-02", startDate); err != nil {
			fields["start_date"] = "must use YYYY-MM-DD"
		} else {
			query = query.Where("entries.date >= ?", startDate)
		}
	}

	endDate := c.Query("end_date")
	if endDate != "" {
		if _, err := timepkg.Parse("2006-01-02", endDate); err != nil {
			fields["end_date"] = "must use YYYY-MM-DD"
		} else {
			query = query.Where("entries.date <= ?", endDate)
		}
	}
	if startDate != "" && endDate != "" && startDate > endDate {
		fields["end_date"] = "must be on or after start_date"
	}

	if tag := strings.TrimSpace(c.Query("tag")); tag != "" {
		query = applyEntryTagFilter(query, tag)
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		if len(search) > 200 {
			fields["q"] = "must not exceed 200 characters"
		} else {
			pattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(
				"LOWER(entries.title) LIKE ? OR LOWER(entries.merchant) LIKE ? OR LOWER(entries.notes) LIKE ?",
				pattern, pattern, pattern,
			)
		}
	}

	if len(fields) > 0 {
		return query, fields
	}
	return query, nil
}

func applyEntryTagFilter(query *gorm.DB, tag string) *gorm.DB {
	tagFilter, err := json.Marshal([]string{tag})
	if err != nil {
		return query
	}
	if database.DB.Dialector.Name() == "sqlite" {
		return query.Where("entries.tags LIKE ?", "%\""+strings.ReplaceAll(tag, `"`, `\"`)+"\"%")
	}
	return query.Where("entries.tags @> ?", string(tagFilter))
}
