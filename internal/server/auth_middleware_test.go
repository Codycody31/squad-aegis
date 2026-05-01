package server

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"go.codycody31.dev/squad-aegis/internal/models"
)

type authTestSQLDriver struct {
	queryContext func(query string, args []driver.NamedValue) (driver.Rows, error)
	execContext  func(query string, args []driver.NamedValue) (driver.Result, error)
}

func (d *authTestSQLDriver) Open(string) (driver.Conn, error) {
	return &authTestSQLConn{driver: d}, nil
}

type authTestSQLConn struct {
	driver *authTestSQLDriver
}

func (c *authTestSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare is not implemented in authTestSQLConn")
}

func (c *authTestSQLConn) Close() error {
	return nil
}

func (c *authTestSQLConn) Begin() (driver.Tx, error) {
	return &authTestSQLTx{}, nil
}

func (c *authTestSQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.driver.queryContext == nil {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	return c.driver.queryContext(query, args)
}

func (c *authTestSQLConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.driver.execContext == nil {
		return nil, fmt.Errorf("unexpected exec: %s", query)
	}
	return c.driver.execContext(query, args)
}

func (c *authTestSQLConn) CheckNamedValue(*driver.NamedValue) error {
	return nil
}

type authTestSQLTx struct{}

func (tx *authTestSQLTx) Commit() error   { return nil }
func (tx *authTestSQLTx) Rollback() error { return nil }

type authTestSQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *authTestSQLRows) Columns() []string {
	return r.columns
}

func (r *authTestSQLRows) Close() error {
	return nil
}

func (r *authTestSQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}

	for i := range dest {
		dest[i] = nil
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

var authTestSQLDriverSeq uint64

func openAuthTestDB(t *testing.T, driverImpl *authTestSQLDriver) *sql.DB {
	t.Helper()

	driverName := fmt.Sprintf("server-auth-test-%d", atomic.AddUint64(&authTestSQLDriverSeq, 1))
	sql.Register(driverName, driverImpl)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func TestAuthSessionAllowsQueryTokenForStorageFileDownloads(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessionID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	db := openAuthTestDB(t, &authTestSQLDriver{
		queryContext: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if len(args) != 1 {
				t.Fatalf("len(args) = %d, want 1", len(args))
			}
			if got, want := fmt.Sprint(args[0].Value), "download-token"; got != want {
				t.Fatalf("session token = %q, want %q", got, want)
			}
			return &authTestSQLRows{
				columns: []string{"id", "user_id", "token", "created_at", "expires_at", "last_seen", "last_seen_ip"},
				values: [][]driver.Value{{
					sessionID.String(),
					userID.String(),
					"download-token",
					now,
					nil,
					now,
					"127.0.0.1",
				}},
			}, nil
		},
		execContext: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if len(args) != 2 {
				t.Fatalf("len(args) = %d, want 2", len(args))
			}
			if got, want := fmt.Sprint(args[1].Value), sessionID.String(); got != want {
				t.Fatalf("updated session id = %q, want %q", got, want)
			}
			return driver.RowsAffected(1), nil
		},
	})

	server := &Server{Dependencies: &Dependencies{DB: db}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/sudo/storage/files/logs/server.log?token=download-token", nil)

	server.authSession(c, true)

	sessionValue, exists := c.Get("session")
	if !exists {
		t.Fatal("session missing from context")
	}
	session, ok := sessionValue.(*models.Session)
	if !ok {
		t.Fatalf("session type = %T, want *models.Session", sessionValue)
	}
	if got, want := session.Token, "download-token"; got != want {
		t.Fatalf("session.Token = %q, want %q", got, want)
	}
}

func TestAuthSessionRejectsQueryTokenForStorageListing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openAuthTestDB(t, &authTestSQLDriver{
		queryContext: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if len(args) != 1 {
				t.Fatalf("len(args) = %d, want 1", len(args))
			}
			if got := fmt.Sprint(args[0].Value); got != "" {
				t.Fatalf("session token = %q, want empty string", got)
			}
			return &authTestSQLRows{
				columns: []string{"id", "user_id", "token", "created_at", "expires_at", "last_seen", "last_seen_ip"},
				values:  nil,
			}, nil
		},
		execContext: func(query string, args []driver.NamedValue) (driver.Result, error) {
			t.Fatalf("unexpected exec: %s", query)
			return nil, nil
		},
	})

	server := &Server{Dependencies: &Dependencies{DB: db}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/sudo/storage/files?token=list-token", nil)

	server.authSession(c, true)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if _, exists := c.Get("session"); exists {
		t.Fatal("session unexpectedly present in context")
	}
}
