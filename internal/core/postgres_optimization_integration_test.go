package core_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/permissions"
)

func TestPostgresCoreOptimizations(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_CORE_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_CORE_DSN is not set")
	}

	database, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	defer database.Close()
	if err := db.Migrate(database, false); err != nil {
		t.Fatalf("migrate PostgreSQL: %v", err)
	}
	insertCoreOptimizationFixtures(t, database)

	ctx := context.Background()
	serverID := uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	regularUserID := uuid.MustParse("11111111-0000-0000-0000-000000000001")
	unauthorizedUserID := uuid.MustParse("22222222-0000-0000-0000-000000000001")
	superUserID := uuid.MustParse("33333333-0000-0000-0000-000000000001")

	executor := &countingExecutor{DB: database}
	server, err := core.GetServerById(ctx, executor, serverID, &models.User{Id: regularUserID})
	if err != nil {
		t.Fatalf("GetServerById authorized: %v", err)
	}
	if !server.Enabled {
		t.Fatal("GetServerById did not scan enabled")
	}
	if executor.queryRowCalls != 1 || executor.queryCalls != 0 {
		t.Fatalf("GetServerById calls = QueryRow:%d Query:%d, want 1 and 0", executor.queryRowCalls, executor.queryCalls)
	}

	_, err = core.GetServerById(ctx, database, serverID, &models.User{Id: unauthorizedUserID})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetServerById unauthorized error = %v, want sql.ErrNoRows", err)
	}

	executor.reset()
	serverPermissions, err := core.GetUserServerPermissions(ctx, executor, regularUserID)
	if err != nil {
		t.Fatalf("GetUserServerPermissions regular user: %v", err)
	}
	if executor.queryCalls != 1 || executor.queryRowCalls != 0 {
		t.Fatalf("GetUserServerPermissions calls = Query:%d QueryRow:%d, want 1 and 0", executor.queryCalls, executor.queryRowCalls)
	}
	gotPermissions := serverPermissions[serverID.String()]
	wantPermissions := []string{"ui:dashboard:view", "ui:metrics:view"}
	if !slices.Equal(gotPermissions, wantPermissions) {
		t.Fatalf("regular permissions = %v, want %v", gotPermissions, wantPermissions)
	}

	superPermissions, err := core.GetUserServerPermissions(ctx, database, superUserID)
	if err != nil {
		t.Fatalf("GetUserServerPermissions super administrator: %v", err)
	}
	if len(superPermissions) != 2 {
		t.Fatalf("super administrator server count = %d, want 2", len(superPermissions))
	}
	for id, permissionCodes := range superPermissions {
		if !slices.Equal(permissionCodes, []string{"*"}) {
			t.Fatalf("super administrator permissions for %s = %v, want [*]", id, permissionCodes)
		}
	}

	_, err = core.GetUserServerPermissions(
		ctx,
		database,
		uuid.MustParse("99999999-0000-0000-0000-000000000001"),
	)
	if !errors.Is(err, core.ErrorUserNotFound) {
		t.Fatalf("missing user error = %v, want ErrorUserNotFound", err)
	}

	repository := permissions.NewRepository(database)
	templates, err := repository.GetRoleTemplatesWithPermissions(ctx)
	if err != nil {
		t.Fatalf("GetRoleTemplatesWithPermissions: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("GetRoleTemplatesWithPermissions returned no seeded templates")
	}

	childRoleID := uuid.MustParse("cccccccc-0000-0000-0000-000000000001")
	if err := repository.SetServerRolePermissions(
		ctx,
		childRoleID,
		[]string{"ui:dashboard:view", "ui:metrics:view"},
	); err != nil {
		t.Fatalf("SetServerRolePermissions: %v", err)
	}
	var permissionCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM server_role_permissions WHERE server_role_id = $1",
		childRoleID,
	).Scan(&permissionCount); err != nil {
		t.Fatalf("count set-based role permissions: %v", err)
	}
	if permissionCount != 2 {
		t.Fatalf("set-based role permission count = %d, want 2", permissionCount)
	}

	assertAuditKeysetPlans(t, database, serverID)
}

type explainDocument struct {
	Plan explainPlanNode `json:"Plan"`
}

type explainPlanNode struct {
	NodeType            string            `json:"Node Type"`
	IndexName           string            `json:"Index Name"`
	ActualRows          float64           `json:"Actual Rows"`
	RowsRemovedByFilter float64           `json:"Rows Removed by Filter"`
	Plans               []explainPlanNode `json:"Plans"`
}

