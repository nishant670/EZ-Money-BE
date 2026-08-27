package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"finance-parser-go/internal/models"
)

func stringPointer(value string) *string { return &value }

func TestAccountProviderAliasesResolveToStableID(t *testing.T) {
	for _, alias := range []string{"HDFC", "hdfc bank", "HDFC card"} {
		provider, ok := matchAccountProvider(alias)
		if !ok || provider.ID != "hdfc" {
			t.Fatalf("alias %q resolved to %#v, %v", alias, provider, ok)
		}
	}
}

func TestListAccountProvidersFiltersBySupportedType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/v1/account-providers", listAccountProviders)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/account-providers?type=upi", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var providers []accountProvider
	if err := json.Unmarshal(response.Body.Bytes(), &providers); err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Fatal("expected UPI providers")
	}
	for _, provider := range providers {
		if !providerSupportsType(provider, "upi") {
			t.Fatalf("non-UPI provider leaked into response: %#v", provider)
		}
	}
}

func TestAccountInputWritesStructuredMetadataAndLegacyFallback(t *testing.T) {
	input := accountInput{
		Type: "credit_card", Name: "Travel card", Provider: "HDFC card",
		Identifier: "1234", Last4: stringPointer("1234"),
	}
	if fields := input.validate(); len(fields) != 0 {
		t.Fatalf("unexpected fields %#v", fields)
	}
	account := models.Account{}
	input.apply(&account)
	if account.ProviderID != "hdfc" || account.Provider != "HDFC Bank" {
		t.Fatalf("provider not structured: %#v", account)
	}
	if account.Last4 != "1234" || account.Identifier != "1234" {
		t.Fatalf("metadata fallback not kept: %#v", account)
	}
}

func TestLegacyAccountEditUpdatesStructuredIdentifier(t *testing.T) {
	account := models.Account{Type: "credit_card", Last4: "1111", ProviderID: "hdfc", Provider: "HDFC Bank"}
	accountInput{Type: "credit_card", Name: "Card", Provider: "Custom issuer", Identifier: "9999"}.apply(&account)
	if account.Last4 != "9999" || account.ProviderID != "" || account.Provider != "Custom issuer" {
		t.Fatalf("legacy edit was not preserved: %#v", account)
	}
}

func TestAccountInputRejectsMetadataForWrongType(t *testing.T) {
	input := accountInput{Type: "wallet", Name: "Wallet", Last4: stringPointer("1234")}
	if input.validate()["last4"] == "" {
		t.Fatal("wallet last4 must be rejected")
	}
}
