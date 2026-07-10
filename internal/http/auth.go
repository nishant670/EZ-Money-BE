package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

// Auth Response Wrapper
type AuthResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      *models.User `json:"user"`
}

const sessionTTL = 30 * 24 * time.Hour

// Generate a random UUID-like string
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("secure random token generation failed: " + err.Error())
	}
	return "fnr_" + hex.EncodeToString(b)
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func issueSession(userID uint) (string, time.Time, error) {
	token := generateSessionToken()
	expiresAt := time.Now().UTC().Add(sessionTTL)
	session := models.AuthSession{
		UserID: userID, TokenHash: hashSessionToken(token), ExpiresAt: expiresAt,
	}
	if err := database.DB.Create(&session).Error; err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func authResponse(user *models.User) (AuthResponse, error) {
	token, expiresAt, err := issueSession(user.ID)
	if err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{Token: token, ExpiresAt: expiresAt, User: user}, nil
}

// POST /v1/auth/guest
func (s *Server) authGuest(c *gin.Context) {
	var input struct {
		DeviceID string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil && err != io.EOF {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}

	var user models.User
	if input.DeviceID != "" {
		err := database.DB.Where("device_id = ? AND is_guest = ?", input.DeviceID, true).First(&user).Error
		if err == nil {
			// Found existing guest session
			if err := ensureDefaultCashAccount(user.ID); err != nil {
				c.JSON(500, gin.H{"error": "failed_ensure_default_account"})
				return
			}
			user.HasPin = user.PinHash != ""
			response, err := authResponse(&user)
			if err != nil {
				c.JSON(500, gin.H{"error": "failed_create_session"})
				return
			}
			c.JSON(200, response)
			return
		}
	}

	var deviceIDPtr *string
	if input.DeviceID != "" {
		deviceIDPtr = &input.DeviceID
	}

	// Generate unique username
	// In production, you might want a retry loop here to ensure uniqueness
	username := "Guest_" + generateUUID()[:8]

	user = models.User{
		UUID:     generateUUID(),
		IsGuest:  true,
		DeviceID: deviceIDPtr,
		Username: username,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_create_guest"})
		return
	}
	if err := ensureDefaultCashAccount(user.ID); err != nil {
		c.JSON(500, gin.H{"error": "failed_create_default_account"})
		return
	}

	user.HasPin = user.PinHash != ""
	response, err := authResponse(&user)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed_create_session"})
		return
	}
	c.JSON(200, response)
}

func ensureDefaultCashAccount(userID uint) error {
	var count int64
	if err := database.DB.Model(&models.Account{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return database.DB.Create(&models.Account{
		UserID: userID, Type: "cash", Name: "Cash", IsDefault: true, Color: "#2ECC71",
	}).Error
}

// POST /v1/auth/identify
func (s *Server) authIdentify(c *gin.Context) {
	var input struct {
		Identifier string `json:"identifier" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	// Check both Email and Phone
	err := database.DB.Where("email = ? OR phone = ?", input.Identifier, input.Identifier).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		c.JSON(200, gin.H{"exists": false})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": "db_error"})
		return
	}

	c.JSON(200, gin.H{"exists": true, "is_guest": user.IsGuest})
}

// POST /v1/auth/otp/send
func (s *Server) authOtpSend(c *gin.Context) {
	// Mock OTP send
	c.JSON(200, gin.H{"message": "otp_sent", "mock_otp": "1234"})
}

// POST /v1/auth/otp/verify
func (s *Server) authOtpVerify(c *gin.Context) {
	var input struct {
		Identifier string `json:"identifier" binding:"required"`
		OTP        string `json:"otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Mock Verify
	if input.OTP != "123456" && input.OTP != "1234" {
		c.JSON(401, gin.H{"error": "invalid_otp"})
		return
	}

	// Return a claim token signed with the identifier type
	// Determine if email or phone (simple check)
	prefix := "phone:"
	if strings.Contains(input.Identifier, "@") {
		prefix = "email:"
	}

	claimToken := "claim_" + prefix + input.Identifier
	c.JSON(200, gin.H{"claim_token": claimToken})
}

// POST /v1/auth/register
func (s *Server) authRegister(c *gin.Context) {
	var input struct {
		ClaimToken        string `json:"claim_token" binding:"required"`
		PIN               string `json:"pin" binding:"required,len=4"`
		GuestUUID         string `json:"guest_uuid"`
		DeviceID          string `json:"device_id"`
		BiometricsEnabled bool   `json:"biometrics_enabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Parse claim token (Mock)
	if !strings.HasPrefix(input.ClaimToken, "claim_") {
		c.JSON(401, gin.H{"error": "invalid_claim_token"})
		return
	}

	tokenPayload := strings.TrimPrefix(input.ClaimToken, "claim_")
	parts := strings.SplitN(tokenPayload, ":", 2)
	if len(parts) != 2 {
		c.JSON(401, gin.H{"error": "invalid_claim_token_format"})
		return
	}

	identifierType := parts[0]
	identifier := parts[1]

	var email *string
	var phone *string

	if identifierType == "email" {
		email = &identifier
	} else {
		phone = &identifier
	}

	// Let's Hash PIN
	hash, err := bcrypt.GenerateFromPassword([]byte(input.PIN), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "encryption_failed"})
		return
	}

	// Check if identifier already taken
	var existing models.User
	query := "email = ?"
	if identifierType != "email" {
		query = "phone = ?"
	}

	if err := database.DB.Where(query, identifier).First(&existing).Error; err == nil {
		c.JSON(409, gin.H{"error": "user_already_exists"})
		return
	}

	var user models.User
	userFound := false

	// Prepare Device ID
	var deviceIDPtr *string
	if input.DeviceID != "" {
		deviceIDPtr = &input.DeviceID
	}

	if input.GuestUUID != "" {
		// Upgrade Flow
		err := database.DB.Where("uuid = ? AND is_guest = ?", input.GuestUUID, true).First(&user).Error
		if err == nil {
			if email != nil {
				user.Email = email
			}
			if phone != nil {
				user.Phone = phone
			}
			user.PinHash = string(hash)
			user.IsGuest = false
			user.BiometricsEnabled = input.BiometricsEnabled
			user.Username = "User_" + generateUUID()[:8] // Unique User Username
			if deviceIDPtr != nil {
				user.DeviceID = deviceIDPtr // Ensure device ID is carried over or updated
			}

			if err := database.DB.Save(&user).Error; err != nil {
				c.JSON(500, gin.H{"error": "failed_upgrade_guest"})
				return
			}
			userFound = true
		}
	}

	if !userFound {
		user = models.User{
			UUID:              generateUUID(),
			Email:             email,
			Phone:             phone,
			PinHash:           string(hash),
			IsGuest:           false,
			BiometricsEnabled: input.BiometricsEnabled,
			DeviceID:          deviceIDPtr,
			Username:          "User_" + generateUUID()[:8],
		}

		if err := database.DB.Create(&user).Error; err != nil {
			c.JSON(500, gin.H{"error": "db_error"})
			return
		}
	}
	if err := ensureDefaultCashAccount(user.ID); err != nil {
		c.JSON(500, gin.H{"error": "failed_ensure_default_account"})
		return
	}

	user.HasPin = user.PinHash != ""
	response, err := authResponse(&user)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed_create_session"})
		return
	}
	c.JSON(201, response)
}

// POST /v1/auth/login
func (s *Server) authLogin(c *gin.Context) {
	var input struct {
		Identifier string `json:"identifier" binding:"required"`
		PIN        string `json:"pin" binding:"required"`
		DeviceID   string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	// Search in both Email and Phone
	if err := database.DB.Where("email = ? OR phone = ?", input.Identifier, input.Identifier).First(&user).Error; err != nil {
		c.JSON(401, gin.H{"error": "invalid_credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(input.PIN)); err != nil {
		c.JSON(401, gin.H{"error": "invalid_credentials"})
		return
	}

	// Update Device ID if provided and different
	shouldSave := false
	if input.DeviceID != "" {
		if user.DeviceID == nil || *user.DeviceID != input.DeviceID {
			user.DeviceID = &input.DeviceID
			shouldSave = true
		}
	}

	if shouldSave {
		database.DB.Save(&user)
	}

	user.HasPin = user.PinHash != ""
	response, err := authResponse(&user)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed_create_session"})
		return
	}
	c.JSON(200, response)
}