func assertAuditKeysetPlans(t *testing.T, database *sql.DB, serverID uuid.UUID) {
	t.Helper()
	if _, err := database.Exec(`
		INSERT INTO audit_logs (id, server_id, action, changes, timestamp, username_snapshot)
		SELECT
			md5(g::text || fixture_server.id::text)::uuid,
			fixture_server.id,
			'fixture.action',
			'{}'::jsonb,
			TIMESTAMP '2026-01-01 00:00:00' + g * INTERVAL '1 millisecond',
			'fixture-user'
		FROM generate_series(1, 100000) AS g
		CROSS JOIN (VALUES ($1::uuid), ($2::uuid)) AS fixture_server(id)
	`, serverID, uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000002")); err != nil {
		t.Fatalf("insert audit plan fixture: %v", err)
	}
	if _, err := database.Exec("ANALYZE audit_logs"); err != nil {
		t.Fatal(err)
	}

	var cursorTime time.Time
	var cursorID uuid.UUID
	if err := database.QueryRow(`
		SELECT timestamp, id
		FROM audit_logs
		WHERE server_id = $1
		ORDER BY timestamp DESC, id DESC
		OFFSET 90000 LIMIT 1
	`, serverID).Scan(&cursorTime, &cursorID); err != nil {
		t.Fatalf("select audit cursor fixture: %v", err)
	}

	assertPlan := func(indexNames []string, query string, args ...interface{}) {
		t.Helper()
		var raw []byte
		if err := database.QueryRow(query, args...).Scan(&raw); err != nil {
			t.Fatalf("explain audit keyset query: %v", err)
		}
		var documents []explainDocument
		if err := json.Unmarshal(raw, &documents); err != nil {
			t.Fatalf("decode audit keyset plan: %v", err)
		}
		if len(documents) != 1 {
			t.Fatalf("audit explain documents = %d, want 1", len(documents))
		}
		var matching *explainPlanNode
		var visit func(*explainPlanNode)
		visit = func(node *explainPlanNode) {
			for _, indexName := range indexNames {
				if node.IndexName == indexName {
					matching = node
				}
			}
			for i := range node.Plans {
				visit(&node.Plans[i])
			}
		}
		visit(&documents[0].Plan)
		if matching == nil {
			t.Fatalf("audit keyset plan did not use one of %v: %s", indexNames, raw)
		}
		scanned := matching.ActualRows + matching.RowsRemovedByFilter
		if scanned > 102 {
			t.Fatalf("audit keyset index scanned %.0f rows, want at most twice the 51-row request", scanned)
		}
	}

	assertPlan([]string{"idx_audit_logs_server_cursor", "idx_audit_logs_cursor"}, `
		EXPLAIN (ANALYZE, FORMAT JSON)
		SELECT id, timestamp
		FROM audit_logs
		WHERE server_id = $1 AND (timestamp, id) < ($2, $3)
		ORDER BY timestamp DESC, id DESC
		LIMIT 51
	`, serverID, cursorTime, cursorID)
	assertPlan([]string{"idx_audit_logs_cursor"}, `
		EXPLAIN (ANALYZE, FORMAT JSON)
		SELECT id, timestamp
		FROM audit_logs
		WHERE (timestamp, id) < ($1, $2)
		ORDER BY timestamp DESC, id DESC
		LIMIT 51
	`, cursorTime, cursorID)
}

type countingExecutor struct {
	*sql.DB
	queryCalls    int
	queryRowCalls int
}

func (e *countingExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	e.queryCalls++
	return e.DB.QueryContext(ctx, query, args...)
}

func (e *countingExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	e.queryRowCalls++
	return e.DB.QueryRowContext(ctx, query, args...)
}

func (e *countingExecutor) reset() {
	e.queryCalls = 0
	e.queryRowCalls = 0
}

func insertCoreOptimizationFixtures(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec(`
		INSERT INTO public.servers (
			id, name, ip_address, game_port, rcon_port, rcon_password, created_at, updated_at
		) VALUES
			(
				'aaaaaaaa-0000-0000-0000-000000000001',
				'Server A',
				'127.0.0.1',
				7787,
				21114,
				'secret',
				NOW(),
				NOW()
			),
			(
				'aaaaaaaa-0000-0000-0000-000000000002',
				'Server B',
				'127.0.0.2',
				7788,
				21115,
				'secret',
				NOW(),
				NOW()
			);

		INSERT INTO public.users (
			id, name, username, password, super_admin, created_at, updated_at
		) VALUES
			(
				'11111111-0000-0000-0000-000000000001',
				'Regular',
				'regular',
				'password',
				FALSE,
				NOW(),
				NOW()
			),
			(
				'22222222-0000-0000-0000-000000000001',
				'Unauthorized',
				'unauthorized',
				'password',
				FALSE,
				NOW(),
				NOW()
			),
			(
				'33333333-0000-0000-0000-000000000001',
				'Super',
				'super',
				'password',
				TRUE,
				NOW(),
				NOW()
			);

		INSERT INTO public.server_roles (
			id, server_id, name, permissions, created_at
		) VALUES
			(
				'cccccccc-0000-0000-0000-000000000001',
				'aaaaaaaa-0000-0000-0000-000000000001',
				'Child',
				'',
				NOW()
			),
			(
				'dddddddd-0000-0000-0000-000000000001',
				'aaaaaaaa-0000-0000-0000-000000000001',
				'Parent',
				'',
				NOW()
			);

		INSERT INTO public.server_admins (
			id, server_id, user_id, server_role_id, created_at
		) VALUES (
			'eeeeeeee-0000-0000-0000-000000000001',
			'aaaaaaaa-0000-0000-0000-000000000001',
			'11111111-0000-0000-0000-000000000001',
			'cccccccc-0000-0000-0000-000000000001',
			NOW()
		);

		INSERT INTO public.role_inheritance (
			parent_role_id, child_role_id
		) VALUES (
			'dddddddd-0000-0000-0000-000000000001',
			'cccccccc-0000-0000-0000-000000000001'
		);

		INSERT INTO public.server_role_permissions (server_role_id, permission_id)
		SELECT
			CASE p.code
				WHEN 'ui:dashboard:view' THEN 'cccccccc-0000-0000-0000-000000000001'::uuid
				ELSE 'dddddddd-0000-0000-0000-000000000001'::uuid
			END,
			p.id
		FROM public.permissions AS p
		WHERE p.code IN ('ui:dashboard:view', 'ui:metrics:view');
	`)
	if err != nil {
		t.Fatalf("insert core optimization fixtures: %v", err)
	}
}
