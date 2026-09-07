// Package payments talks to the payment provider. Only Razorpay is wired: the
// decision recorded in docs/PLANS_AND_CREDITS_PRICING_PLAN.md is web checkout
// on finnri-web rather than store billing, because UPI carries no MDR where
// Play Billing takes 15%.
//
// The package deliberately does nothing with subscriptions or credits. It
// creates orders and it verifies signatures; deciding what a verified event
// means is billing's job, not the transport's.
package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// ProviderRazorpay is the value written to payments.provider and
	// user_subscriptions.provider.
	ProviderRazorpay = "razorpay"

	defaultRazorpayBaseURL = "https://api.razorpay.com/v1"
)

// ErrNotConfigured is returned when a payment call is attempted without keys.
var ErrNotConfigured = errors.New("payments: razorpay is not configured")

// RazorpayConfig holds the three secrets Razorpay issues. KeyID is public —
// the browser needs it to open checkout. KeySecret and WebhookSecret are not,
// and neither is ever returned to a client.
type RazorpayConfig struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
	BaseURL       string
}

// Configured reports whether real orders can be created. The webhook secret is
// part of the test: without it no payment could ever be confirmed, so a
// deployment holding only the API keys must not advertise checkout as working.
func (c RazorpayConfig) Configured() bool {
	return strings.TrimSpace(c.KeyID) != "" &&
		strings.TrimSpace(c.KeySecret) != "" &&
		strings.TrimSpace(c.WebhookSecret) != ""
}

// Client calls the Razorpay REST API.
type Client struct {
	cfg        RazorpayConfig
	httpClient *http.Client
}

func NewClient(cfg RazorpayConfig) *Client {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultRazorpayBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{cfg: cfg, httpClient: &http.Client{Timeout: 20 * time.Second}}
}

// KeyID is the publishable key the checkout script needs.
func (c *Client) KeyID() string { return c.cfg.KeyID }

// Configured mirrors RazorpayConfig.Configured.
func (c *Client) Configured() bool { return c.cfg.Configured() }

// CreateOrderRequest describes one order. AmountMinor is in paise: Razorpay
// has no notion of rupees, and the plan catalogue is already stored in minor
// units, so no conversion happens anywhere in this path.
type CreateOrderRequest struct {
	AmountMinor int64
	Currency    string
	Receipt     string
	Notes       map[string]string
}

