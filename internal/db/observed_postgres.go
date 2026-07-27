package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// OpenPostgres opens lib/pq behind a small driver wrapper that records stable
// query labels, latency, failures, and returned row counts. Argument values and
// full SQL text are never logged.
func OpenPostgres(dsn string) (*sql.DB, error) {
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(&observedConnector{connector: connector}), nil
}

type observedConnector struct {
	connector driver.Connector
}

func (c *observedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &observedConn{conn: conn}, nil
}

func (c *observedConnector) Driver() driver.Driver { return c.connector.Driver() }

type observedConn struct {
	conn driver.Conn
}

func (c *observedConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &observedStmt{stmt: stmt, label: postgresQueryLabel(query)}, nil
}

func (c *observedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	preparer, ok := c.conn.(driver.ConnPrepareContext)
	if !ok {
		return c.Prepare(query)
	}
	stmt, err := preparer.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &observedStmt{stmt: stmt, label: postgresQueryLabel(query)}, nil
}

func (c *observedConn) Close() error { return c.conn.Close() }

func (c *observedConn) Begin() (driver.Tx, error) { return c.conn.Begin() }

func (c *observedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	beginner, ok := c.conn.(driver.ConnBeginTx)
	if !ok {
		return nil, driver.ErrSkip
	}
	return beginner.BeginTx(ctx, opts)
}

func (c *observedConn) Ping(ctx context.Context) error {
	pinger, ok := c.conn.(driver.Pinger)
	if !ok {
		return driver.ErrSkip
	}
	return pinger.Ping(ctx)
}

func (c *observedConn) ResetSession(ctx context.Context) error {
	resetter, ok := c.conn.(driver.SessionResetter)
	if !ok {
		return nil
	}
	return resetter.ResetSession(ctx)
}

func (c *observedConn) IsValid() bool {
	validator, ok := c.conn.(driver.Validator)
	return !ok || validator.IsValid()
}

func (c *observedConn) CheckNamedValue(value *driver.NamedValue) error {
	checker, ok := c.conn.(driver.NamedValueChecker)
	if !ok {
		return driver.ErrSkip
	}
	return checker.CheckNamedValue(value)
}

func (c *observedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	started := time.Now()
	result, err := execer.ExecContext(ctx, query, args)
	rows := int64(0)
	if err == nil && result != nil {
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil {
			rows = affected
		}
	}
	logPostgresQuery(postgresQueryLabel(query), time.Since(started), err, rows)
	return result, err
}

func (c *observedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	started := time.Now()
	rows, err := queryer.QueryContext(ctx, query, args)
	if err != nil {
		logPostgresQuery(postgresQueryLabel(query), time.Since(started), err, 0)
		return nil, err
	}
	return newObservedDriverRows(rows, postgresQueryLabel(query), started), nil
}

type observedStmt struct {
	stmt  driver.Stmt
	label string
}

func (s *observedStmt) Close() error  { return s.stmt.Close() }
func (s *observedStmt) NumInput() int { return s.stmt.NumInput() }

func (s *observedStmt) Exec(args []driver.Value) (driver.Result, error) {
	started := time.Now()
	result, err := s.stmt.Exec(args)
	rows := int64(0)
	if err == nil && result != nil {
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil {
			rows = affected
		}
	}
	logPostgresQuery(s.label, time.Since(started), err, rows)
	return result, err
}

func (s *observedStmt) Query(args []driver.Value) (driver.Rows, error) {
	started := time.Now()
	rows, err := s.stmt.Query(args)
	if err != nil {
		logPostgresQuery(s.label, time.Since(started), err, 0)
		return nil, err
	}
	return newObservedDriverRows(rows, s.label, started), nil
}

func (s *observedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := s.stmt.(driver.StmtExecContext); ok {
		started := time.Now()
		result, err := execer.ExecContext(ctx, args)
		rows := int64(0)
		if err == nil && result != nil {
			if affected, affectedErr := result.RowsAffected(); affectedErr == nil {
				rows = affected
			}
		}
		logPostgresQuery(s.label, time.Since(started), err, rows)
		return result, err
	}
	values, err := namedValues(args)
	if err != nil {
		return nil, err
	}
	return s.Exec(values)
}

func (s *observedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := s.stmt.(driver.StmtQueryContext); ok {
		started := time.Now()
		rows, err := queryer.QueryContext(ctx, args)
		if err != nil {
			logPostgresQuery(s.label, time.Since(started), err, 0)
			return nil, err
		}
		return newObservedDriverRows(rows, s.label, started), nil
	}
	values, err := namedValues(args)
	if err != nil {
		return nil, err
	}
	return s.Query(values)
}

func namedValues(args []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		if arg.Name != "" {
			return nil, errors.New("named PostgreSQL parameters are not supported")
		}
		values[i] = arg.Value
	}
	return values, nil
}

type observedDriverRows struct {
	rows    driver.Rows
	label   string
	started time.Time
	count   atomic.Int64
	once    sync.Once
}

func newObservedDriverRows(rows driver.Rows, label string, started time.Time) *observedDriverRows {
	return &observedDriverRows{rows: rows, label: label, started: started}
}

func (r *observedDriverRows) Columns() []string { return r.rows.Columns() }

func (r *observedDriverRows) Close() error {
	err := r.rows.Close()
	r.finish(err)
	return err
}

func (r *observedDriverRows) Next(dest []driver.Value) error {
	err := r.rows.Next(dest)
	if err == nil {
		r.count.Add(1)
		return nil
	}
	if errors.Is(err, io.EOF) {
		r.finish(nil)
	} else {
		r.finish(err)
	}
	return err
}

func (r *observedDriverRows) HasNextResultSet() bool {
	rows, ok := r.rows.(driver.RowsNextResultSet)
	return ok && rows.HasNextResultSet()
}

func (r *observedDriverRows) NextResultSet() error {
	rows, ok := r.rows.(driver.RowsNextResultSet)
	if !ok {
		return io.EOF
	}
	err := rows.NextResultSet()
	if errors.Is(err, io.EOF) {
		r.finish(nil)
	} else if err != nil {
		r.finish(err)
	}
	return err
}

func (r *observedDriverRows) finish(err error) {
	r.once.Do(func() {
		logPostgresQuery(r.label, time.Since(r.started), err, r.count.Load())
	})
}

func postgresQueryLabel(query string) string {
	start := strings.Index(query, "/*")
	if start < 0 {
		return "unlabeled"
	}
	comment := query[start+2:]
	end := strings.Index(comment, "*/")
	if end < 0 {
		return "unlabeled"
	}
	label := strings.TrimSpace(comment[:end])
	if !strings.HasPrefix(label, "sa.") {
		return "unlabeled"
	}
	return label
}

func logPostgresQuery(label string, duration time.Duration, err error, rows int64) {
	event := log.Debug()
	if err != nil {
		event = log.Error().Err(err)
	} else if duration >= 500*time.Millisecond {
		event = log.Warn()
	}
	event.
		Str("query", label).
		Dur("duration", duration).
		Bool("failure", err != nil).
		Int64("rows", rows).
		Msg("database query")
}
