package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	timepkg "time"

	"github.com/gin-gonic/gin"
	"github.com/xeipuuv/gojsonschema"

	"finance-parser-go/internal/ai"
	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

type Server struct {
	cfg       *config.Config
	validator *gojsonschema.Schema
	openai    *ai.OpenAIClient
}

func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors(cfg))
	r.Use(logging())

	if cfg.AuthBearer != "" {
		r.Use(func(c *gin.Context) {
			if c.Request.URL.Path == "/health" ||
				strings.HasPrefix(c.Request.URL.Path, "/v1/auth/") ||
				strings.HasPrefix(c.Request.URL.Path, "/v1/entries") ||
				strings.HasPrefix(c.Request.URL.Path, "/v1/user") ||
				strings.HasPrefix(c.Request.URL.Path, "/v1/parse") {
				c.Next()
				return
			}
			if c.GetHeader("Authorization") != "Bearer "+cfg.AuthBearer {
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

	s := &Server{cfg: cfg, validator: schema, openai: openai}
	// Auth
	r.POST("/v1/auth/guest", s.authGuest)
	r.POST("/v1/auth/identify", s.authIdentify)
	r.POST("/v1/auth/otp/send", s.authOtpSend)
	r.POST("/v1/auth/otp/verify", s.authOtpVerify)
	r.POST("/v1/auth/register", s.authRegister)
	r.POST("/v1/auth/login", s.authLogin)

	// Protected Routes (User Token)
	authorized := r.Group("/v1")
	authorized.Use(AuthMiddleware())
	{
		authorized.POST("/parse", s.handleParse)
		authorized.POST("/entries", s.saveEntry)
		authorized.GET("/entries", s.listEntries)
		authorized.GET("/entries/:id", s.getEntry)
		authorized.PUT("/entries/:id", s.updateEntry)
		authorized.DELETE("/entries/:id", s.deleteEntry)
		authorized.PUT("/user", s.updateProfile)
		authorized.POST("/upload", s.handleUpload)
	}

	r.Static("/uploads", "./uploads")
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	return r
}

func (s *Server) handleParse(c *gin.Context) {
	// ... (no changes to handleParse logic yet)
	ctx, cancel := context.WithTimeout(c.Request.Context(), timepkg.Duration(s.cfg.ReqTimeoutSec)*timepkg.Second)
	defer cancel()

	tz := c.PostForm("tz")
	if tz == "" {
		tz = s.cfg.TZDefault
	}

	var transcript string
	file, header, err := c.Request.FormFile("audio")
	if err == nil {
		defer file.Close()
		if header.Size > s.cfg.MaxUploadMB*1024*1024 {
			c.JSON(413, gin.H{"error": "file too large"})
			return
		}
		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, file); err != nil {
			c.JSON(400, gin.H{"error": "failed to read file"})
			return
		}
		if t, err := s.openai.Transcribe(ctx, header.Filename, buf.Bytes()); err == nil {
			transcript = t
		} else {
			log.Printf("stt error: %v", err)
		}
	}

	if transcript == "" {
		transcript = c.PostForm("hint_text")
	}
	if strings.TrimSpace(transcript) == "" {
		c.JSON(400, gin.H{"error": "no audio or hint_text provided"})
		return
	}

	parsed, err := s.openai.ParseText(ctx, transcript, tz)
	if err != nil {
		c.JSON(422, gin.H{"error": "could_not_parse", "transcript": transcript})
		return
	}

	var parsedObj map[string]any
	if err := json.Unmarshal(parsed, &parsedObj); err != nil {
		c.JSON(500, gin.H{"error": "invalid_parse_response"})
		return
	}

	if s.ensureDate(parsedObj, transcript, tz) {
		parsed, err = json.Marshal(parsedObj)
		if err != nil {
			c.JSON(500, gin.H{"error": "serialization_failed"})
			return
		}
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
	fmt.Print("parsedData", parsed)
	c.Data(200, "application/json", parsed)
}

func (s *Server) saveEntry(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	userID := val.(uint)

	var entry models.Entry

	if err := c.BindJSON(&entry); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	entry.UserID = userID

	if err := database.DB.Create(&entry).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, entry)
}

func (s *Server) listEntries(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	userID := val.(uint)

	var entries []models.Entry

	query := database.DB.Where("user_id = ?", userID).Order("date desc, created_at desc")

	if t := strings.TrimSpace(c.Query("type")); t != "" && t != "All" {
		query = query.Where("LOWER(type) = LOWER(?)", t)
	}

	if cat := strings.TrimSpace(c.Query("category")); cat != "" {
		query = query.Where("LOWER(category) = LOWER(?)", cat)
	}

	if mode := strings.TrimSpace(c.Query("mode")); mode != "" {
		query = query.Where("LOWER(mode) = LOWER(?)", mode)
	}

	if minStr := c.Query("min_amount"); minStr != "" {
		if min, err := strconv.ParseFloat(minStr, 64); err == nil {
			query = query.Where("amount >= ?", min)
		}
	}

	if maxStr := c.Query("max_amount"); maxStr != "" {
		if max, err := strconv.ParseFloat(maxStr, 64); err == nil {
			query = query.Where("amount <= ?", max)
		}
	}

	if start := c.Query("start_date"); start != "" {
		query = query.Where("date >= ?", start)
	}

	if end := c.Query("end_date"); end != "" {
		query = query.Where("date <= ?", end)
	}

	if tag := strings.TrimSpace(c.Query("tag")); tag != "" {
		if tagFilter, err := json.Marshal([]string{tag}); err == nil {
			query = query.Where("tags @> ?", string(tagFilter))
		}
	}

	log.Printf("[DEBUG] listEntries Filters | Type: %s | Cat: %s | Mode: %s | Min: %s | Max: %s | Start: %s | End: %s | Tag: %s",
		c.Query("type"), c.Query("category"), c.Query("mode"),
		c.Query("min_amount"), c.Query("max_amount"),
		c.Query("start_date"), c.Query("end_date"), c.Query("tag"),
	)

	if err := query.Find(&entries).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, entries)
	log.Printf("[DEBUG] listEntries: Found %d entries", len(entries))
}

func (s *Server) getEntry(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	var entry models.Entry
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&entry).Error; err != nil {
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
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&entry).Error; err != nil {
		c.JSON(404, gin.H{"error": "entry not found"})
		return
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Update fields if present in input
	if v, ok := input["title"].(string); ok {
		entry.Title = v
	}
	if v, ok := input["amount"].(float64); ok {
		entry.Amount = v
	}
	if v, ok := input["type"].(string); ok {
		entry.Type = strings.ToLower(v)
	}
	if v, ok := input["mode"].(string); ok {
		entry.Mode = v
	}
	if v, ok := input["category"].(string); ok {
		entry.Category = v
	}
	if v, ok := input["notes"].(string); ok {
		entry.Notes = v
	}
	if v, ok := input["merchant"].(string); ok {
		entry.Merchant = v
	}
	if v, ok := input["date"].(string); ok {
		entry.Date = v
	}
	if v, ok := input["time"].(string); ok {
		entry.Time = v
	}
	if v, ok := input["tag"].(string); ok {
		entry.Tag = v
	}
	if v, ok := input["attachment"].(string); ok {
		entry.Attachment = v
	}

	if err := database.DB.Save(&entry).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, entry)
}

