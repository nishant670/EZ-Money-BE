package models

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// GORM's snake_case converter mangles the EMI acronym: left to itself it
// pluralises CardEMIPlan to "card_em_iplans", which is not the table the
// migration creates. Nothing errors when that happens — AutoMigrate just makes
// a second, empty table next to the real one, and every read comes back empty
// while every write disappears into it.
//
// So the names are pinned on the models, and checked here against the
// migration that actually creates them.
func TestCardEMITableNamesMatchTheMigration(t *testing.T) {
	migration := readMigration(t, "0038_create_card_emi_plans.sql")

	for _, tableName := range []string{
		CardEMIPlan{}.TableName(),
		CardEMIInstallment{}.TableName(),
	} {
		if !strings.Contains(migration, "CREATE TABLE IF NOT EXISTS "+tableName+" (") {
			t.Errorf("model table %q is not created by the migration", tableName)
		}
	}

	if got := (CardEMIPlan{}).TableName(); got != "card_emi_plans" {
		t.Errorf("plan table = %q, want card_emi_plans", got)
	}
	if got := (CardEMIInstallment{}).TableName(); got != "card_emi_installments" {
		t.Errorf("installment table = %q, want card_emi_installments", got)
	}
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate the test file")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations", name)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", name, err)
	}
	return string(contents)
}
