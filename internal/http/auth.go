package http

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"finance-parser-go/internal/billing"
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
const maxOTPAttempts = 5
const maxPINAttempts = 5
const loginLockDuration = 15 * time.Minute
const claimTokenPrefix = "fnrct_"

type verifiedClaim struct {
	IdentifierType string
	Identifier     string
}

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

func generateOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func validOTPCode(otp string) bool {
	if len(otp) != 6 {
		return false
	}
	for _, char := range otp {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validPINFormat(pin string) bool {
	if len(pin) != 4 {
		return false
	}
	for _, char := range pin {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validPIN(pin string) bool {
	if !validPINFormat(pin) {
		return false
	}
	allSame := true
	for i, char := range pin {
		if i > 0 && byte(char) != pin[0] {
			allSame = false
		}
	}
	return !allSame
}

func hashOTP(otp string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyOTPHash(hash, otp string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(otp)) == nil
}

func generateClaimToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("secure random claim token generation failed: " + err.Error())
	}
	return claimTokenPrefix + hex.EncodeToString(b)
}

func validClaimTokenFormat(token string) bool {
	if !strings.HasPrefix(token, claimTokenPrefix) {
		return false
	}
	raw := strings.TrimPrefix(token, claimTokenPrefix)
	if len(raw) != 64 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

func hashClaimToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeIdentifier(identifier string) (string, string, error) {
	normalized := strings.TrimSpace(identifier)
	if normalized == "" {
		return "", "", errors.New("identifier_required")
	}
	if strings.Contains(normalized, "@") {
		return "email", strings.ToLower(normalized), nil
	}
	return "phone", normalized, nil
}

func consumeClaimToken(rawToken string) (verifiedClaim, error) {
	if !validClaimTokenFormat(rawToken) {
		return verifiedClaim{}, errors.New("invalid_claim_token")
	}

	var verification models.AuthVerification
	now := time.Now().UTC()
	if err := database.DB.
		Where("claim_token_hash = ? AND claim_used_at IS NULL AND claim_expires_at > ?", hashClaimToken(rawToken), now).
		First(&verification).Error; err != nil {
		return verifiedClaim{}, errors.New("invalid_or_expired_claim_token")
	}

	verification.ClaimUsedAt = &now
	if err := database.DB.Save(&verification).Error; err != nil {
		return verifiedClaim{}, err
	}

	return verifiedClaim{IdentifierType: verification.IdentifierType, Identifier: verification.Identifier}, nil
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

func findUserByVerifiedIdentifier(identifierType, identifier string) (models.User, error) {
	var user models.User
	query := "email = ?"
	if identifierType != "email" {
		query = "phone = ?"
	}
	err := database.DB.Where(query, identifier).First(&user).Error
	return user, err
}

func findUserByLoginIdentifier(identifier string) (models.User, error) {
	identifierType, normalized, err := normalizeIdentifier(identifier)
	if err != nil {
		return models.User{}, err
	}
	return findUserByVerifiedIdentifier(identifierType, normalized)
}

func resetLoginLock(user *models.User) error {
	user.FailedLoginAttempts = 0
	user.LoginLockedUntil = nil
	return database.DB.Model(user).Updates(map[string]interface{}{
		"failed_login_attempts": 0,
		"login_locked_until":    nil,
	}).Error
}

func clearExpiredLoginLock(user *models.User, now time.Time) error {
	if user.LoginLockedUntil == nil || user.LoginLockedUntil.After(now) {
		return nil
	}
	return resetLoginLock(user)
}

func recordFailedLoginAttempt(user *models.User, now time.Time) (int, *time.Time, error) {
	attempts := user.FailedLoginAttempts + 1
	updates := map[string]interface{}{
		"failed_login_attempts": attempts,
	}

	var lockedUntil *time.Time
	if attempts >= maxPINAttempts {
		lockUntil := now.Add(loginLockDuration)
		lockedUntil = &lockUntil
		updates["login_locked_until"] = lockedUntil
	}

	if err := database.DB.Model(user).Updates(updates).Error; err != nil {
		return 0, nil, err
	}

	user.FailedLoginAttempts = attempts
	user.LoginLockedUntil = lockedUntil
	remaining := maxPINAttempts - attempts
	if remaining < 0 {
		remaining = 0
	}
	return remaining, lockedUntil, nil
}

// POST /v1/auth/guest
func (s *Server) authGuest(c *gin.Context) {
	var input struct {
		DeviceID string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil && err != io.EOF {
		if requestBodyTooLarge(err) {
			c.JSON(413, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}

	deviceID := strings.TrimSpace(input.DeviceID)
	var user models.User
	if deviceID != "" {
		if err := database.DB.
			Where("device_id = ? AND is_guest = ?", deviceID, true).
			Limit(1).
			Find(&user).Error; err != nil {
			c.JSON(500, gin.H{"error": "failed_lookup_guest"})
			return
		}
		if user.ID != 0 {
			// Found existing guest session
			if err := ensureDefaultCashAccount(user.ID); err != nil {
				c.JSON(500, gin.H{"error": "failed_ensure_default_account"})
				return
			}
			if _, _, err := billing.NewCreditService(database.DB).EnsureGuestTrialGrant(deviceID, c.ClientIP()); err != nil {
				c.JSON(500, gin.H{"error": "failed_ensure_guest_credits"})
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
	if deviceID != "" {
		deviceIDPtr = &deviceID
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
		if deviceID != "" {
			var existingGuest models.User
			lookupErr := database.DB.
				Where("device_id = ? AND is_guest = ?", deviceID, true).
				Limit(1).
				Find(&existingGuest).Error
			if lookupErr == nil && existingGuest.ID != 0 {
				if err := ensureDefaultCashAccount(existingGuest.ID); err != nil {
					c.JSON(500, gin.H{"error": "failed_ensure_default_account"})
					return
				}
				if _, _, err := billing.NewCreditService(database.DB).EnsureGuestTrialGrant(deviceID, c.ClientIP()); err != nil {
					c.JSON(500, gin.H{"error": "failed_ensure_guest_credits"})
					return
				}
				existingGuest.HasPin = existingGuest.PinHash != ""
				response, err := authResponse(&existingGuest)
				if err != nil {
					c.JSON(500, gin.H{"error": "failed_create_session"})
					return
				}
				c.JSON(200, response)
				return
			}
		}
		c.JSON(500, gin.H{"error": "failed_create_guest"})
		return
	}
	if err := ensureDefaultCashAccount(user.ID); err != nil {
		c.JSON(500, gin.H{"error": "failed_create_default_account"})
		return
	}
	if _, _, err := billing.NewCreditService(database.DB).EnsureGuestTrialGrant(deviceID, c.ClientIP()); err != nil {
		c.JSON(500, gin.H{"error": "failed_ensure_guest_credits"})
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
		if requestBodyTooLarge(err) {
			c.JSON(413, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	identifierType, identifier, err := normalizeIdentifier(input.Identifier)
	if err != nil {
		c.JSON(422, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	query := "email = ?"
	if identifierType == "phone" {
		query = "phone = ?"
	}
	err = database.DB.Where(query, identifier).First(&user).Error
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
	var input struct {
		Identifier string `json:"identifier" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		if requestBodyTooLarge(err) {
			c.JSON(413, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	identifierType, identifier, err := normalizeIdentifier(input.Identifier)
	if err != nil {
		c.JSON(422, gin.H{"error": err.Error()})
		return
	}

	otp := s.cfg.OTPDevCode
	if !s.cfg.OTPDebugResponse || !validOTPCode(otp) {
		generatedOTP, err := generateOTPCode()
		if err != nil {
			c.JSON(500, gin.H{"error": "otp_generation_failed"})
			return
		}
		otp = generatedOTP
	}
	otpHash, err := hashOTP(otp)
	if err != nil {
		c.JSON(500, gin.H{"error": "otp_hash_failed"})
		return
	}

	expiresAt := time.Now().UTC().Add(time.Duration(s.cfg.OTPExpiresMinutes) * time.Minute)
	verification := models.AuthVerification{
		IdentifierType: identifierType,
		Identifier:     identifier,
		OTPHash:        otpHash,
		OTPExpiresAt:   expiresAt,
	}
	if err := database.DB.Create(&verification).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_create_otp"})
		return
	}

	response := gin.H{"message": "otp_sent", "expires_at": expiresAt}
	if s.cfg.OTPDebugResponse {
		response["dev_otp"] = otp
	}
	c.JSON(200, response)
}

// POST /v1/auth/otp/verify
func (s *Server) authOtpVerify(c *gin.Context) {
	var input struct {
		Identifier string `json:"identifier" binding:"required"`
		OTP        string `json:"otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		if requestBodyTooLarge(err) {
			c.JSON(413, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	identifierType, identifier, err := normalizeIdentifier(input.Identifier)
	if err != nil {
		c.JSON(422, gin.H{"error": err.Error()})
		return
	}

	var verification models.AuthVerification
	now := time.Now().UTC()
	err = database.DB.
		Where("identifier_type = ? AND identifier = ? AND verified_at IS NULL AND otp_expires_at > ?", identifierType, identifier, now).
		Order("created_at DESC").
		First(&verification).Error
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid_or_expired_otp"})
		return
	}

	if verification.Attempts >= maxOTPAttempts {
		c.JSON(429, gin.H{"error": "too_many_otp_attempts"})
		return
	}

	if !verifyOTPHash(verification.OTPHash, input.OTP) {
		database.DB.Model(&verification).Update("attempts", verification.Attempts+1)
		c.JSON(401, gin.H{"error": "invalid_otp"})
		return
	}

	claimToken := generateClaimToken()
	claimTokenHash := hashClaimToken(claimToken)
	claimExpiresAt := now.Add(time.Duration(s.cfg.ClaimTokenMinutes) * time.Minute)
	verification.VerifiedAt = &now
	verification.ClaimTokenHash = &claimTokenHash
	verification.ClaimExpiresAt = &claimExpiresAt
	if err := database.DB.Save(&verification).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_create_claim"})
		return
	}

	c.JSON(200, gin.H{"claim_token": claimToken, "expires_at": claimExpiresAt})
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
		if requestBodyTooLarge(err) {
			c.JSON(413, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !validPIN(input.PIN) {
		c.JSON(400, gin.H{"error": "weak_pin"})
		return
	}

	claim, err := consumeClaimToken(input.ClaimToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid_claim_token"})
		return
	}

	var email *string
	var phone *string

	if claim.IdentifierType == "email" {
		email = &claim.Identifier
	} else {
		phone = &claim.Identifier
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
	if claim.IdentifierType != "email" {
		query = "phone = ?"
	}

	if err := database.DB.Where(query, claim.Identifier).First(&existing).Error; err == nil {
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
	if _, _, err := billing.NewCreditService(database.DB).EnsureLoggedInFreeTrialGrant(user.ID); err != nil {
		c.JSON(500, gin.H{"error": "failed_ensure_trial_credits"})
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

// POST /v1/auth/google
func (s *Server) authGoogle(c *gin.Context) {
	var input struct {
		IDToken           string `json:"id_token" binding:"required"`
		Nonce             string `json:"nonce"`
		GuestUUID         string `json:"guest_uuid"`
		DeviceID          string `json:"device_id"`
		BiometricsEnabled bool   `json:"biometrics_enabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		if requestBodyTooLarge(err) {
			c.JSON(413, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if len(s.cfg.GoogleClientIDs) == 0 {
		c.JSON(503, gin.H{"error": "google_login_not_configured"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	identity, err := verifyGoogleIDToken(ctx, strings.TrimSpace(input.IDToken), s.cfg.GoogleClientIDs)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}
	if input.Nonce != "" && identity.Nonce != input.Nonce {
		c.JSON(401, gin.H{"error": "invalid_google_nonce"})
		return
	}

	var user models.User
	googleSubject := identity.Subject
	email := identity.Email
	deviceID := strings.TrimSpace(input.DeviceID)
	deviceIDPtr := (*string)(nil)
	if deviceID != "" {
		deviceIDPtr = &deviceID
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("google_subject = ?", googleSubject).First(&user).Error; err == nil {
			updates := map[string]interface{}{
				"failed_login_attempts": 0,
				"login_locked_until":    nil,
			}
			if user.Email == nil || *user.Email == "" {
				updates["email"] = email
			}
			if deviceID != "" {
				updates["device_id"] = deviceID
			}
			if err := tx.Model(&user).Updates(updates).Error; err != nil {
				return err
			}
			return tx.First(&user, user.ID).Error
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		var existing models.User
		if err := tx.Where("LOWER(email) = LOWER(?)", email).First(&existing).Error; err == nil {
			if existing.GoogleSubject != nil && *existing.GoogleSubject != googleSubject {
				return errors.New("email_linked_to_different_google_account")
			}
			updates := map[string]interface{}{
				"google_subject":        googleSubject,
				"failed_login_attempts": 0,
				"login_locked_until":    nil,
			}
			if deviceID != "" {
				updates["device_id"] = deviceID
			}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return err
			}
			user = existing
			return tx.First(&user, existing.ID).Error
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		if strings.TrimSpace(input.GuestUUID) != "" {
			if err := tx.Where("uuid = ? AND is_guest = ?", strings.TrimSpace(input.GuestUUID), true).First(&user).Error; err == nil {
				user.Email = &email
				user.GoogleSubject = &googleSubject
				user.IsGuest = false
				user.BiometricsEnabled = input.BiometricsEnabled
				user.Username = "User_" + generateUUID()[:8]
				if identity.Picture != "" {
					user.ProfileImage = identity.Picture
				}
				if deviceIDPtr != nil {
					user.DeviceID = deviceIDPtr
				}
				return tx.Save(&user).Error
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
		}

		user = models.User{
			UUID:              generateUUID(),
			Email:             &email,
			GoogleSubject:     &googleSubject,
			IsGuest:           false,
			BiometricsEnabled: input.BiometricsEnabled,
			DeviceID:          deviceIDPtr,
			Username:          "User_" + generateUUID()[:8],
			ProfileImage:      identity.Picture,
		}
		return tx.Create(&user).Error
	})
	if err != nil {
		if err.Error() == "email_linked_to_different_google_account" {
			c.JSON(409, gin.H{"error": "email_linked_to_different_google_account"})
			return
		}
		c.JSON(500, gin.H{"error": "google_login_failed"})
		return
	}

	if err := ensureDefaultCashAccount(user.ID); err != nil {
		c.JSON(500, gin.H{"error": "failed_ensure_default_account"})
		return
	}
	if _, _, err := billing.NewCreditService(database.DB).EnsureLoggedInFreeTrialGrant(user.ID); err != nil {
		c.JSON(500, gin.H{"error": "failed_ensure_trial_credits"})
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

// POST /v1/auth/pin/reset
func (s *Server) authPinReset(c *gin.Context) {
	var input struct {
		ClaimToken        string `json:"claim_token" binding:"required"`
		PIN               string `json:"pin" binding:"required,len=4"`
		DeviceID          string `json:"device_id"`
		BiometricsEnabled *bool  `json:"biometrics_enabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		if requestBodyTooLarge(err) {
			c.JSON(413, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !validPIN(input.PIN) {
		c.JSON(400, gin.H{"error": "weak_pin"})
		return
	}

	claim, err := consumeClaimToken(input.ClaimToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid_claim_token"})
		return
	}

	user, err := findUserByVerifiedIdentifier(claim.IdentifierType, claim.Identifier)
	if err != nil {
		c.JSON(404, gin.H{"error": "user_not_found"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.PIN), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "encryption_failed"})
		return
	}

	updates := map[string]interface{}{
		"pin_hash":              string(hash),
		"failed_login_attempts": 0,
		"login_locked_until":    nil,
	}
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID != "" {
		updates["device_id"] = deviceID
	}
	if input.BiometricsEnabled != nil {
		updates["biometrics_enabled"] = *input.BiometricsEnabled
	}

	if err := database.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_reset_pin"})
		return
	}
	if err := database.DB.First(&user, user.ID).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed_load_user"})
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

// POST /v1/auth/login
func (s *Server) authLogin(c *gin.Context) {
	var input struct {
		Identifier string `json:"identifier" binding:"required"`
		PIN        string `json:"pin" binding:"required"`
		DeviceID   string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		if requestBodyTooLarge(err) {
			c.JSON(413, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !validPINFormat(input.PIN) {
		c.JSON(401, gin.H{"error": "invalid_credentials"})
		return
	}

	user, err := findUserByLoginIdentifier(input.Identifier)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid_credentials"})
		return
	}

	now := time.Now().UTC()
	if err := clearExpiredLoginLock(&user, now); err != nil {
		c.JSON(500, gin.H{"error": "failed_update_login_lock"})
		return
	}
	if user.LoginLockedUntil != nil && user.LoginLockedUntil.After(now) {
		c.JSON(429, gin.H{
			"error":        "login_locked",
			"locked_until": user.LoginLockedUntil,
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(input.PIN)); err != nil {
		attemptsRemaining, lockedUntil, updateErr := recordFailedLoginAttempt(&user, now)
		if updateErr != nil {
			c.JSON(500, gin.H{"error": "failed_update_login_attempts"})
			return
		}
		if lockedUntil != nil {
			c.JSON(429, gin.H{
				"error":        "login_locked",
				"locked_until": lockedUntil,
			})
			return
		}
		c.JSON(401, gin.H{
			"error":              "invalid_credentials",
			"attempts_remaining": attemptsRemaining,
		})
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
	if user.FailedLoginAttempts != 0 || user.LoginLockedUntil != nil {
		if err := resetLoginLock(&user); err != nil {
			c.JSON(500, gin.H{"error": "failed_update_login_lock"})
			return
		}
	}

	user.HasPin = user.PinHash != ""
	response, err := authResponse(&user)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed_create_session"})
		return
	}
	c.JSON(200, response)
}
