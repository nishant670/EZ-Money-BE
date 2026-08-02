package http

import (
	"net/http"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

func TestAuthLoginAcceptsLegacyRepeatedDigitPIN(t *testing.T) {
	useSmokeDatabase(t)
	router := smokeRouter(t)

	email := "legacy@example.com"
	hash, err := bcrypt.GenerateFromPassword([]byte("0000"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash legacy PIN: %v", err)
	}
	user := models.User{
		UUID:     generateUUID(),
		Email:    &email,
		PinHash:  string(hash),
		Username: "legacy_user",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create legacy user: %v", err)
	}

	response := performJSONRequest[AuthResponse](
		t,
		router,
		http.MethodPost,
		"/v1/auth/login",
		"",
		map[string]string{"identifier": email, "pin": "0000"},
		http.StatusOK,
	)
	if !strings.HasPrefix(response.Token, "fnr_") {
		t.Fatalf("expected opaque session token, got %q", response.Token)
	}
	if response.User == nil || response.User.ID != user.ID || !response.User.HasPin {
		t.Fatalf("unexpected login user response: %#v", response.User)
	}
}
