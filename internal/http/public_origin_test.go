package http

import (
	"crypto/tls"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicOriginUsesForwardedProtoBehindTLSTerminatingProxy(t *testing.T) {
	// What Railway, Fly, Render, or nginx hand the container: plain HTTP with
	// the original scheme in X-Forwarded-Proto.
	request := httptest.NewRequest(nethttp.MethodPost, "/v1/upload", nil)
	request.Host = "finnri.up.railway.app"
	request.Header.Set("X-Forwarded-Proto", "https")

	if got := publicOrigin(request); got != "https://finnri.up.railway.app" {
		t.Fatalf("attachment URL would be stored as cleartext: got %q", got)
	}
}

func TestPublicOriginPrefersLeftmostForwardedValue(t *testing.T) {
	request := httptest.NewRequest(nethttp.MethodPost, "/v1/upload", nil)
	request.Host = "internal:8080"
	request.Header.Set("X-Forwarded-Proto", "https, http")
	request.Header.Set("X-Forwarded-Host", "finnri.up.railway.app, internal:8080")

	if got := publicOrigin(request); got != "https://finnri.up.railway.app" {
		t.Fatalf("unexpected origin: %q", got)
	}
}

func TestPublicOriginFallsBackToDirectConnection(t *testing.T) {
	plain := httptest.NewRequest(nethttp.MethodPost, "/v1/upload", nil)
	plain.Host = "localhost:8080"
	if got := publicOrigin(plain); got != "http://localhost:8080" {
		t.Fatalf("expected local http origin, got %q", got)
	}

	secure := httptest.NewRequest(nethttp.MethodPost, "/v1/upload", nil)
	secure.Host = "finnri.example.com"
	secure.TLS = &tls.ConnectionState{}
	if got := publicOrigin(secure); got != "https://finnri.example.com" {
		t.Fatalf("expected https origin for direct TLS, got %q", got)
	}
}
