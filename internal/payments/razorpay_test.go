package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestConfiguredRequiresAllThreeSecrets(t *testing.T) {
	for _, testCase := range []struct {
		name string
		cfg  RazorpayConfig
		want bool
	}{
		{"complete", RazorpayConfig{KeyID: "k", KeySecret: "s", WebhookSecret: "w"}, true},
		{"no key id", RazorpayConfig{KeySecret: "s", WebhookSecret: "w"}, false},
		{"no secret", RazorpayConfig{KeyID: "k", WebhookSecret: "w"}, false},
		// Without the webhook secret nothing could ever be confirmed, so
		// advertising checkout would take money and grant nothing.
		{"no webhook secret", RazorpayConfig{KeyID: "k", KeySecret: "s"}, false},
		{"whitespace only", RazorpayConfig{KeyID: " ", KeySecret: " ", WebhookSecret: " "}, false},
	} {
		if got := testCase.cfg.Configured(); got != testCase.want {
			t.Fatalf("%s: Configured() = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"event":"payment.captured","payload":{}}`)
	valid := sign(secret, body)

	if !VerifyWebhookSignature(secret, body, valid) {
		t.Fatal("a correctly signed body must verify")
	}
	if VerifyWebhookSignature(secret, body, strings.ToUpper(valid)) != true {
		// hex decoding is case-insensitive, so an upper-case digest is still
		// the same bytes. Asserting it keeps a future "normalise" refactor
		// from quietly rejecting valid deliveries.
		t.Fatal("an upper-case hex signature is the same digest and must verify")
	}
	if VerifyWebhookSignature("whsec_other", body, valid) {
		t.Fatal("a signature under a different secret must not verify")
	}
	if VerifyWebhookSignature(secret, append(body, ' '), valid) {
		t.Fatal("a tampered body must not verify")
	}
	if VerifyWebhookSignature(secret, body, "not-hex") {
		t.Fatal("a non-hex signature must not verify")
	}
	if VerifyWebhookSignature(secret, body, "") {
		t.Fatal("an absent signature must not verify")
	}
	if VerifyWebhookSignature("", body, valid) {
		t.Fatal("an unconfigured secret must never verify anything")
	}
}

func TestVerifyPaymentSignature(t *testing.T) {
	secret := "rzp_secret"
	orderID, paymentID := "order_ABC", "pay_XYZ"
	valid := sign(secret, []byte(orderID+"|"+paymentID))

	if !VerifyPaymentSignature(secret, orderID, paymentID, valid) {
		t.Fatal("the documented order|payment digest must verify")
	}
	// Swapping the two halves must not verify — otherwise the concatenation is
	// ambiguous and a crafted pair could be made to collide.
	if VerifyPaymentSignature(secret, paymentID, orderID, valid) {
		t.Fatal("transposed ids must not verify")
	}
	if VerifyPaymentSignature(secret, orderID, "", valid) {
		t.Fatal("a missing payment id must not verify")
	}
}

func TestCreateOrderPostsAmountInMinorUnits(t *testing.T) {
	var captured map[string]any
	var gotUser, gotPass string
	var ok bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, ok = r.BasicAuth()
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"order_TEST","entity":"order","amount":14900,"currency":"INR","status":"created","receipt":"fnr_1"}`))
	}))
	defer server.Close()

	client := NewClient(RazorpayConfig{KeyID: "rzp_key", KeySecret: "rzp_secret", WebhookSecret: "w", BaseURL: server.URL})
	order, err := client.CreateOrder(context.Background(), CreateOrderRequest{
		AmountMinor: 14900,
		Currency:    "inr",
		Receipt:     "fnr_1",
		Notes:       map[string]string{"plan_code": "monthly"},
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	if order.ID != "order_TEST" {
		t.Fatalf("unexpected order id %q", order.ID)
	}
	if !ok || gotUser != "rzp_key" || gotPass != "rzp_secret" {
		t.Fatalf("expected basic auth with the API keys, got user=%q ok=%v", gotUser, ok)
	}
	// 14900 paise, not 149 rupees and not 1490000. The plan catalogue stores
	// minor units and Razorpay wants minor units, so nothing converts.
	if captured["amount"] != float64(14900) {
		t.Fatalf("amount must be sent in paise unchanged, got %#v", captured["amount"])
	}
	if captured["currency"] != "INR" {
		t.Fatalf("currency must be upper-cased, got %#v", captured["currency"])
	}
}

func TestCreateOrderTruncatesOverlongReceipt(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"id":"order_TEST"}`))
	}))
	defer server.Close()

	client := NewClient(RazorpayConfig{KeyID: "k", KeySecret: "s", WebhookSecret: "w", BaseURL: server.URL})
	if _, err := client.CreateOrder(context.Background(), CreateOrderRequest{
		AmountMinor: 100,
		Receipt:     strings.Repeat("x", 80),
	}); err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	// Razorpay rejects the whole order past 40 characters, which would surface
	// to the user as an opaque checkout failure.
	if receipt, _ := captured["receipt"].(string); len(receipt) != 40 {
		t.Fatalf("receipt should be truncated to 40 chars, got %d", len(receipt))
	}
}

func TestCreateOrderSurfacesProviderReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"BAD_REQUEST_ERROR","description":"amount must be atleast INR 1.00"}}`))
	}))
	defer server.Close()

	client := NewClient(RazorpayConfig{KeyID: "k", KeySecret: "s", WebhookSecret: "w", BaseURL: server.URL})
	_, err := client.CreateOrder(context.Background(), CreateOrderRequest{AmountMinor: 1})
	if err == nil {
		t.Fatal("a 400 from the provider must be an error")
	}
	if !strings.Contains(err.Error(), "amount must be atleast INR 1.00") {
		t.Fatalf("the provider's reason must reach the log: %v", err)
	}
}

func TestCreateOrderRefusesWithoutConfiguration(t *testing.T) {
	client := NewClient(RazorpayConfig{})
	if _, err := client.CreateOrder(context.Background(), CreateOrderRequest{AmountMinor: 100}); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestCreateOrderRejectsNonPositiveAmount(t *testing.T) {
	client := NewClient(RazorpayConfig{KeyID: "k", KeySecret: "s", WebhookSecret: "w"})
	if _, err := client.CreateOrder(context.Background(), CreateOrderRequest{AmountMinor: 0}); err == nil {
		t.Fatal("a zero-amount order must be refused before any network call")
	}
}

func TestParseWebhookReadsCaptureEntity(t *testing.T) {
	body := []byte(`{
		"entity":"event",
		"event":"payment.captured",
		"payload":{"payment":{"entity":{
			"id":"pay_123","order_id":"order_456","status":"captured",
			"amount":14900,"currency":"INR","method":"upi",
			"notes":{"plan_code":"monthly","user_id":"7"}
		}}}
	}`)
	envelope, err := ParseWebhook(body)
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	payment := envelope.Payload.Payment.Entity
	if envelope.Event != "payment.captured" || payment.ID != "pay_123" || payment.OrderID != "order_456" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if payment.AmountMinor != 14900 || payment.Method != "upi" {
		t.Fatalf("unexpected payment entity: %#v", payment)
	}
	if payment.Notes["plan_code"] != "monthly" {
		t.Fatalf("notes did not decode: %#v", payment.Notes)
	}
}

// TestParseWebhookToleratesArrayNotes covers a real Razorpay quirk: with no
// notes set it sends an empty JSON array, not an object. A strict
// map[string]string decode fails on that, which would reject every note-free
// event — meaning every event, since notes are optional.
func TestParseWebhookToleratesArrayNotes(t *testing.T) {
	body := []byte(`{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_1","order_id":"order_1","notes":[]}}}}`)
	envelope, err := ParseWebhook(body)
	if err != nil {
		t.Fatalf("empty array notes must not fail the parse: %v", err)
	}
	if envelope.Payload.Payment.Entity.ID != "pay_1" {
		t.Fatalf("payment entity lost: %#v", envelope)
	}
}

func TestParseWebhookRejectsNamelessEvent(t *testing.T) {
	if _, err := ParseWebhook([]byte(`{"payload":{}}`)); err == nil {
		t.Fatal("an event with no name must be rejected")
	}
	if _, err := ParseWebhook([]byte(`not json`)); err == nil {
		t.Fatal("a non-JSON body must be rejected")
	}
}

func TestParseWebhookReadsRefundEntity(t *testing.T) {
	body := []byte(`{"event":"refund.processed","payload":{"refund":{"entity":{"id":"rfnd_1","payment_id":"pay_1","amount":14900,"status":"processed"}}}}`)
	envelope, err := ParseWebhook(body)
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	refund := envelope.Payload.Refund.Entity
	if refund.PaymentID != "pay_1" || refund.AmountMinor != 14900 {
		t.Fatalf("unexpected refund entity: %#v", refund)
	}
}