func (s *Server) deleteEntry(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Entry{}).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "entry deleted"})
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

	// 2. Handle Contact Updates with Security (Claim Token)
	if payload.Email != "" && (user.Email == nil || *user.Email != payload.Email) {
		// Verify Claim Token for Email
		if !strings.HasPrefix(payload.ClaimToken, "claim_email:"+payload.Email) {
			c.JSON(403, gin.H{"error": "Email verification required. Please verify OTP."})
			return
		}
		user.Email = &payload.Email
	} else if payload.Email == "" {
		// Optional: Allow clearing email? Or just ignore if empty?
		// For now assuming empty string in payload means 'no change' or 'clear' managed by FE logic.
		// If explicit clear is needed, logic might differ. Assuming update sends current value if unchanged.
	}

	if payload.Phone != "" && (user.Phone == nil || *user.Phone != payload.Phone) {
		// Verify Claim Token for Phone
		if !strings.HasPrefix(payload.ClaimToken, "claim_phone:"+payload.Phone) {
			c.JSON(403, gin.H{"error": "Phone verification required. Please verify OTP."})
			return
		}
		user.Phone = &payload.Phone
	}

	user.Username = payload.Username

	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update profile."})
		return
	}

	// Re-serialize user to ensure clean JSON response
	c.JSON(200, gin.H{"user": user})
}

func cors(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", cfg.AllowOrigins)
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := timepkg.Now()
		c.Next()
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), timepkg.Since(start))
	}
}

func (s *Server) ensureDate(entry map[string]any, transcript, tz string) bool {
	loc := loadLocationOrIndia(tz, s.cfg.TZDefault)
	now := timepkg.Now().In(loc)

	var desired string

	dateStr, _ := entry["date"].(string)
	dateStr = strings.TrimSpace(dateStr)
	_, parsedErr := timepkg.Parse("2006-01-02", dateStr)
	needsDateConfirmation := needsDateConfirmation(entry)

	switch {
	case needsDateConfirmation:
		desired = now.Format("2006-01-02")
	case dateStr == "" || parsedErr != nil:
		desired = now.Format("2006-01-02")
	}

	if desired == "" {
		return false
	}

	entry["date"] = desired
	return true
}

func needsDateConfirmation(entry map[string]any) bool {
	raw, ok := entry["needs_confirmation"].(map[string]any)
	if !ok {
		return false
	}
	if val, ok := raw["date"].(bool); ok {
		return val
	}
	return false
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
		c.JSON(400, gin.H{"error": "no file provided"})
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
