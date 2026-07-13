package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	timepkg "time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/xeipuuv/gojsonschema"
	"gorm.io/gorm"

	"finance-parser-go/internal/ai"
	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

type Server struct {
	cfg       *config.Config
	validator *gojsonschema.Schema
	parser    ai.Parser
}

func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors(cfg))
	r.Use(logging())

	if cfg.AuthBearer != "" {
		r.Use(func(c *gin.Context) {
			if skipsStaticBearer(c.Request.URL.Path) {
				log.Printf("[DEBUG] Auth Skip: %s", c.Request.URL.Path)
				c.Next()
				return
			}
			if c.GetHeader("Authorization") != "Bearer "+cfg.AuthBearer {
				log.Printf("[ERROR] Auth Fail: %s (Header: %s)", c.Request.URL.Path, c.GetHeader("Authorization"))
				c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
				return
			}
			c.Next()
		})
	}

	loader := gojsonschema.NewReferenceLoader("file://./schemas/expense_entry.schema.json")
	schema, err := gojsonschema.NewSchema(loader)
	if err != nil {
		panic(err)
	}

	openai := ai.NewOpenAIClient(cfg)

	s := &Server{cfg: cfg, validator: schema, parser: openai}
	// Auth
	authLimited := r.Group("/v1/auth")
	authLimited.Use(jsonRequestLimits(cfg), rateLimit(cfg, "auth"))
	{
		authLimited.POST("/guest", s.authGuest)
		authLimited.POST("/identify", s.authIdentify)
		authLimited.POST("/otp/send", s.authOtpSend)
		authLimited.POST("/otp/verify", s.authOtpVerify)
		authLimited.POST("/register", s.authRegister)
		authLimited.POST("/login", s.authLogin)
		authLimited.POST("/pin/reset", s.authPinReset)
	}

	// Protected Routes (User Token)
	authorized := r.Group("/v1")
	authorized.Use(AuthMiddleware())
	{
		authorized.POST("/parse", uploadRequestLimits(cfg), rateLimit(cfg, "ai"), s.handleParse)
		authorized.POST("/entries", s.saveEntry)
		authorized.GET("/entries", s.listEntries)
		authorized.GET("/entries/:id", s.getEntry)
		authorized.PUT("/entries/:id", s.updateEntry)
		authorized.DELETE("/entries/:id", s.deleteEntry)
		authorized.GET("/quick-prompts", s.listQuickPrompts)
		authorized.POST("/quick-prompts", s.saveQuickPrompt)
		authorized.PUT("/quick-prompts/:id", s.updateQuickPrompt)
		authorized.DELETE("/quick-prompts/:id", s.deleteQuickPrompt)
		authorized.PUT("/user", s.updateProfile)
		authorized.DELETE("/user", s.deleteUser)
		authorized.POST("/upload", uploadRequestLimits(cfg), s.handleUpload)

		// Accounts
		authorized.POST("/accounts", s.saveAccount)
		authorized.GET("/accounts", s.listAccounts)
		authorized.PUT("/accounts/:id", s.updateAccount)
		authorized.DELETE("/accounts/:id", s.deleteAccount)

		// Insights
		authorized.GET("/dashboard", s.getDashboard)
		authorized.GET("/insights", s.getInsights)

		// Notifications
		authorized.GET("/notifications", s.listNotifications)
		authorized.GET("/notifications/unread-count", s.getUnreadNotificationCount)
		authorized.PATCH("/notifications/read-all", s.markAllNotificationsRead)
		authorized.PATCH("/notifications/:id/read", s.markNotificationRead)
		authorized.DELETE("/notifications/:id", s.deleteNotification)

		// Split ledger
		authorized.POST("/split/friends", s.createSplitFriend)
		authorized.GET("/split/friends", s.listSplitFriends)
		authorized.PUT("/split/friends/:id", s.updateSplitFriend)
		authorized.DELETE("/split/friends/:id", s.archiveSplitFriend)
		authorized.POST("/split/groups", s.createSplitGroup)
		authorized.GET("/split/groups", s.listSplitGroups)
		authorized.PUT("/split/groups/:id", s.updateSplitGroup)
		authorized.DELETE("/split/groups/:id", s.archiveSplitGroup)
		authorized.POST("/split/bills", s.createSplitBill)
		authorized.GET("/split/bills", s.listSplitBills)
		authorized.POST("/split/settlements", s.createSplitSettlement)
		authorized.GET("/split/settlements", s.listSplitSettlements)
		authorized.GET("/split/balances", s.listSplitBalances)

		// Financial tools
		authorized.POST("/tools/emi/calculate", s.calculateEMI)
	}

	r.Static("/uploads", "./uploads")
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	return r
}

