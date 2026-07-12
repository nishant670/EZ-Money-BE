package database

import (
	"strings"
	"testing"
)

func TestRuntimeSchemaIncludesDataIntegrityConstraints(t *testing.T) {
	joined := strings.Join(runtimeSchemaStatements(), "\n")
	required := []string{
		"entries_amount_positive_check",
		"entries_type_check",
		"entries_source_check",
		"accounts_type_check",
		"accounts_credit_limit_non_negative_check",
		"fk_entries_owned_account",
		"FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id)",
	}
	for _, fragment := range required {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("runtime schema is missing %q", fragment)
		}
	}
}
