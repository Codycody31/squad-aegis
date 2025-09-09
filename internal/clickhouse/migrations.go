package clickhouse

import (
	"database/sql"
	"embed"

	"github.com/rs/zerolog/log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/samber/oops"
)

//go:embed migrations/*.sql
var Migrations embed.FS

// migrationLogger implements the migrate.Logger interface.
type migrationsLogger struct {
	verbose bool
}

func (ml *migrationsLogger) Printf(format string, v ...any) {
	if len(format) > 0 && format[len(format)-1] == '\n' {
		format = format[:len(format)-1]
	}

	log.Trace().Msgf(format, v...)
}

func (ml *migrationsLogger) Verbose() bool {
	return ml.verbose
}

func Migrate(database *sql.DB, verbose bool) error {
	driver, err := clickhouse.WithInstance(database, &clickhouse.Config{
		MigrationsTable:       "migrations",
		MultiStatementEnabled: true,
	})
	if err != nil {
		return oops.Wrapf(err, "failed to create clickhouse driver")
	}

	// Create a source driver for embedded files
	d, err := iofs.New(Migrations, "migrations")
	if err != nil {
		return oops.Wrapf(err, "failed to create iofs driver")
	}

	// Create a new migration instance
	m, err := migrate.NewWithInstance("iofs", d, "clickhouse", driver)
	if err != nil {
		return oops.Wrapf(err, "failed to create migrate instance")
	}

	// Close the iofs, but not the postgres driver
	defer d.Close()

	// Set logger
	m.Log = &migrationsLogger{verbose: verbose}

	// Run migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return oops.Wrapf(err, "migration failed")
	}

	return nil
}
