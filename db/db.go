package db

import (
	"context"
	"database/sql"
)

// Executor is an interface that wraps methods for executing SQL queries via sql.DB or sql.Tx.
type Executor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type Pagination struct {
	Limit int `json:"limit"`
	Page  int `json:"page"`
}

func (p *Pagination) Offset() int {
	return p.Limit * (p.Page - 1)
}
