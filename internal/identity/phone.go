package identity

import "unicode"

// NormalizePhone returns the India-first comparison key used for account and
// split identities. Provider prefixes, spaces, punctuation, and an optional
// country code are deliberately ignored; the final ten digits are the stable
// part users and their friends are both likely to enter.
func NormalizePhone(value string) string {
	digits := make([]rune, 0, len(value))
	for _, char := range value {
		if unicode.IsDigit(char) {
			digits = append(digits, char)
		}
	}
	if len(digits) < 10 {
		return ""
	}
	return string(digits[len(digits)-10:])
}