// Order is the subset of Razorpay's order object this codebase relies on.
type Order struct {
	ID          string `json:"id"`
	Entity      string `json:"entity"`
	AmountMinor int64  `json:"amount"`
	Currency    string `json:"currency"`
	Receipt     string `json:"receipt"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
}

// CreateOrder registers an order with Razorpay. The order is the thing the
// browser's checkout is opened against; it authorises nothing on its own.
func (c *Client) CreateOrder(ctx context.Context, request CreateOrderRequest) (Order, error) {
	if !c.cfg.Configured() {
		return Order{}, ErrNotConfigured
	}
	if request.AmountMinor <= 0 {
		return Order{}, fmt.Errorf("payments: order amount must be positive, got %d", request.AmountMinor)
	}
	if strings.TrimSpace(request.Currency) == "" {
		request.Currency = "INR"
	}
	// Razorpay caps the receipt at 40 characters and rejects the order
	// outright past it, which would surface as an opaque checkout failure.
	if len(request.Receipt) > 40 {
		request.Receipt = request.Receipt[:40]
	}

	payload := map[string]any{
		"amount":   request.AmountMinor,
		"currency": strings.ToUpper(request.Currency),
		"receipt":  request.Receipt,
	}
	if len(request.Notes) > 0 {
		payload["notes"] = request.Notes
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Order{}, fmt.Errorf("payments: encode order: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/orders", bytes.NewReader(encoded))
	if err != nil {
		return Order{}, fmt.Errorf("payments: build order request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.SetBasicAuth(c.cfg.KeyID, c.cfg.KeySecret)

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return Order{}, fmt.Errorf("payments: create order: %w", err)
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Order{}, fmt.Errorf("payments: razorpay responded %d: %s", response.StatusCode, razorpayErrorDescription(body))
	}

	var order Order
	if err := json.Unmarshal(body, &order); err != nil {
		return Order{}, fmt.Errorf("payments: decode order: %w", err)
	}
	if order.ID == "" {
		return Order{}, errors.New("payments: razorpay returned an order with no id")
	}
	return order, nil
}

// razorpayErrorDescription digs the human-readable reason out of an error
// body, so a log line says "amount must be atleast INR 1.00" rather than
// repeating a wall of JSON.
func razorpayErrorDescription(body []byte) string {
	var envelope struct {
		Error struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Description != "" {
		if envelope.Error.Code != "" {
			return envelope.Error.Code + ": " + envelope.Error.Description
		}
		return envelope.Error.Description
	}
	return strings.TrimSpace(string(body))
}

// VerifyWebhookSignature checks the X-Razorpay-Signature header against the
// raw request body. The body must be the exact bytes received: re-encoding
// parsed JSON reorders keys and changes the digest.
//
// hmac.Equal is a constant-time comparison, which matters here — a byte-wise
// early return would leak the expected digest to anyone able to time it.
func VerifyWebhookSignature(secret string, body []byte, signature string) bool {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(signature) == "" {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), provided)
}

// VerifyPaymentSignature checks the signature the browser receives from
// checkout, which is HMAC-SHA256 of "<order_id>|<payment_id>" keyed by the API
// secret. It is a fast confirmation for the success page; it is not authority
// to grant anything. Only the webhook is that.
func VerifyPaymentSignature(keySecret, orderID, paymentID, signature string) bool {
	if strings.TrimSpace(keySecret) == "" || orderID == "" || paymentID == "" {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(keySecret))
	mac.Write([]byte(orderID + "|" + paymentID))
	return hmac.Equal(mac.Sum(nil), provided)
}

// WebhookEnvelope is the shape every Razorpay webhook shares.
type WebhookEnvelope struct {
	Entity    string `json:"entity"`
	Event     string `json:"event"`
	CreatedAt int64  `json:"created_at"`
	Payload   struct {
		Payment struct {
			Entity WebhookPayment `json:"entity"`
		} `json:"payment"`
		Order struct {
			Entity WebhookOrder `json:"entity"`
		} `json:"order"`
		Refund struct {
			Entity WebhookRefund `json:"entity"`
		} `json:"refund"`
	} `json:"payload"`
}

type WebhookPayment struct {
	ID                string            `json:"id"`
	OrderID           string            `json:"order_id"`
	Status            string            `json:"status"`
	AmountMinor       int64             `json:"amount"`
	AmountRefunded    int64             `json:"amount_refunded"`
	Currency          string            `json:"currency"`
	Method            string            `json:"method"`
	ErrorCode         string            `json:"error_code"`
	ErrorDescription  string            `json:"error_description"`
	InternationalOnly bool              `json:"international"`
	Notes             map[string]string `json:"-"`
	RawNotes          json.RawMessage   `json:"notes"`
}

type WebhookOrder struct {
	ID          string `json:"id"`
	AmountMinor int64  `json:"amount"`
	AmountPaid  int64  `json:"amount_paid"`
	Status      string `json:"status"`
	Receipt     string `json:"receipt"`
}

type WebhookRefund struct {
	ID          string `json:"id"`
	PaymentID   string `json:"payment_id"`
	AmountMinor int64  `json:"amount"`
	Status      string `json:"status"`
}

// ParseWebhook decodes a verified webhook body.
//
// Notes are decoded leniently: Razorpay sends an empty JSON array rather than
// an object when there are none, which fails a map[string]string decode and
// would otherwise reject every note-free event.
func ParseWebhook(body []byte) (WebhookEnvelope, error) {
	var envelope WebhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return WebhookEnvelope{}, fmt.Errorf("payments: decode webhook: %w", err)
	}
	if envelope.Event == "" {
		return WebhookEnvelope{}, errors.New("payments: webhook has no event name")
	}
	if len(envelope.Payload.Payment.Entity.RawNotes) > 0 {
		notes := map[string]string{}
		if err := json.Unmarshal(envelope.Payload.Payment.Entity.RawNotes, &notes); err == nil {
			envelope.Payload.Payment.Entity.Notes = notes
		}
	}
	return envelope, nil
}

// CheckoutURL is unused by the backend but documents where the browser is sent
// when the hosted page is preferred over the embedded script.
func CheckoutURL(orderID string) string {
	return "https://api.razorpay.com/v1/checkout/embedded?" + url.Values{"order_id": {orderID}}.Encode()
}
