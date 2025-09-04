package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/rs/zerolog/log"
)

// Client represents a ClickHouse client
type Client struct {
	conn *sql.DB
	cfg  Config
}

// Config holds ClickHouse connection configuration
type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	Debug    bool
}

// NewClient creates a new ClickHouse client
func NewClient(cfg Config) (*Client, error) {
	// Set defaults
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 9000
	}
	if cfg.Database == "" {
		cfg.Database = "squad_aegis"
	}
	if cfg.Username == "" {
		cfg.Username = "default"
	}

	// Build connection options
	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 30 * time.Second,
		Debug:       cfg.Debug,
	}

	// Open connection
	conn := clickhouse.OpenDB(options)
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	log.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("database", cfg.Database).
		Msg("Connected to ClickHouse")

	return &Client{
		conn: conn,
		cfg:  cfg,
	}, nil
}

// Close closes the ClickHouse connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Ping tests the connection
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.PingContext(ctx)
}

// Exec executes a query without returning results
func (c *Client) Exec(ctx context.Context, query string, args ...interface{}) error {
	_, err := c.conn.ExecContext(ctx, query, args...)
	return err
}

// Query executes a query and returns rows
func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.conn.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row
func (c *Client) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return c.conn.QueryRowContext(ctx, query, args...)
}

// Begin starts a transaction
func (c *Client) Begin(ctx context.Context) (*sql.Tx, error) {
	return c.conn.BeginTx(ctx, nil)
}

// GetConnection returns the underlying SQL connection for advanced usage
func (c *Client) GetConnection() *sql.DB {
	return c.conn
}