func skipsStaticBearer(path string) bool {
	return path == "/health" ||
		strings.HasPrefix(path, "/v1/auth/") ||
		strings.HasPrefix(path, "/v1/entries") ||
		strings.HasPrefix(path, "/v1/quick-prompts") ||
		strings.HasPrefix(path, "/v1/user") ||
		strings.HasPrefix(path, "/v1/insights") ||
		strings.HasPrefix(path, "/v1/dashboard") ||
		strings.HasPrefix(path, "/v1/accounts") ||
		strings.HasPrefix(path, "/v1/notifications") ||
		strings.HasPrefix(path, "/v1/split") ||
		strings.HasPrefix(path, "/v1/tools") ||
		strings.HasPrefix(path, "/v1/parse")
}

func (s *Server) handleParse(c *gin.Context) {
	// ... (no changes to handleParse logic yet)
	ctx, cancel := context.WithTimeout(c.Request.Context(), timepkg.Duration(s.cfg.ReqTimeoutSec)*timepkg.Second)
	defer cancel()

	if err := c.Request.ParseMultipartForm(s.cfg.MaxUploadMB * 1024 * 1024); err != nil && requestBodyTooLarge(err) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request_body_too_large"})
		return
	}

	tz := c.PostForm("tz")
	if tz == "" {
		tz = s.cfg.TZDefault
	}

	var transcript string
	file, header, err := c.Request.FormFile("audio")
	if err == nil {
		defer file.Close()
		if header.Size > s.cfg.MaxUploadMB*1024*1024 {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file_too_large"})
			return
		}
		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, file); err != nil {
			if requestBodyTooLarge(err) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request_body_too_large"})
				return
			}
			c.JSON(400, gin.H{"error": "failed to read file"})
			return
		}
		if t, err := s.parser.Transcribe(ctx, header.Filename, buf.Bytes()); err == nil {
			transcript = t
		} else {
			log.Printf("stt error: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "transcription_failed"})
			return
		}
	}

	if transcript == "" {
		transcript = c.PostForm("hint_text")
	}
	if strings.TrimSpace(transcript) == "" {
		c.JSON(400, gin.H{"error": "no audio or hint_text provided"})
		return
	}
	if s.cfg.MaxTranscriptChars > 0 && utf8.RuneCountInString(transcript) > s.cfg.MaxTranscriptChars {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error":          "transcript_too_long",
			"max_characters": s.cfg.MaxTranscriptChars,
		})
		return
	}

	parsed, err := s.parser.ParseText(ctx, transcript, tz)
	if err != nil {
		c.JSON(422, gin.H{"error": "could_not_parse", "transcript": transcript})
		return
	}

	var parsedObj map[string]any
	if err := json.Unmarshal(parsed, &parsedObj); err != nil {
		c.JSON(500, gin.H{"error": "invalid_parse_response"})
		return
	}

	normalizeParsedDraft(parsedObj, transcript)
	parsed, err = json.Marshal(parsedObj)
	if err != nil {
		c.JSON(500, gin.H{"error": "serialization_failed"})
		return
	}

	res, err := s.validator.Validate(gojsonschema.NewBytesLoader(parsed))
	if err != nil {
		c.JSON(500, gin.H{"error": "validation_failed"})
		return
	}
	if !res.Valid() {
		d := []string{}
		for _, e := range res.Errors() {
			d = append(d, e.String())
		}
		c.JSON(422, gin.H{"error": "schema_invalid", "details": d, "transcript": transcript})
		return
	}
	c.Data(200, "application/json", parsed)
}

