package http

import (
	"context"
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

func TestAuthGoogleCreatesUserAndSession(t *testing.T) {
	useSmokeDatabase(t)
	router := smokeRouter(t)
	stubGoogleVerifier(t, googleIdentity{
		Subject:       "google-sub-new",
		Email:         "new-google@example.com",
		EmailVerified: true,
		Name:          "New Google",
		Picture:       "https://example.com/avatar.png",
	})

	response := performJSONRequest[AuthResponse](
		t,
		router,
		http.MethodPost,
		"/v1/auth/google",
		"",
		map[string]string{"id_token": "valid-google-token", "device_id": "google-device"},
		http.StatusOK,
	)

	if !strings.HasPrefix(response.Token, "fnr_") {
		t.Fatalf("expected opaque session token, got %q", response.Token)
	}
	if response.User == nil || response.User.Email == nil || *response.User.Email != "new-google@example.com" || response.User.IsGuest {
		t.Fatalf("unexpected google user response: %#v", response.User)
	}
	if response.User.HasPin {
		t.Fatalf("google-only account should not require an existing PIN")
	}
}

func TestAuthGoogleLinksExistingEmailUser(t *testing.T) {
	useSmokeDatabase(t)
	router := smokeRouter(t)
	email := "link-google@example.com"
	user := models.User{
		UUID:     generateUUID(),
		Email:    &email,
		PinHash:  "legacy-pin-hash",
		Username: "link_google_user",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create existing user: %v", err)
	}
	stubGoogleVerifier(t, googleIdentity{
		Subject:       "google-sub-link",
		Email:         email,
		EmailVerified: true,
	})

	response := performJSONRequest[AuthResponse](
		t,
		router,
		http.MethodPost,
		"/v1/auth/google",
		"",
		map[string]string{"id_token": "valid-google-token"},
		http.StatusOK,
	)

	if response.User == nil || response.User.ID != user.ID {
		t.Fatalf("expected existing user to be linked, got %#v", response.User)
	}
	var linked models.User
	if err := database.DB.First(&linked, user.ID).Error; err != nil {
		t.Fatalf("failed to reload linked user: %v", err)
	}
	if linked.GoogleSubject == nil || *linked.GoogleSubject != "google-sub-link" {
		t.Fatalf("expected google subject to be linked, got %#v", linked.GoogleSubject)
	}
}

func TestAuthGoogleUpgradesGuest(t *testing.T) {
	useSmokeDatabase(t)
	router := smokeRouter(t)
	guestResponse := performJSONRequest[AuthResponse](
		t,
		router,
		http.MethodPost,
		"/v1/auth/guest",
		"",
		map[string]string{"device_id": "google-guest-device"},
		http.StatusOK,
	)
	stubGoogleVerifier(t, googleIdentity{
		Subject:       "google-sub-guest",
		Email:         "guest-upgrade@example.com",
		EmailVerified: true,
	})

	response := performJSONRequest[AuthResponse](
		t,
		router,
		http.MethodPost,
		"/v1/auth/google",
		"",
		map[string]string{
			"id_token":   "valid-google-token",
			"guest_uuid": guestResponse.User.UUID,
			"device_id":  "google-guest-device",
		},
		http.StatusOK,
	)

	if response.User == nil || response.User.UUID != guestResponse.User.UUID || response.User.IsGuest {
		t.Fatalf("expected guest to be upgraded in place, got %#v", response.User)
	}
}

func stubGoogleVerifier(t *testing.T, identity googleIdentity) {
	t.Helper()
	previous := verifyGoogleIDToken
	verifyGoogleIDToken = func(ctx context.Context, rawToken string, audiences []string) (googleIdentity, error) {
		if rawToken != "valid-google-token" {
			t.Fatalf("unexpected google token %q", rawToken)
		}
		if len(audiences) != 1 || audiences[0] != "test-google-client" {
			t.Fatalf("unexpected google audiences %#v", audiences)
		}
		return identity, nil
	}
	t.Cleanup(func() {
		verifyGoogleIDToken = previous
	})
}
