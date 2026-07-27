package workflow_manager

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"go.codycody31.dev/squad-aegis/internal/db"
)

func TestWorkflowDatabaseBatchesVariablesAndUsesKeysets(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_WORKFLOW_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_WORKFLOW_DSN is not set")
	}

	database, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	defer database.Close()
	if err := db.Migrate(database, false); err != nil {
		t.Fatalf("migrate PostgreSQL: %v", err)
	}
	insertWorkflowDatabaseFixtures(t, database)

	ctx := context.Background()
	executor := &workflowCountingExecutor{DB: database}
	repository := NewWorkflowDatabase(executor)
	serverID := uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")

	workflows, err := repository.GetWorkflowsByServerID(ctx, serverID)
	if err != nil {
		t.Fatalf("GetWorkflowsByServerID: %v", err)
	}
	if executor.queryCalls != 2 {
		t.Fatalf("GetWorkflowsByServerID query count = %d, want 2", executor.queryCalls)
	}
	if len(workflows) != 2 {
		t.Fatalf("GetWorkflowsByServerID workflow count = %d, want 2", len(workflows))
	}
	for _, workflow := range workflows {
		if len(workflow.Variables) != 1 {
			t.Fatalf("workflow %s variable count = %d, want 1", workflow.ID, len(workflow.Variables))
		}
		if workflow.Variables[0].WorkflowID != workflow.ID {
			t.Fatalf(
				"workflow %s received variable for %s",
				workflow.ID,
				workflow.Variables[0].WorkflowID,
			)
		}
	}

	executor.reset()
	enabled, err := repository.GetEnabledWorkflows(ctx)
	if err != nil {
		t.Fatalf("GetEnabledWorkflows: %v", err)
	}
	if executor.queryCalls != 2 {
		t.Fatalf("GetEnabledWorkflows query count = %d, want 2", executor.queryCalls)
	}
	if len(enabled) != 2 {
		t.Fatalf("GetEnabledWorkflows workflow count = %d, want 2", len(enabled))
	}

	workflowID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")
	executor.reset()
	firstPage, err := repository.GetExecutionsByWorkflowID(ctx, workflowID, 2, nil, nil)
	if err != nil {
		t.Fatalf("GetExecutionsByWorkflowID first page: %v", err)
	}
	if executor.queryCalls != 1 {
		t.Fatalf("first execution page query count = %d, want 1", executor.queryCalls)
	}
	if len(firstPage) != 2 {
		t.Fatalf("first execution page length = %d, want 2", len(firstPage))
	}
	wantFirstIDs := []uuid.UUID{
		uuid.MustParse("dddddddd-0000-0000-0000-000000000003"),
		uuid.MustParse("dddddddd-0000-0000-0000-000000000002"),
	}
	for i := range wantFirstIDs {
		if firstPage[i].ID != wantFirstIDs[i] {
			t.Fatalf("first page ID %d = %s, want %s", i, firstPage[i].ID, wantFirstIDs[i])
		}
	}

	beforeStartedAt := firstPage[len(firstPage)-1].StartedAt
	beforeID := firstPage[len(firstPage)-1].ID
	// A row inserted after the first page must not shift the cursor window or
	// duplicate an already-returned row. All original fixtures intentionally
	// share a timestamp, which also exercises the UUID tie-breaker.
	if _, err := database.Exec(`
		INSERT INTO public.server_workflow_executions (
			id, workflow_id, execution_id, status, started_at
		) VALUES ($1, $2, $3, 'RUNNING', $4)
	`,
		uuid.MustParse("dddddddd-0000-0000-0000-000000000004"),
		workflowID,
		uuid.MustParse("ffffffff-0000-0000-0000-000000000004"),
		beforeStartedAt.Add(time.Minute),
	); err != nil {
		t.Fatalf("insert execution between pages: %v", err)
	}
	secondPage, err := repository.GetExecutionsByWorkflowID(
		ctx,
		workflowID,
		2,
		&beforeStartedAt,
		&beforeID,
	)
	if err != nil {
		t.Fatalf("GetExecutionsByWorkflowID second page: %v", err)
	}
	if len(secondPage) != 1 {
		t.Fatalf("second execution page length = %d, want 1", len(secondPage))
	}
	if want := uuid.MustParse("dddddddd-0000-0000-0000-000000000001"); secondPage[0].ID != want {
		t.Fatalf("second page ID = %s, want %s", secondPage[0].ID, want)
	}
	seen := make(map[uuid.UUID]struct{}, len(firstPage)+len(secondPage))
	for _, execution := range append(firstPage, secondPage...) {
		if _, exists := seen[execution.ID]; exists {
			t.Fatalf("execution %s appeared on more than one cursor page", execution.ID)
		}
		seen[execution.ID] = struct{}{}
	}
	for _, want := range []uuid.UUID{
		uuid.MustParse("dddddddd-0000-0000-0000-000000000001"),
		uuid.MustParse("dddddddd-0000-0000-0000-000000000002"),
		uuid.MustParse("dddddddd-0000-0000-0000-000000000003"),
	} {
		if _, exists := seen[want]; !exists {
			t.Fatalf("execution %s was skipped across cursor pages", want)
		}
	}
	finalStartedAt := secondPage[0].StartedAt
	finalID := secondPage[0].ID
	finalPage, err := repository.GetExecutionsByWorkflowID(
		ctx,
		workflowID,
		2,
		&finalStartedAt,
		&finalID,
	)
	if err != nil {
		t.Fatalf("GetExecutionsByWorkflowID empty final page: %v", err)
	}
	if len(finalPage) != 0 {
		t.Fatalf("empty final execution page length = %d, want 0", len(finalPage))
	}

	executor.reset()
	_, err = repository.GetExecutionsByWorkflowID(ctx, workflowID, 2, &beforeStartedAt, nil)
	if err == nil {
		t.Fatal("GetExecutionsByWorkflowID accepted a partial keyset cursor")
	}
	if executor.queryCalls != 0 {
		t.Fatalf("partial cursor executed %d queries, want 0", executor.queryCalls)
	}
}