func (s *Server) saveEntry(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	userID := val.(uint)

	var input entryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(422, gin.H{"error": "invalid_entry", "fields": fields})
		return
	}
	if input.AccountID != nil {
		if ok, err := userOwnsAccount(userID, *input.AccountID); err != nil {
			c.JSON(500, gin.H{"error": "account_lookup_failed"})
			return
		} else if !ok {
			c.JSON(422, gin.H{"error": "invalid_entry", "fields": gin.H{"account_id": "must belong to the current user"}})
			return
		}
	}
	if fields, err := validateEntrySplitReferences(userID, input.Split); err != nil {
		c.JSON(500, gin.H{"error": "split_lookup_failed"})
		return
	} else if len(fields) > 0 {
		c.JSON(422, gin.H{"error": "invalid_entry", "fields": fields})
		return
	}

	idempotencyKey, idempotencyFields := parseIdempotencyKey(c.GetHeader("Idempotency-Key"))
	if len(idempotencyFields) > 0 {
		c.JSON(422, gin.H{"error": "invalid_entry", "fields": idempotencyFields})
		return
	}
	if idempotencyKey != "" {
		var existing models.Entry
		if err := database.DB.Preload("Account").
			Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).
			First(&existing).Error; err == nil {
			c.Header("Idempotency-Replayed", "true")
			c.JSON(200, existing)
			return
		} else if err != gorm.ErrRecordNotFound {
			c.JSON(500, gin.H{"error": "idempotency_lookup_failed"})
			return
		}
	}

	entry := input.toModel(userID)
	if idempotencyKey != "" {
		entry.IdempotencyKey = &idempotencyKey
	}
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		return createEntrySplitBill(tx, userID, entry, input.Split)
	}); err != nil {
		if idempotencyKey != "" {
			var existing models.Entry
			if lookupErr := database.DB.Preload("Account").
				Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).
				First(&existing).Error; lookupErr == nil {
				c.Header("Idempotency-Replayed", "true")
				c.JSON(200, existing)
				return
			}
		}
		c.JSON(500, gin.H{"error": "failed_create_entry"})
		return
	}
	_ = database.DB.Preload("Account").First(&entry, entry.ID).Error
	_ = createEntryNotification(userID, "transaction.created", "Transaction added", entryNotificationBody("Added", entry), entry.ID)

	c.JSON(201, entry)
}

