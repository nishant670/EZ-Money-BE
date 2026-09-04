package http

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
	"finance-parser-go/internal/mailer"
	"finance-parser-go/internal/models"
)

// stubMailer records what would have been sent, and can be told to fail, so
// the tests below can assert on the code that actually reaches an inbox rather
// than on the response body alone.
type stubMailer struct {
	mu   sync.Mutex
	sent []mailer.Message
	err  error
}

func (s *stubMailer) Name() string { return "stub" }

func (s *stubMailer) Send(_ context.Context, msg mailer.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, msg)
	return nil
}

func (s *stubMailer) messages() []mailer.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mailer.Message(nil), s.sent...)
}

func withMailer(sender mailer.Sender) func(*Server, *config.Config) {
	return func(s *Server, cfg *config.Config) {
		s.mailer = sender
		cfg.OTPExpiresMinutes = 10
	}
}

// TestAuthOtpSendDeliversTheCodeThatVerifies is the regression guard for the
// defect this endpoint shipped with: it answered "otp_sent" while no mail
// existed. Asserting a 200 is not enough — the test pulls the code out of the
// delivered message and spends it on /otp/verify, which is the only way to
// prove the thing in the inbox is the thing the server will accept.
func TestAuthOtpSendDeliversTheCodeThatVerifies(t *testing.T) {
	useSmokeDatabase(t)
	sender := &stubMailer{}
	router := smokeRouter(t, withMailer(sender))

	performJSONRequest[map[string]any](
		t, router, http.MethodPost, "/v1/auth/otp/send", "",
		map[string]string{"identifier": "Signup@Example.com "}, http.StatusOK,
	)

	messages := sender.messages()
	if len(messages) != 1 {
		t.Fatalf("expected exactly one email, sent %d", len(messages))
	}
	// The identifier is normalised before it is used as a recipient.
	if messages[0].To != "signup@example.com" {
		t.Fatalf("expected normalised recipient, got %q", messages[0].To)
	}

	code := extractOTPCode(t, messages[0])
	claim := performJSONRequest[map[string]any](
		t, router, http.MethodPost, "/v1/auth/otp/verify", "",
		map[string]string{"identifier": "signup@example.com", "otp": code}, http.StatusOK,
	)
	token, _ := claim["claim_token"].(string)
	if !strings.HasPrefix(token, claimTokenPrefix) {
		t.Fatalf("emailed code did not verify into a claim token: %#v", claim)
	}
}