type workflowCountingExecutor struct {
	*sql.DB
	queryCalls    int
	queryRowCalls int
	execCalls     int
}

func (e *workflowCountingExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	e.execCalls++
	return e.DB.ExecContext(ctx, query, args...)
}

func (e *workflowCountingExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	e.queryCalls++
	return e.DB.QueryContext(ctx, query, args...)
}

func (e *workflowCountingExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	e.queryRowCalls++
	return e.DB.QueryRowContext(ctx, query, args...)
}

func (e *workflowCountingExecutor) reset() {
	e.queryCalls = 0
	e.queryRowCalls = 0
	e.execCalls = 0
}

func insertWorkflowDatabaseFixtures(t *testing.T, database *sql.DB) {
	t.Helper()

	startedAt := time.Date(2026, time.July, 26, 20, 0, 0, 0, time.UTC)
	_, err := database.Exec(`
		INSERT INTO public.server_workflows (
			id, server_id, name, enabled, definition, created_by, created_at, updated_at
		) VALUES
			(
				'bbbbbbbb-0000-0000-0000-000000000001',
				'aaaaaaaa-0000-0000-0000-000000000001',
				'Workflow A',
				TRUE,
				'{"version":"1","triggers":[],"variables":{},"steps":[]}'::jsonb,
				'cccccccc-0000-0000-0000-000000000001',
				NOW(),
				NOW()
			),
			(
				'bbbbbbbb-0000-0000-0000-000000000002',
				'aaaaaaaa-0000-0000-0000-000000000001',
				'Workflow B',
				TRUE,
				'{"version":"1","triggers":[],"variables":{},"steps":[]}'::jsonb,
				'cccccccc-0000-0000-0000-000000000001',
				NOW(),
				NOW()
			);

		INSERT INTO public.server_workflow_variables (
			id, workflow_id, name, value, created_at, updated_at
		) VALUES
			(
				'eeeeeeee-0000-0000-0000-000000000001',
				'bbbbbbbb-0000-0000-0000-000000000001',
				'variable_a',
				'{"value":"a"}'::jsonb,
				NOW(),
				NOW()
			),
			(
				'eeeeeeee-0000-0000-0000-000000000002',
				'bbbbbbbb-0000-0000-0000-000000000002',
				'variable_b',
				'{"value":"b"}'::jsonb,
				NOW(),
				NOW()
			);
	`)
	if err != nil {
		t.Fatalf("insert workflow and variable fixtures: %v", err)
	}

	_, err = database.Exec(`
		INSERT INTO public.server_workflow_executions (
			id, workflow_id, execution_id, status, started_at
		) VALUES
			(
				'dddddddd-0000-0000-0000-000000000001',
				'bbbbbbbb-0000-0000-0000-000000000001',
				'ffffffff-0000-0000-0000-000000000001',
				'RUNNING',
				$1
			),
			(
				'dddddddd-0000-0000-0000-000000000002',
				'bbbbbbbb-0000-0000-0000-000000000001',
				'ffffffff-0000-0000-0000-000000000002',
				'RUNNING',
				$1
			),
			(
				'dddddddd-0000-0000-0000-000000000003',
				'bbbbbbbb-0000-0000-0000-000000000001',
				'ffffffff-0000-0000-0000-000000000003',
				'RUNNING',
				$1
			);
	`, startedAt)
	if err != nil {
		t.Fatalf("insert workflow execution fixtures: %v", err)
	}
}
