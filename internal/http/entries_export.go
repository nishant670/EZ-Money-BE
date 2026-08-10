package http

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"finance-parser-go/internal/models"
)

const maxEntryExportRows = 10000

var entryExportCSVHeader = []string{
	"id",
	"date",
	"time",
	"type",
	"amount",
	"currency",
	"title",
	"category",
	"merchant",
	"account_id",
	"account_name",
	"mode",
	"source",
	"notes",
	"tags",
	"created_at",
	"updated_at",
}

func (s *Server) exportEntriesCSV(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := val.(uint)

	format := strings.TrimSpace(strings.ToLower(c.DefaultQuery("format", "csv")))
	if format != "csv" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  "invalid_filters",
			"fields": gin.H{"format": "must be csv"},
		})
		return
	}

	query, fields := filteredEntriesQuery(userID, c)
	if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_filters", "fields": fields})
		return
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_count_entries"})
		return
	}
	if total > maxEntryExportRows {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  "export_too_large",
			"fields": gin.H{"filters": fmt.Sprintf("export is limited to %d rows; narrow the date range or filters", maxEntryExportRows)},
		})
		return
	}

	var entries []models.Entry
	if err := query.Session(&gorm.Session{}).
		Preload("Account").
		Order("date desc, created_at desc").
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_export_entries"})
		return
	}

	body, err := entriesCSV(entries)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_export_entries"})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="finnri-entries.csv"`)
	c.String(http.StatusOK, body)
}

func entriesCSV(entries []models.Entry) (string, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(entryExportCSVHeader); err != nil {
		return "", err
	}
	for _, entry := range entries {
		accountID := ""
		accountName := ""
		if entry.AccountID != nil {
			accountID = fmt.Sprintf("%d", *entry.AccountID)
		}
		if entry.Account != nil {
			accountName = strings.TrimSpace(entry.Account.Name)
		}
		if err := writer.Write([]string{
			fmt.Sprintf("%d", entry.ID),
			entry.Date,
			entry.Time,
			entry.Type,
			entry.Amount.String(),
			entry.Currency,
			entry.Title,
			entry.Category,
			entry.Merchant,
			accountID,
			accountName,
			entry.Mode,
			entry.Source,
			entry.Notes,
			strings.Join(entry.Tags, "|"),
			entry.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			entry.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return buffer.String(), nil
}