// TestAuthOtpSendSurfacesDeliveryFailure covers the second half of the defect:
// a failure must reach the caller, and must not leave a live code behind that
// nobody received but the cooldown still counts against.
func TestAuthOtpSendSurfacesDeliveryFailure(t *testing.T) {
	useSmokeDatabase(t)
	sender := &stubMailer{err: context.DeadlineExceeded}
	router := smokeRouter(t, withMailer(sender))

	body := performJSONRequest[map[string]any](
		t, router, http.MethodPost, "/v1/auth/otp/send", "",
		map[string]string{"identifier": "down@example.com"}, http.StatusBadGateway,
	)
	if body["error"] != "otp_send_failed" {
		t.Fatalf("expected otp_send_failed, got %#v", body)
	}

	var remaining int64
	if err := database.DB.Model(&models.AuthVerification{}).
		Where("identifier = ?", "down@example.com").Count(&remaining).Error; err != nil {
		t.Fatalf("failed to count verifications: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("a failed send left %d verification rows behind", remaining)
	}
}

// TestAuthOtpSendWithNoProviderConfiguredFails pins the default. A deployment
// that forgets EMAIL_PROVIDER must break loudly at the first signup, not go on
// claiming to have sent codes.
func TestAuthOtpSendWithNoProviderConfiguredFails(t *testing.T) {
	useSmokeDatabase(t)
	router := smokeRouter(t)

	body := performJSONRequest[map[string]any](
		t, router, http.MethodPost, "/v1/auth/otp/send", "",
		map[string]string{"identifier": "nobody@example.com"}, http.StatusBadGateway,
	)
	if body["error"] != "otp_send_failed" {
		t.Fatalf("expected otp_send_failed with no provider, got %#v", body)
	}
}

func TestAuthOtpSendRejectsPhoneWhileSMSIsUnwired(t *testing.T) {
	useSmokeDatabase(t)
	sender := &stubMailer{}
	router := smokeRouter(t, withMailer(sender))

	body := performJSONRequest[map[string]any](
		t, router, http.MethodPost, "/v1/auth/otp/send", "",
		map[string]string{"identifier": "9876543210"}, http.StatusUnprocessableEntity,
	)
	if body["error"] != "otp_channel_unavailable" {
		t.Fatalf("expected otp_channel_unavailable, got %#v", body)
	}
	if len(sender.messages()) != 0 {
		t.Fatalf("a phone identifier must not send email")
	}
	var stored int64
	database.DB.Model(&models.AuthVerification{}).Where("identifier_type = ?", "phone").Count(&stored)
	if stored != 0 {
		t.Fatalf("a rejected phone request stored %d codes", stored)
	}
}

func TestAuthOtpSendThrottlesRepeatSendsToOneAddress(t *testing.T) {
	useSmokeDatabase(t)
	sender := &stubMailer{}
	router := smokeRouter(t, func(s *Server, cfg *config.Config) {
		withMailer(sender)(s, cfg)
		cfg.OTPResendCooldownSeconds = 60
	})

	performJSONRequest[map[string]any](
		t, router, http.MethodPost, "/v1/auth/otp/send", "",
		map[string]string{"identifier": "repeat@example.com"}, http.StatusOK,
	)

	response := performRawRequest(t, router, http.MethodPost, "/v1/auth/otp/send", "",
		strings.NewReader(`{"identifier":"repeat@example.com"}`))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on immediate resend, got %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Retry-After") == "" {
		t.Fatalf("a throttled resend must tell the app when to retry")
	}
	if len(sender.messages()) != 1 {
		t.Fatalf("the throttled resend still sent mail: %d messages", len(sender.messages()))
	}
}

// TestOtpResendCooldownExpires proves the throttle is a window, not a lock.
func TestOtpResendCooldownExpires(t *testing.T) {
	useSmokeDatabase(t)
	created := time.Now().UTC().Add(-90 * time.Second)
	verification := models.AuthVerification{
		IdentifierType: "email",
		Identifier:     "cooled@example.com",
		OTPHash:        "irrelevant",
		OTPExpiresAt:   created.Add(10 * time.Minute),
		CreatedAt:      created,
	}
	if err := database.DB.Create(&verification).Error; err != nil {
		t.Fatalf("failed to seed verification: %v", err)
	}

	if _, blocked := otpResendCooldownRemaining("email", "cooled@example.com", 60, time.Now().UTC()); blocked {
		t.Fatalf("a 90-second-old code must not block a 60-second cooldown")
	}
	if remaining, blocked := otpResendCooldownRemaining("email", "cooled@example.com", 300, time.Now().UTC()); !blocked || remaining <= 0 {
		t.Fatalf("a 90-second-old code must block a 300-second cooldown, got remaining=%d blocked=%v", remaining, blocked)
	}
}

// extractOTPCode pulls the six digits out of a delivered message and checks
// that every part of the email agrees on them — a subject advertising one code
// and a body carrying another is the kind of defect that only shows up on a
// real phone.
func extractOTPCode(t *testing.T, msg mailer.Message) string {
	t.Helper()
	code := ""
	for _, field := range strings.Fields(msg.Subject) {
		if validOTPCode(field) {
			code = field
			break
		}
	}
	if code == "" {
		t.Fatalf("no six-digit code in subject %q", msg.Subject)
	}
	if !strings.Contains(msg.Text, code) {
		t.Fatalf("text body does not carry the subject's code %q: %s", code, msg.Text)
	}
	if msg.HTML != "" && !strings.Contains(msg.HTML, code) {
		t.Fatalf("html body does not carry the subject's code %q", code)
	}
	return code
}