func (s *Server) listEntries(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	userID := val.(uint)

	page, pageSize, fields := parseEntryPagination(c.Query("page"), c.Query("page_size"))
	if len(fields) > 0 {
		c.JSON(422, gin.H{"error": "invalid_filters", "fields": fields})
		return
	}

	var entries []models.Entry
	query := database.DB.Model(&models.Entry{}).Where("user_id = ?", userID)

	if t := strings.TrimSpace(c.Query("type")); t != "" && !strings.EqualFold(t, "all") {
		if !strings.EqualFold(t, "expense") && !strings.EqualFold(t, "income") {
			c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"type": "must be expense or income"}})
			return
		}
		query = query.Where("LOWER(type) = LOWER(?)", t)
	}

	if cat := strings.TrimSpace(c.Query("category")); cat != "" {
		query = query.Where("LOWER(category) = LOWER(?)", cat)
	}

	if mode := strings.TrimSpace(c.Query("mode")); mode != "" {
		switch strings.ToLower(mode) {
		case "cash", "upi", "credit card", "wallets":
		default:
			c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"mode": "is invalid"}})
			return
		}
		query = query.Where("LOWER(mode) = LOWER(?)", mode)
	}
	if accountID := strings.TrimSpace(c.Query("account_id")); accountID != "" {
		parsed, err := strconv.ParseUint(accountID, 10, 32)
		if err != nil || parsed == 0 {
			c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"account_id": "must be a positive integer"}})
			return
		}
		query = query.Where("account_id = ?", parsed)
	}

	var minAmount, maxAmount *models.Money
	if minStr := c.Query("min_amount"); minStr != "" {
		min, err := models.ParseMoney(minStr)
		if err != nil {
			c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"min_amount": err.Error()}})
			return
		}
		minAmount = &min
		query = query.Where("amount >= ?", min)
	}

	if maxStr := c.Query("max_amount"); maxStr != "" {
		max, err := models.ParseMoney(maxStr)
		if err != nil {
			c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"max_amount": err.Error()}})
			return
		}
		maxAmount = &max
		query = query.Where("amount <= ?", max)
	}
	if minAmount != nil && maxAmount != nil && *minAmount > *maxAmount {
		c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"max_amount": "must be greater than or equal to min_amount"}})
		return
	}

	startDate := c.Query("start_date")
	if startDate != "" {
		if _, err := timepkg.Parse("2006-01-02", startDate); err != nil {
			c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"start_date": "must use YYYY-MM-DD"}})
			return
		}
		query = query.Where("date >= ?", startDate)
	}

	endDate := c.Query("end_date")
	if endDate != "" {
		if _, err := timepkg.Parse("2006-01-02", endDate); err != nil {
			c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"end_date": "must use YYYY-MM-DD"}})
			return
		}
		query = query.Where("date <= ?", endDate)
	}
	if startDate != "" && endDate != "" && startDate > endDate {
		c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"end_date": "must be on or after start_date"}})
		return
	}

	if tag := strings.TrimSpace(c.Query("tag")); tag != "" {
		if tagFilter, err := json.Marshal([]string{tag}); err == nil {
			query = query.Where("tags @> ?", string(tagFilter))
		}
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		if len(search) > 200 {
			c.JSON(422, gin.H{"error": "invalid_filters", "fields": gin.H{"q": "must not exceed 200 characters"}})
			return
		}
		pattern := "%" + search + "%"
		query = query.Where(
			"title ILIKE ? OR merchant ILIKE ? OR notes ILIKE ?",
			pattern, pattern, pattern,
		)
	}

	listQuery := query.Session(&gorm.Session{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_count_entries"})
		return
	}

	if err := listQuery.Preload("Account").
		Order("date desc, created_at desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&entries).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_list_entries"})
		return
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	c.JSON(200, gin.H{
		"entries": entries, "page": page, "page_size": pageSize,
		"total": total, "total_pages": totalPages,
	})
}

func (s *Server) getEntry(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	var entry models.Entry
	if err := ownedEntries(database.DB, userID).Preload("Account").Where("id = ?", id).First(&entry).Error; err != nil {
		c.JSON(404, gin.H{"error": "entry not found"})
		return
	}

	c.JSON(200, entry)
}

