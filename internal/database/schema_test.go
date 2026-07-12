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
		"split_bills_total_amount_positive_check",
		"split_participants_direction_check",
		"split_settlements_direction_check",
		"INSERT INTO accounts",
		"AND entries.account_id IS NULL",
		"SET source = 'manual'",
	}
	for _, fragment := range required {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("runtime schema is missing %q", fragment)
		}
	}

	accountBackfill := strings.Index(joined, "AND entries.account_id IS NULL")
	accountNotNull := strings.Index(joined, "ALTER COLUMN account_id SET NOT NULL")
	if accountBackfill < 0 || accountNotNull < 0 || accountBackfill > accountNotNull {
		t.Fatal("runtime schema must backfill null entry accounts before account_id is made non-null")
	}
}
