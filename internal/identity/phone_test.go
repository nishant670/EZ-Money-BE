package identity

import "testing"

func TestNormalizePhoneUsesLastTenDigits(t *testing.T) {
	tests := map[string]string{
		"9871801518":        "9871801518",
		"+919871801518":     "9871801518",
		"+91 98718 01518":   "9871801518",
		"0091-98718-01518":  "9871801518",
		"not enough digits": "",
	}
	for input, want := range tests {
		if got := NormalizePhone(input); got != want {
			t.Fatalf("NormalizePhone(%q) = %q, want %q", input, got, want)
		}
	}
}
