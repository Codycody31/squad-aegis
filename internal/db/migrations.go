package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var Migrations embed.FS

// migrationLogger implements the migrate.Logger interface.
type migrationsLogger struct {
	verbose bool
}

func (ml *migrationsLogger) Printf(format string, v ...any) {
	log.Printf(format, v...)
}

func (ml *migrationsLogger) Verbose() bool {
	return ml.verbose
}

func PostgresDSN(host string, port int, user, pass, name string) string {
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   name,
	}
	query := dsn.Query()
	query.Set("application_name", "squad-aegis")
	query.Set("sslmode", "disable")
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func Migrate(database *sql.DB, verbose bool) error {
	// Create a driver for PostgreSQL
	driver, err := postgres.WithInstance(database, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %v", err)
	}

	// Create a source driver for embedded files
	d, err := iofs.New(Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create iofs driver: %v", err)
	}

	// Create a new migration instance
	m, err := migrate.NewWithInstance("iofs", d, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %v", err)
	}

	// Close the iofs, but not the postgres driver
	defer d.Close()

	// Set logger
	m.Log = &migrationsLogger{verbose: verbose}

	// Run migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %v", err)
	}

	return nil
}
