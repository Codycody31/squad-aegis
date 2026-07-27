package db

import (
	"database/sql"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const postgresOptimizationBaselineVersion = 36

func TestPostgresDSN(t *testing.T) {
	dsn := PostgresDSN("2001:db8::1", 5432, "user@example.com", "p@ss/word", "squad aegis")
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse PostgresDSN: %v", err)
	}

	if got := parsed.Query().Get("application_name"); got != "squad-aegis" {
		t.Fatalf("application_name = %q, want squad-aegis", got)
	}
	if got := parsed.Query().Get("sslmode"); got != "disable" {
		t.Fatalf("sslmode = %q, want disable", got)
	}
	if got := parsed.User.Username(); got != "user@example.com" {
		t.Fatalf("username = %q, want user@example.com", got)
	}
	if got, _ := parsed.User.Password(); got != "p@ss/word" {
		t.Fatalf("password = %q, want p@ss/word", got)
	}
	if got := strings.TrimPrefix(parsed.EscapedPath(), "/"); got != "squad%20aegis" {
		t.Fatalf("database path = %q, want squad%%20aegis", got)
	}
}

func TestConcurrentMigrationsContainOneStatement(t *testing.T) {
	files, err := fs.Glob(Migrations, "migrations/*.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}

	for _, filename := range files {
		contents, err := Migrations.ReadFile(filename)
		if err != nil {
			t.Fatalf("read %s: %v", filename, err)
		}
		sqlText := string(contents)
		if !strings.Contains(sqlText, "CONCURRENTLY") {
			continue
		}
		if got := strings.Count(sqlText, "CONCURRENTLY"); got != 1 {
			t.Errorf("%s contains %d concurrent operations, want 1", filename, got)
		}
		if got := strings.Count(sqlText, ";"); got != 1 {
			t.Errorf("%s contains %d SQL statement terminators, want 1", filename, got)
		}
	}
}

func TestObservedPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	database, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Ping(); err != nil {
		t.Fatal(err)
	}
	rows, err := database.Query(`/* sa.test.postgres_observer */ SELECT generate_series(1, 3)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("observed PostgreSQL rows = %d, want 3", count)
	}
}

func TestPostgresMigrationsRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}

	database, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	defer database.Close()

	migrator, closeMigrator := newIntegrationMigrator(t, database)
	defer closeMigrator()

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("apply up migrations: %v", err)
	}
	latestVersion, _, err := migrator.Version()
	if err != nil {
		t.Fatalf("read latest migration version: %v", err)
	}

	if err := migrator.Migrate(postgresOptimizationBaselineVersion); err != nil {
		t.Fatalf("roll back optimization migrations: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("reapply optimization migrations: %v", err)
	}
	reappliedVersion, dirty, err := migrator.Version()
	if err != nil {
		t.Fatalf("read reapplied migration version: %v", err)
	}
	if dirty {
		t.Fatal("reapplied migration version is dirty")
	}
	if reappliedVersion != latestVersion {
		t.Fatalf("reapplied version = %d, want %d", reappliedVersion, latestVersion)
	}

	if err := migrator.Migrate(48); err != nil {
		t.Fatalf("roll back to execution ID preflight: %v", err)
	}
	insertDuplicateWorkflowExecutionIDs(t, database)
	err = migrator.Steps(1)
	if err == nil {
		t.Fatal("duplicate execution IDs passed preflight")
	}
	if !strings.Contains(err.Error(), "cannot create unique workflow execution index") {
		t.Fatalf("preflight error %q does not explain duplicate execution IDs", err)
	}
}

func newIntegrationMigrator(t *testing.T, database *sql.DB) (*migrate.Migrate, func()) {
	t.Helper()

	driver, err := postgres.WithInstance(database, &postgres.Config{})
	if err != nil {
		t.Fatalf("create PostgreSQL migration driver: %v", err)
	}
	source, err := iofs.New(Migrations, "migrations")
	if err != nil {
		t.Fatalf("create migration source: %v", err)
	}
	migrator, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}

	return migrator, func() {
		sourceErr, databaseErr := migrator.Close()
		if sourceErr != nil {
			t.Errorf("close migration source: %v", sourceErr)
		}
		if databaseErr != nil {
			t.Errorf("close migration database: %v", databaseErr)
		}
	}
}

func insertDuplicateWorkflowExecutionIDs(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec(`
		INSERT INTO public.server_workflows (
			id, server_id, name, enabled, definition, created_by
		) VALUES (
			'10000000-0000-0000-0000-000000000001',
			'20000000-0000-0000-0000-000000000001',
			'preflight test',
			TRUE,
			'{}'::jsonb,
			'30000000-0000-0000-0000-000000000001'
		);

		INSERT INTO public.server_workflow_executions (
			id, workflow_id, execution_id, status
		) VALUES
			(
				'40000000-0000-0000-0000-000000000001',
				'10000000-0000-0000-0000-000000000001',
				'50000000-0000-0000-0000-000000000001',
				'RUNNING'
			),
			(
				'40000000-0000-0000-0000-000000000002',
				'10000000-0000-0000-0000-000000000001',
				'50000000-0000-0000-0000-000000000001',
				'RUNNING'
			);
	`)
	if err != nil {
		t.Fatalf("insert duplicate workflow execution IDs: %v", err)
	}
}
