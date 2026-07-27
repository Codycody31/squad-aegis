package clickhouse

import (
	"fmt"
	"strings"
	"testing"
)

func TestOptimizationMigrationsHaveDownMigrations(t *testing.T) {
	for version := 18; version <= 24; version++ {
		pattern := fmt.Sprintf("migrations/%06d_", version)
		entries, err := Migrations.ReadDir("migrations")
		if err != nil {
			t.Fatal(err)
		}

		var up, down bool
		for _, entry := range entries {
			name := "migrations/" + entry.Name()
			if !strings.HasPrefix(name, pattern) {
				continue
			}
			up = up || strings.HasSuffix(name, ".up.sql")
			down = down || strings.HasSuffix(name, ".down.sql")
		}
		if !up || !down {
			t.Errorf("migration %06d: up=%t down=%t", version, up, down)
		}
	}
}

func TestStartupMigrationsDoNotRunHeavyMaintenance(t *testing.T) {
	entries, err := Migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		body, err := Migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		sqlText := strings.ToUpper(string(body))
		if strings.Contains(sqlText, "MATERIALIZE INDEX") {
			t.Errorf("%s materializes a skip index during startup", entry.Name())
		}
		if strings.Contains(sqlText, "OPTIMIZE TABLE") && strings.Contains(sqlText, " FINAL") {
			t.Errorf("%s runs OPTIMIZE FINAL", entry.Name())
		}
	}
}

func TestStableKeyMigrationsDoNotDependOnVirtualRandomUUIDs(t *testing.T) {
	for _, name := range []string{
		"migrations/000019_workflow_stable_entry_ids.up.sql",
		"migrations/000020_gameplay_stable_keys.up.sql",
	} {
		body, err := Migrations.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		sqlText := string(body)
		if !strings.Contains(sqlText, "reinterpretAsUUID(MD5(concat(") {
			t.Errorf("%s does not define an MD5-derived UUID", name)
		}
		if strings.Contains(sqlText, "generateUUIDv4()") {
			t.Errorf("%s uses a random UUID for a historical cursor/evidence key", name)
		}
	}
}