func (s *Server) updateEntry(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	var entry models.Entry
	if err := ownedEntries(database.DB, userID).Where("id = ?", id).First(&entry).Error; err != nil {
		c.JSON(404, gin.H{"error": "entry not found"})
		return
	}

	var input updateEntryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "invalid_json"})
		return
	}

	amount, title, entryType := entry.Amount, entry.Title, entry.Type
	currency, source, mode, category, date := entry.Currency, entry.Source, entry.Mode, entry.Category, entry.Date
	if input.Amount != nil {
		amount = *input.Amount
	}
	if input.Type != nil {
		entryType = *input.Type
	}
	if input.Title != nil {
		title = *input.Title
	}
	if input.Currency != nil {
		currency = *input.Currency
	}
	if input.Source != nil {
		source = *input.Source
	}
	if input.Mode != nil {
		mode = *input.Mode
	}
	if input.Category != nil {
		category = *input.Category
	}
	if input.Date != nil {
		date = *input.Date
	}
	if fields := validateEntryValues(amount, title, entryType, currency, source, mode, category, date); len(fields) > 0 {
		c.JSON(422, gin.H{"error": "invalid_entry", "fields": fields})
		return
	}
	if !input.AccountID.Set {
		c.JSON(422, gin.H{"error": "invalid_entry", "fields": gin.H{"account_id": "is required"}})
		return
	}
	if input.AccountID.Value == nil {
		c.JSON(422, gin.H{"error": "invalid_entry", "fields": gin.H{"account_id": "is required"}})
		return
	}
	if *input.AccountID.Value == 0 {
		c.JSON(422, gin.H{"error": "invalid_entry", "fields": gin.H{"account_id": "must be a positive integer"}})
		return
	}
	if ok, err := userOwnsAccount(userID, *input.AccountID.Value); err != nil {
		c.JSON(500, gin.H{"error": "account_lookup_failed"})
		return
	} else if !ok {
		c.JSON(422, gin.H{"error": "invalid_entry", "fields": gin.H{"account_id": "must belong to the current user"}})
		return
	}
	if input.Title != nil {
		entry.Title = *input.Title
	}
	if input.Amount != nil {
		entry.Amount = *input.Amount
	}
	if input.Type != nil {
		entry.Type = strings.ToLower(*input.Type)
	}
	if input.Currency != nil {
		entry.Currency = strings.ToUpper(strings.TrimSpace(*input.Currency))
	}
	if input.Source != nil {
		entry.Source = strings.ToLower(strings.TrimSpace(*input.Source))
	}
	if input.Mode != nil {
		entry.Mode = *input.Mode
	}
	if input.CardNetwork != nil {
		entry.CardNetwork = *input.CardNetwork
	}
	if input.Category != nil {
		entry.Category = *input.Category
	}
	if input.Notes != nil {
		entry.Notes = *input.Notes
	}
	if input.Merchant != nil {
		entry.Merchant = *input.Merchant
	}
	if input.PurposeType != nil {
		entry.PurposeType = *input.PurposeType
	}
	if input.Date != nil {
		entry.Date = *input.Date
	}
	if input.Time != nil {
		entry.Time = *input.Time
	}
	if input.Tag != nil {
		entry.Tag = *input.Tag
	}
	if input.Tags != nil {
		entry.Tags = *input.Tags
	}
	if input.SourceText != nil {
		entry.SourceText = *input.SourceText
	}
	if input.Attachment != nil {
		entry.Attachment = *input.Attachment
	}
	if input.AccountID.Set {
		entry.AccountID = input.AccountID.Value
	}

	if err := database.DB.Save(&entry).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_update_entry"})
		return
	}
	_ = database.DB.Preload("Account").First(&entry, entry.ID).Error
	_ = createEntryNotification(userID, "transaction.updated", "Transaction updated", entryNotificationBody("Updated", entry), entry.ID)

	c.JSON(200, entry)
}

func userOwnsAccount(userID, accountID uint) (bool, error) {
	var count int64
	err := database.DB.Model(&models.Account{}).
		Where("id = ? AND user_id = ?", accountID, userID).
		Count(&count).Error
	return count == 1, err
}

func ownedEntries(db *gorm.DB, userID uint) *gorm.DB {
	return db.Where("user_id = ?", userID)
}

func (s *Server) deleteEntry(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	var entry models.Entry
	if err := ownedEntries(database.DB, userID).Where("id = ?", id).First(&entry).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, gin.H{"error": "entry not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	result := ownedEntries(database.DB, userID).Where("id = ?", id).Delete(&models.Entry{})
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "entry not found"})
		return
	}
	_ = createNotification(userID, "transaction.deleted", "Transaction deleted", entryNotificationBody("Deleted", entry), "")

	c.JSON(200, gin.H{"message": "entry deleted"})
}

