package http

import (
	"path/filepath"
	"testing"

	"finance-parser-go/internal/models"
)

func TestDeleteUserSkipStaticBearerGate(t *testing.T) {
	if !skipsStaticBearer("/v1/user") {
		t.Fatal("user deletion should use user-session auth, not the static bearer gate")
	}
}

func TestVerificationIdentifiersForUser(t *testing.T) {
	email := " USER@Example.COM "
	phone := " +919876543210 "
	identifiers := verificationIdentifiersForUser(models.User{Email: &email, Phone: &phone})

	if len(identifiers) != 2 {
		t.Fatalf("expected two identifiers, got %v", identifiers)
	}
	if identifiers[0].identifierType != "email" || identifiers[0].identifier != "user@example.com" {
		t.Fatalf("unexpected email identifier: %#v", identifiers[0])
	}
	if identifiers[1].identifierType != "phone" || identifiers[1].identifier != "+919876543210" {
		t.Fatalf("unexpected phone identifier: %#v", identifiers[1])
	}
}

func TestLocalUploadPathFromAttachment(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "relative upload", raw: "uploads/receipt.png", want: filepath.FromSlash("uploads/receipt.png"), ok: true},
		{name: "absolute upload URL", raw: "https://api.example.com/uploads/receipt.png", want: filepath.FromSlash("uploads/receipt.png"), ok: true},
		{name: "nested upload", raw: "/uploads/2026/receipt.png", want: filepath.FromSlash("uploads/2026/receipt.png"), ok: true},
		{name: "empty", raw: " ", ok: false},
		{name: "directory only", raw: "uploads", ok: false},
		{name: "path traversal", raw: "uploads/../secret.txt", ok: false},
		{name: "outside uploads", raw: "/tmp/uploads/receipt.png", ok: false},
	}

	for _, tt := range tests {
		got, ok := localUploadPathFromAttachment(tt.raw)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("%s: got path=%q ok=%v, want path=%q ok=%v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

func TestDeleteUserDataRejectsMissingUserID(t *testing.T) {
	if _, err := deleteUserData(nil, models.User{}); err == nil {
		t.Fatal("expected missing user id to be rejected before database access")
	}
}
