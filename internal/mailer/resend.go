package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultResendBaseURL = "https://api.resend.com"

// ResendOptions configures the Resend HTTP driver. Resend also speaks SMTP, so
// this driver is a convenience rather than a necessity: it needs one API key
// and no port or TLS decisions, which is the shortest path from a fresh
// account to a delivered code.
type ResendOptions struct {
	APIKey  string
	BaseURL string
	From    Address
	Client  *http.Client
}

type resendSender struct {
	opts ResendOptions
}

func NewResendSender(opts ResendOptions) Sender {
	if strings.TrimSpace(opts.BaseURL) == "" {
		opts.BaseURL = defaultResendBaseURL
	}
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 15 * time.Second}
	}
	return &resendSender{opts: opts}
}

func (r *resendSender) Name() string { return "resend" }

func (r *resendSender) Send(ctx context.Context, msg Message) error {
	if strings.TrimSpace(r.opts.APIKey) == "" {
		return errors.New("mailer: RESEND_API_KEY is required when EMAIL_PROVIDER=resend")
	}
	if err := r.opts.From.valid(); err != nil {
		return err
	}
	if err := msg.Validate(); err != nil {
		return err
	}

	payload := map[string]any{
		"from":    r.opts.From.String(),
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"text":    msg.Text,
	}
	if strings.TrimSpace(msg.HTML) != "" {
		payload["html"] = msg.HTML
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mailer: encode resend payload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.opts.BaseURL+"/emails", bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("mailer: build resend request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+r.opts.APIKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := r.opts.Client.Do(request)
	if err != nil {
		return fmt.Errorf("mailer: resend request: %w", err)
	}
	defer response.Body.Close()

	// Cap the error body: a provider returning an HTML error page should not
	// put a megabyte of markup into the API log.
	body, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("mailer: resend responded %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
