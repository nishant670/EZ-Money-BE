package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"finance-parser-go/internal/config"
)

func TestCORSMiddlewareAllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(cors(&config.Config{AllowOrigins: "https://app.finnri.com, http://localhost:3000"}))
	router.GET("/health", func(c *gin.Context) { c.JSON(nethttp.StatusOK, gin.H{"ok": true}) })

	request := httptest.NewRequest(nethttp.MethodGet, "/health", nil)
	request.Header.Set("Origin", "https://app.finnri.com")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://app.finnri.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := response.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q", got)
	}
}

func TestCORSMiddlewareRejectsDisallowedPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(cors(&config.Config{AllowOrigins: "https://app.finnri.com"}))
	router.POST("/v1/auth/login", func(c *gin.Context) { c.Status(nethttp.StatusOK) })

	request := httptest.NewRequest(nethttp.MethodOptions, "/v1/auth/login", nil)
	request.Header.Set("Origin", "https://evil.example")
	request.Header.Set("Access-Control-Request-Method", "POST")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != nethttp.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin should not receive CORS header, got %q", got)
	}
}

func TestCORSMiddlewareIgnoresWildcardConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(cors(&config.Config{AllowOrigins: "*"}))
	router.GET("/health", func(c *gin.Context) { c.JSON(nethttp.StatusOK, gin.H{"ok": true}) })

	request := httptest.NewRequest(nethttp.MethodGet, "/health", nil)
	request.Header.Set("Origin", "https://app.finnri.com")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("wildcard config should not grant CORS access, got %q", got)
	}
}