func (s *Server) listQuickPrompts(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var prompts []models.QuickPrompt
	if err := database.DB.Where("user_id = ?", userID).Find(&prompts).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Seed default prompts if none exist
	if len(prompts) == 0 {
		defaults := []models.QuickPrompt{
			{UserID: userID, Title: "Morning Coffee", Amount: 150, Mode: "Cash", Category: "Food & Drinks", Icon: "coffee-outline"},
			{UserID: userID, Title: "Metro Recharge", Amount: 500, Mode: "UPI", Category: "Travel", Icon: "train"},
			{UserID: userID, Title: "Car Fuel", Amount: 3000, Mode: "Credit Card", Category: "Transport", Icon: "gas-station-outline"},
		}
		for _, p := range defaults {
			database.DB.Create(&p)
		}
		// Fetch again to get IDs and updated list
		database.DB.Where("user_id = ?", userID).Find(&prompts)
	}

	c.JSON(200, prompts)
}

func (s *Server) saveQuickPrompt(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var prompt models.QuickPrompt
	if err := c.BindJSON(&prompt); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	prompt.UserID = userID
	if err := database.DB.Create(&prompt).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, prompt)
}

func (s *Server) updateQuickPrompt(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	var prompt models.QuickPrompt
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&prompt).Error; err != nil {
		c.JSON(404, gin.H{"error": "prompt not found"})
		return
	}

	if err := c.BindJSON(&prompt); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	prompt.ID = uint(id)
	prompt.UserID = userID
	if err := database.DB.Save(&prompt).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, prompt)
}

func (s *Server) deleteQuickPrompt(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.QuickPrompt{}).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "prompt deleted"})
}

