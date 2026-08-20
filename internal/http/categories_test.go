package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The canonical list, the JSON schema the parser response is validated against,
// and the prompt the model is given are three separate files. They drifted once:
// the app grew "Food & Drinks" and "Transport" while the schema and prompt kept
// the original six, so the parser physically could not emit either value. These
// tests fail the build if they ever diverge again.

func TestCanonicalCategoriesMatchParseSchema(t *testing.T) {
	path := filepath.Join(projectRoot(t), "schemas", "expense_entry.schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read parse schema: %v", err)
	}

	var schema struct {
		Properties struct {
			Category struct {
				Enum []any `json:"enum"`
			} `json:"category"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("failed to decode parse schema: %v", err)
	}

	var schemaCategories []string
	nullable := false
	for _, value := range schema.Properties.Category.Enum {
		if value == nil {
			nullable = true
			continue
		}
		text, ok := value.(string)
		if !ok {
			t.Fatalf("unexpected category enum member %#v", value)
		}
		schemaCategories = append(schemaCategories, text)
	}
	if !nullable {
		t.Error("parse schema should still allow a null category for non-transaction text")
	}

	if strings.Join(schemaCategories, "|") != strings.Join(canonicalCategories, "|") {
		t.Fatalf("parse schema categories drifted from canonicalCategories\n schema: %v\n  code:  %v",
			schemaCategories, canonicalCategories)
	}
}

func TestCanonicalCategoriesMatchPrompt(t *testing.T) {
	path := filepath.Join(projectRoot(t), "internal", "ai", "prompt.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read parser prompt: %v", err)
	}

	want := `"category": "` + strings.Join(canonicalCategories, "|") + `|null",`
	if !strings.Contains(string(raw), want) {
		t.Fatalf("parser prompt does not declare the canonical categories; expected a line containing:\n%s", want)
	}
}

func TestCanonicalCategoryResolvesAliasesAndLegacyNames(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// The legacy name older app builds still send.
		{"Food", "Food & Drinks"},
		{"food & drinks", "Food & Drinks"},
		{"  GROCERIES  ", "Food & Drinks"},
		{"restaurant", "Food & Drinks"},

		// Everyday movement is Transport, not Travel.
		{"auto rickshaw", "Transport"},
		{"uber", "Transport"},
		{"metro", "Transport"},
		{"petrol", "Transport"},

		// Going somewhere is Travel.
		{"flight", "Travel"},
		{"hotel", "Travel"},
		{"airbnb", "Travel"},
		{"vacation", "Travel"},

		// Strays that reached the table before save-time validation existed.
		{"Finance", "Misc"},
		{"Split", "Misc"},

		// Exact canonical values pass through untouched.
		{"Family/Gifts", "Family/Gifts"},
		{"Bills", "Bills"},
	}

	for _, tc := range cases {
		got, ok := canonicalCategory(tc.in)
		if !ok {
			t.Errorf("canonicalCategory(%q) was not recognised", tc.in)
			continue
		}
		if got != tc.want {
			t.Errorf("canonicalCategory(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalCategoryRejectsUnknownValues(t *testing.T) {
	for _, value := range []string{"", "   ", "Crypto", "Pet Supplies", "Food and Beverages"} {
		if resolved, ok := canonicalCategory(value); ok {
			t.Errorf("canonicalCategory(%q) = %q, want rejection so the user is asked", value, resolved)
		}
	}
}

// The category picker has a "Custom category" field. Save-time validation has to
// canonicalize the names it knows without throwing away names people invent.
func TestCategoryForSaveKeepsCustomNamesAndCanonicalizesKnownOnes(t *testing.T) {
	canonicalized := map[string]string{
		"Food":          "Food & Drinks",
		"  uber  ":      "Transport",
		"Misc":          "Misc",
		"Family/Gifts":  "Family/Gifts",
		"Food & Drinks": "Food & Drinks",
	}
	for in, want := range canonicalized {
		got, ok := categoryForSave(in)
		if !ok || got != want {
			t.Errorf("categoryForSave(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}

	for _, custom := range []string{"Pet Care", "Tuition", "Crypto", "  Side Hustle  "} {
		got, ok := categoryForSave(custom)
		if !ok {
			t.Errorf("categoryForSave(%q) rejected a user-authored category", custom)
			continue
		}
		if got != strings.TrimSpace(custom) {
			t.Errorf("categoryForSave(%q) = %q, want it preserved verbatim", custom, got)
		}
	}

	if _, ok := categoryForSave("   "); ok {
		t.Error("categoryForSave should reject a blank category")
	}
	if _, ok := categoryForSave(strings.Repeat("x", maxCustomCategoryLength+1)); ok {
		t.Errorf("categoryForSave should reject names longer than %d characters", maxCustomCategoryLength)
	}
	if _, ok := categoryForSave(strings.Repeat("x", maxCustomCategoryLength)); !ok {
		t.Errorf("categoryForSave should accept names of exactly %d characters", maxCustomCategoryLength)
	}
}

func TestCategoryAliasesResolveToCanonicalMembers(t *testing.T) {
	allowed := map[string]bool{}
	for _, category := range canonicalCategories {
		allowed[category] = true
	}
	for alias, target := range categoryAliases {
		if !allowed[target] {
			t.Errorf("alias %q maps to %q, which is not a canonical category", alias, target)
		}
		if allowed[alias] {
			t.Errorf("alias %q duplicates a canonical category and is redundant", alias)
		}
	}
}

// The endpoint exists so that clients do not have to keep a list of their own.
// Both the mobile app and the web app kept one, and the web app's had drifted to
// a different eight values by the time anyone noticed. These tests fail if the
// served list stops matching the canonical one, which is the only thing that
// makes "read it from the API" better than "copy it and hope".
func TestCategoriesEndpointServesCanonicalList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	auth := performJSONRequest[AuthResponse](
		t, router, http.MethodPost, "/v1/auth/guest", "", map[string]string{
			"device_id": "categories-test-device",
		}, http.StatusOK,
	)

	response := performJSONRequest[CategoriesResponse](
		t, router, http.MethodGet, "/v1/categories", auth.Token, nil, http.StatusOK,
	)

	if strings.Join(response.Categories, "|") != strings.Join(canonicalCategories, "|") {
		t.Fatalf("served categories drifted from canonicalCategories\n served: %v\n  code:  %v",
			response.Categories, canonicalCategories)
	}
	if response.Default != defaultCategory {
		t.Fatalf("served default category = %q, want %q", response.Default, defaultCategory)
	}

	// Every served value must survive a round trip through the save path, or a
	// picker built from this list can still offer something the API rewrites.
	for _, category := range response.Categories {
		resolved, ok := categoryForSave(category)
		if !ok {
			t.Fatalf("served category %q is rejected by categoryForSave", category)
		}
		if resolved != category {
			t.Fatalf("served category %q is rewritten to %q on save", category, resolved)
		}
	}
}

func TestCategoriesEndpointRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/v1/categories", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /v1/categories status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
