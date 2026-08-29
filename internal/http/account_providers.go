package http

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"finance-parser-go/internal/models"
)

type accountProvider = models.AccountProviderDetails

var accountProviders = []accountProvider{
	{ID: "hdfc", DisplayName: "HDFC Bank", TypeSupport: []string{"bank", "credit_card", "debit_card"}, AssetKey: "bank", Aliases: []string{"HDFC", "HDFC card"}},
	{ID: "icici", DisplayName: "ICICI Bank", TypeSupport: []string{"bank", "credit_card", "debit_card"}, AssetKey: "bank", Aliases: []string{"ICICI", "ICICI card"}},
	{ID: "sbi", DisplayName: "State Bank of India", TypeSupport: []string{"bank", "credit_card", "debit_card"}, AssetKey: "bank", Aliases: []string{"SBI", "SBI card", "SBI State Bank of India"}},
	{ID: "axis", DisplayName: "Axis Bank", TypeSupport: []string{"bank", "credit_card", "debit_card"}, AssetKey: "bank", Aliases: []string{"Axis", "Axis card"}},
	{ID: "kotak", DisplayName: "Kotak Mahindra Bank", TypeSupport: []string{"bank", "credit_card", "debit_card"}, AssetKey: "bank", Aliases: []string{"Kotak", "Kotak Bank"}},
	{ID: "amex", DisplayName: "American Express", TypeSupport: []string{"credit_card"}, AssetKey: "credit-card", Aliases: []string{"Amex", "American Express card"}},
	{ID: "hsbc", DisplayName: "HSBC", TypeSupport: []string{"bank", "credit_card", "debit_card"}, AssetKey: "bank", Aliases: []string{"HSBC Bank", "HSBC card"}},
	{ID: "standard-chartered", DisplayName: "Standard Chartered", TypeSupport: []string{"bank", "credit_card", "debit_card"}, AssetKey: "bank", Aliases: []string{"SCB", "Standard Chartered Bank"}},
	{ID: "citibank", DisplayName: "Citibank", TypeSupport: []string{"bank", "credit_card", "debit_card"}, AssetKey: "bank", Aliases: []string{"Citi", "Citi Bank", "Citi card"}},
	{ID: "google-pay", DisplayName: "Google Pay", TypeSupport: []string{"upi"}, AssetKey: "google", Aliases: []string{"GPay", "GooglePay"}},
	{ID: "phonepe", DisplayName: "PhonePe", TypeSupport: []string{"upi", "wallet"}, AssetKey: "alpha-p-circle", Aliases: []string{"Phone Pe", "PhonePe Wallet"}},
	{ID: "paytm", DisplayName: "Paytm", TypeSupport: []string{"upi", "wallet"}, AssetKey: "wallet", Aliases: []string{"Paytm UPI", "Paytm Wallet"}},
	{ID: "bhim", DisplayName: "BHIM", TypeSupport: []string{"upi"}, AssetKey: "qrcode-scan", Aliases: []string{"BHIM UPI"}},
	{ID: "amazon-pay", DisplayName: "Amazon Pay", TypeSupport: []string{"upi", "wallet"}, AssetKey: "amazon", Aliases: []string{"AmazonPay", "Amazon Pay UPI"}},
	{ID: "mobikwik", DisplayName: "MobiKwik", TypeSupport: []string{"wallet"}, AssetKey: "wallet", Aliases: []string{"Mobi Kwik"}},
}

func listAccountProviders(c *gin.Context) {
	requestedType := strings.TrimSpace(c.Query("type"))
	if requestedType != "" {
		requestedType = normalizeAccountType(requestedType)
		if requestedType == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_account_type"})
			return
		}
	}
	providers := make([]accountProvider, 0, len(accountProviders))
	for _, provider := range accountProviders {
		if requestedType == "" || providerSupportsType(provider, requestedType) {
			providers = append(providers, provider)
		}
	}
	sort.SliceStable(providers, func(i, j int) bool { return providers[i].DisplayName < providers[j].DisplayName })
	c.JSON(http.StatusOK, providers)
}

func accountProviderByID(id string) (accountProvider, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, provider := range accountProviders {
		if provider.ID == id {
			return provider, true
		}
	}
	return accountProvider{}, false
}

func normalizeProviderText(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return -1
	}, strings.TrimSpace(value))
}

func matchAccountProvider(value string) (accountProvider, bool) {
	needle := normalizeProviderText(value)
	if needle == "" {
		return accountProvider{}, false
	}
	for _, provider := range accountProviders {
		candidates := append([]string{provider.ID, provider.DisplayName}, provider.Aliases...)
		for _, candidate := range candidates {
			if normalizeProviderText(candidate) == needle {
				return provider, true
			}
		}
	}
	return accountProvider{}, false
}

func providerSupportsType(provider accountProvider, accountType string) bool {
	for _, supported := range provider.TypeSupport {
		if supported == accountType {
			return true
		}
	}
	return false
}

func hydrateAccountProvider(account *models.Account) {
	if account.ProviderID == "" {
		if provider, ok := matchAccountProvider(account.Provider); ok && providerSupportsType(provider, normalizeAccountType(account.Type)) {
			account.ProviderID = provider.ID
		}
	}
	if provider, ok := accountProviderByID(account.ProviderID); ok {
		copy := provider
		account.ProviderDetails = &copy
	}
	if account.Identifier == "" {
		account.Identifier = accountDisplayIdentifier(*account)
	}
}