func (s *Server) updateProfile(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	userID := val.(uint)

	var payload struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Phone      string `json:"phone"`
		ClaimToken string `json:"claim_token"`
	}

	if err := c.BindJSON(&payload); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(payload.Username) == "" {
		c.JSON(400, gin.H{"error": "Username cannot be empty"})
		return
	}

	// 1. Check Username Uniqueness
	var existingUser models.User
	if err := database.DB.Where("username = ? AND id != ?", payload.Username, userID).First(&existingUser).Error; err == nil {
		c.JSON(409, gin.H{"error": "Username is already taken"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	emailChanged := payload.Email != "" && (user.Email == nil || *user.Email != payload.Email)
	phoneChanged := payload.Phone != "" && (user.Phone == nil || *user.Phone != payload.Phone)
	if emailChanged && phoneChanged {
		c.JSON(403, gin.H{"error": "Only one verified contact field can be updated at a time."})
		return
	}

	if emailChanged || phoneChanged {
		claim, err := consumeClaimToken(payload.ClaimToken)
		if err != nil {
			c.JSON(403, gin.H{"error": "Contact verification required. Please verify OTP."})
			return
		}
		if emailChanged {
			_, normalizedEmail, err := normalizeIdentifier(payload.Email)
			if err != nil || claim.IdentifierType != "email" || claim.Identifier != normalizedEmail {
				c.JSON(403, gin.H{"error": "Email verification required. Please verify OTP."})
				return
			}
			user.Email = &normalizedEmail
		}
		if phoneChanged {
			_, normalizedPhone, err := normalizeIdentifier(payload.Phone)
			if err != nil || claim.IdentifierType != "phone" || claim.Identifier != normalizedPhone {
				c.JSON(403, gin.H{"error": "Phone verification required. Please verify OTP."})
				return
			}
			user.Phone = &normalizedPhone
		}
	} else if payload.Email == "" {
		// Optional: Allow clearing email? Or just ignore if empty?
		// For now assuming empty string in payload means 'no change' or 'clear' managed by FE logic.
		// If explicit clear is needed, logic might differ. Assuming update sends current value if unchanged.
	}

	user.Username = payload.Username

	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update profile."})
		return
	}

	// Re-serialize user to ensure clean JSON response
	c.JSON(200, gin.H{"user": user})
}

func (s *Server) deleteUser(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	user := models.User{ID: userID}
	if value, exists := c.Get("user"); exists {
		if currentUser, ok := value.(*models.User); ok && currentUser != nil {
			user = *currentUser
		}
	}
	if user.ID == 0 {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	if user.Email == nil && user.Phone == nil {
		if err := database.DB.First(&user, userID).Error; err != nil {
			c.JSON(404, gin.H{"error": "user_not_found"})
			return
		}
	}

	attachments, err := deleteUserData(database.DB, user)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed_delete_user"})
		return
	}
	deleteLocalUploadFiles(attachments)
	c.JSON(200, gin.H{"message": "account deleted"})
}

func deleteUserData(db *gorm.DB, user models.User) ([]string, error) {
	if user.ID == 0 {
		return nil, errors.New("user_id_required")
	}

	var attachments []string
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Entry{}).
			Where("user_id = ? AND attachment <> ?", user.ID, "").
			Pluck("attachment", &attachments).Error; err != nil {
			return err
		}

		deletes := []struct {
			model any
			name  string
		}{
			{&models.SplitSettlement{}, "split settlements"},
			{&models.SplitParticipant{}, "split participants"},
			{&models.SplitBill{}, "split bills"},
			{&models.SplitGroupMember{}, "split group members"},
			{&models.SplitGroup{}, "split groups"},
			{&models.SplitFriend{}, "split friends"},
			{&models.Notification{}, "notifications"},
			{&models.QuickPrompt{}, "quick prompts"},
			{&models.Entry{}, "entries"},
			{&models.Account{}, "accounts"},
			{&models.AuthSession{}, "auth sessions"},
		}
		for _, item := range deletes {
			if err := tx.Where("user_id = ?", user.ID).Delete(item.model).Error; err != nil {
				return fmt.Errorf("delete %s: %w", item.name, err)
			}
		}

		for _, identifier := range verificationIdentifiersForUser(user) {
			if err := tx.Where("identifier_type = ? AND identifier = ?", identifier.identifierType, identifier.identifier).
				Delete(&models.AuthVerification{}).Error; err != nil {
				return fmt.Errorf("delete auth verifications: %w", err)
			}
		}

		if err := tx.Where("id = ?", user.ID).Delete(&models.User{}).Error; err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		return nil
	})
	return attachments, err
}

type verificationIdentifier struct {
	identifierType string
	identifier     string
}

func verificationIdentifiersForUser(user models.User) []verificationIdentifier {
	identifiers := make([]verificationIdentifier, 0, 2)
	if user.Email != nil {
		email := strings.TrimSpace(*user.Email)
		if email != "" {
			identifiers = append(identifiers, verificationIdentifier{identifierType: "email", identifier: strings.ToLower(email)})
		}
	}
	if user.Phone != nil {
		phone := strings.TrimSpace(*user.Phone)
		if phone != "" {
			identifiers = append(identifiers, verificationIdentifier{identifierType: "phone", identifier: phone})
		}
	}
	return identifiers
}

func localUploadPathFromAttachment(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
		value = parsed.Path
	}

	cleaned := filepath.Clean(strings.TrimPrefix(value, "/"))
	relative, err := filepath.Rel("uploads", cleaned)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return "", false
	}
	return filepath.Join("uploads", relative), true
}

func deleteLocalUploadFiles(attachments []string) {
	seen := make(map[string]struct{}, len(attachments))
	for _, attachment := range attachments {
		path, ok := localUploadPathFromAttachment(attachment)
		if !ok {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("failed_delete_upload file=%s err=%v", filepath.Base(path), err)
		}
	}
}

func cors(cfg *config.Config) gin.HandlerFunc {
	allowedOrigins := parseAllowedOrigins(cfg.AllowOrigins)
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if _, ok := allowedOrigins[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			} else if c.Request.Method == http.MethodOptions {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cors_origin_not_allowed"})
				return
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func parseAllowedOrigins(raw string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" || origin == "*" {
			continue
		}
		allowed[origin] = struct{}{}
	}
	return allowed
}

func logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := timepkg.Now()
		c.Next()
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), timepkg.Since(start))
	}
}

func loadLocationOrIndia(requested, fallback string) *timepkg.Location {
	if strings.TrimSpace(requested) == "" {
		requested = fallback
	}
	if strings.TrimSpace(requested) == "" {
		requested = "Asia/Kolkata"
	}
	loc, err := timepkg.LoadLocation(requested)
	if err == nil {
		return loc
	}
	loc, err = timepkg.LoadLocation("Asia/Kolkata")
	if err == nil {
		return loc
	}
	return timepkg.FixedZone("IST", 5*3600+1800)
}

func (s *Server) handleUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		if requestBodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": "no file provided"})
		return
	}
	if file.Size > s.cfg.MaxUploadMB*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file_too_large"})
		return
	}

	// Create unique filename
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), file.Filename)
	path := "uploads/" + filename

	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(500, gin.H{"error": "failed to save file"})
		return
	}

	// build full url using host header
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	fullURL := fmt.Sprintf("%s://%s/%s", scheme, c.Request.Host, path)

	c.JSON(200, gin.H{"url": fullURL})
}
func (s *Server) saveAccount(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var input accountInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(422, gin.H{"error": "invalid_account", "fields": fields})
		return
	}
	account := models.Account{UserID: userID}
	input.apply(&account)
	var accountCount int64
	if err := database.DB.Model(&models.Account{}).Where("user_id = ?", userID).Count(&accountCount).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_count_accounts"})
		return
	}
	if accountCount == 0 {
		account.IsDefault = true
	}
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if account.IsDefault {
			if err := tx.Model(&models.Account{}).Where("user_id = ?", userID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&account).Error
	}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, account)
}

func (s *Server) listAccounts(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var accounts []models.Account
	if err := database.DB.Where("user_id = ?", userID).Find(&accounts).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, accounts)
}

func (s *Server) updateAccount(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	var account models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&account).Error; err != nil {
		c.JSON(404, gin.H{"error": "account not found"})
		return
	}
	wasDefault := account.IsDefault
	var input accountInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(422, gin.H{"error": "invalid_account", "fields": fields})
		return
	}
	input.apply(&account)
	if wasDefault {
		account.IsDefault = true
	}
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if account.IsDefault {
			if err := tx.Model(&models.Account{}).
				Where("user_id = ? AND id <> ?", userID, id).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(&account).Error
	}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, account)
}

func (s *Server) deleteAccount(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	var account models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&account).Error; err != nil {
		c.JSON(404, gin.H{"error": "account not found"})
		return
	}
	var entryCount int64
	if err := database.DB.Model(&models.Entry{}).Where("account_id = ? AND user_id = ?", id, userID).Count(&entryCount).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_check_account_usage"})
		return
	}
	if entryCount > 0 {
		c.JSON(409, gin.H{"error": "account_in_use", "message": "Move or delete linked transactions before deleting this account."})
		return
	}
	var accountCount int64
	if err := database.DB.Model(&models.Account{}).Where("user_id = ?", userID).Count(&accountCount).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_count_accounts"})
		return
	}
	if accountCount <= 1 {
		c.JSON(409, gin.H{"error": "last_account", "message": "Create another account before deleting your only account."})
		return
	}
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&account).Error; err != nil {
			return err
		}
		if account.IsDefault {
			var replacement models.Account
			if err := tx.Where("user_id = ?", userID).Order("created_at asc").First(&replacement).Error; err != nil {
				return err
			}
			return tx.Model(&replacement).Update("is_default", true).Error
		}
		return nil
	}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "account deleted"})
}
