package clickhouse

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	clickhousedriver "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/golang-migrate/migrate/v4"
	migrateclickhouse "github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
)

func integrationClient(t *testing.T) *Client {
	t.Helper()
	host := os.Getenv("CLICKHOUSE_TEST_HOST")
	if host == "" {
		t.Skip("set CLICKHOUSE_TEST_HOST to run ClickHouse integration tests")
	}

	port := 9000
	if rawPort := os.Getenv("CLICKHOUSE_TEST_PORT"); rawPort != "" {
		parsed, err := strconv.Atoi(rawPort)
		if err != nil {
			t.Fatalf("parse CLICKHOUSE_TEST_PORT: %v", err)
		}
		port = parsed
	}
	client, err := NewClient(Config{
		Host:     host,
		Port:     port,
		Database: "squad_aegis",
		Password: os.Getenv("CLICKHOUSE_TEST_PASSWORD"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ClickHouse client: %v", err)
		}
	})
	return client
}

func TestClickHouseMigrationsIntegration(t *testing.T) {
	client := integrationClient(t)
	if err := Migrate(client.GetConnection(), false); err != nil {
		t.Fatal(err)
	}

	driver, err := migrateclickhouse.WithInstance(client.GetConnection(), &migrateclickhouse.Config{
		MigrationsTable:       "migrations",
		MultiStatementEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := iofs.New(Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	migrator, err := migrate.NewWithInstance("iofs", source, "clickhouse", driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrator.Steps(-7); err != nil {
		t.Fatalf("roll back ClickHouse optimization migrations: %v", err)
	}
	if err := migrator.Steps(7); err != nil {
		t.Fatalf("reapply ClickHouse optimization migrations: %v", err)
	}

	var version uint64
	var dirty uint8
	if err := client.QueryRow(context.Background(), `
		SELECT version, dirty
		FROM squad_aegis.migrations
		ORDER BY sequence DESC
		LIMIT 1
	`).Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version != 24 || dirty != 0 {
		t.Fatalf("migration state = version:%d dirty:%d, want version:24 dirty:0", version, dirty)
	}
}

func TestHardcodedClickHouseObjectsExistIntegration(t *testing.T) {
	client := integrationClient(t)
	if err := Migrate(client.GetConnection(), false); err != nil {
		t.Fatal(err)
	}

	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	objectPattern := regexp.MustCompile(`squad_aegis\.([A-Za-z_][A-Za-z0-9_]*)`)
	references := make(map[string][]string)
	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range objectPattern.FindAllSubmatch(body, -1) {
			name := string(match[1])
			references[name] = append(references[name], path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := client.Query(context.Background(), `
		SELECT name
		FROM system.tables
		WHERE database = 'squad_aegis'
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	existing := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		existing[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	var missing []string
	for name, paths := range references {
		if _, ok := existing[name]; ok {
			continue
		}
		sort.Strings(paths)
		missing = append(missing, name+" ("+strings.Join(paths, ", ")+")")
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		t.Fatalf("hardcoded ClickHouse objects missing from system.tables:\n%s", strings.Join(missing, "\n"))
	}
}

func TestNativeBatchAndAcknowledgedWorkflowInsertIntegration(t *testing.T) {
	client := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	chainID := "integration-" + uuid.NewString()
	batch, err := client.PrepareBatch(ctx, `
		INSERT INTO squad_aegis.server_tick_rate_events
			(event_time, server_id, chain_id, tick_rate, ingested_at)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.Append(time.Now().UTC(), uuid.New(), chainID, float32(40), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := batch.Send(); err != nil {
		t.Fatal(err)
	}

	var count uint64
	if err := client.QueryRow(ctx, `
		SELECT count()
		FROM squad_aegis.server_tick_rate_events
		WHERE chain_id = ?
	`, chainID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("native batch row count = %d, want 1", count)
	}

	messageID := uuid.New()
	ingester := NewEventIngester(ctx, client, nil)
	defer ingester.writeCancel()
	if err := ingester.ingestChatMessages(ctx, []*IngestEvent{{
		EventID:   messageID,
		ServerID:  uuid.New(),
		EventTime: time.Now().UTC(),
		Data: &event_manager.RconChatMessageData{
			SteamID:    "76561198000000000",
			EosID:      "integration-eos",
			PlayerName: "Integration",
			ChatType:   "all",
			Message:    "native ingester row",
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := client.QueryRow(ctx, `
		SELECT count()
		FROM squad_aegis.server_player_chat_messages
		WHERE message_id = ?
	`, messageID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("native ingester row count = %d, want 1", count)
	}

	message := &models.WorkflowLogMessage{
		ExecutionID: uuid.New(),
		WorkflowID:  uuid.New(),
		ServerID:    uuid.New(),
		StepID:      "integration",
		StepName:    "Integration",
		LogTime:     time.Now().UTC(),
		LogLevel:    "INFO",
		Message:     "acknowledged asynchronous insert",
		Variables:   map[string]any{},
		Metadata:    map[string]any{},
	}
	if err := client.LogWorkflowMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	if message.EntryID == uuid.Nil {
		t.Fatal("workflow message did not receive an entry ID")
	}
	if err := client.QueryRow(ctx, `
		SELECT count()
		FROM squad_aegis.workflow_log_messages
		WHERE server_id = ? AND workflow_id = ? AND execution_id = ? AND entry_id = ?
	`, message.ServerID, message.WorkflowID, message.ExecutionID, message.EntryID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("acknowledged workflow message count = %d, want 1", count)
	}
}

func TestWorkflowLogKeysetIntegration(t *testing.T) {
	client := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverID := uuid.New()
	workflowID := uuid.New()
	executionID := uuid.New()
	eventTime := time.Now().UTC().Truncate(time.Millisecond)
	entryIDs := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		uuid.MustParse("00000000-0000-0000-0000-000000000003"),
	}

	write := func(entryID uuid.UUID) {
		t.Helper()
		err := client.LogWorkflowExecution(ctx, &models.WorkflowExecutionLog{
			EntryID:          entryID,
			ExecutionID:      executionID,
			WorkflowID:       workflowID,
			ServerID:         serverID,
			EventTime:        eventTime,
			TriggerEventType: "integration",
			TriggerEventData: map[string]any{},
			Status:           "RUNNING",
			StepName:         "same tuple",
			StepType:         "integration",
			StepOrder:        1,
			StepStatus:       "COMPLETED",
			StepInput:        map[string]any{},
			StepOutput:       map[string]any{},
			Variables:        map[string]any{},
			Metadata:         map[string]any{},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	write(entryIDs[0])
	write(entryIDs[2])

	firstPage, err := client.GetWorkflowExecutionLogs(
		ctx,
		serverID,
		workflowID,
		executionID,
		1,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) != 1 || firstPage[0].EntryID != entryIDs[0] {
		t.Fatalf("first page entry IDs = %v, want [%s]", workflowEntryIDs(firstPage), entryIDs[0])
	}

	// This row sorts between the first page and the row that already existed.
	// A keyset continuation must return it without duplicating the first row.
	write(entryIDs[1])
	after := &WorkflowExecutionLogCursor{
		StepOrder: firstPage[0].StepOrder,
		EventTime: firstPage[0].EventTime,
		EntryID:   firstPage[0].EntryID,
	}
	secondPage, err := client.GetWorkflowExecutionLogs(
		ctx,
		serverID,
		workflowID,
		executionID,
		3,
		after,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := workflowEntryIDs(secondPage); len(got) != 2 ||
		got[0] != entryIDs[1] ||
		got[1] != entryIDs[2] {
		t.Fatalf("second page entry IDs = %v, want [%s %s]", got, entryIDs[1], entryIDs[2])
	}

	wrongServer, err := client.GetWorkflowExecutionLogs(
		ctx,
		uuid.New(),
		workflowID,
		executionID,
		3,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(wrongServer) != 0 {
		t.Fatalf("wrong-server query returned %d rows", len(wrongServer))
	}
}

func TestCombatBloomIndexPrunesAtLeastEightyPercent(t *testing.T) {
	client := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	serverID := uuid.New()
	target := "combat-target-" + uuid.NewString()

	if err := client.Exec(ctx, `/* sa.test.combat_fixture */
		INSERT INTO squad_aegis.server_player_damaged_events (
			event_time, server_id, chain_id,
			victim_name, victim_steam, victim_eos,
			damage, teamkill,
			attacker_name, attacker_controller, weapon,
			attacker_steam, attacker_eos
		)
		SELECT
			now64(3) - toIntervalMillisecond(number),
			?,
			toString(number),
			'victim', 'other-victim', '',
			toFloat32(1), 0,
			'attacker', '', 'weapon',
			if(number = 50000, ?, 'other-attacker'), ''
		FROM numbers(100000)
	`, serverID, target); err != nil {
		t.Fatal(err)
	}

	rows, err := client.Query(ctx, `/* sa.test.combat_explain */
		EXPLAIN indexes = 1
		SELECT count()
		FROM squad_aegis.server_player_damaged_events
		WHERE server_id = ?
		  AND (
			attacker_steam = ? OR attacker_eos = ?
			OR victim_steam = ? OR victim_eos = ?
		  )
	`, serverID, target, target, target, target)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(planLines, "\n")
	match := regexp.MustCompile(`(?s)Name: idx_damaged_identity_bloom.*?Granules: ([0-9]+)/([0-9]+)`).FindStringSubmatch(plan)
	if len(match) != 3 {
		t.Fatalf("combat Bloom index was not used:\n%s", plan)
	}
	kept, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatal(err)
	}
	total, err := strconv.Atoi(match[2])
	if err != nil {
		t.Fatal(err)
	}
	if total < 5 || kept*5 > total {
		t.Fatalf("combat Bloom pruning kept %d/%d granules, want at most 20%%", kept, total)
	}
}

func TestWorkflowLookupReadsOneGranuleInTwoMillionRowFixture(t *testing.T) {
	client := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	serverID, workflowID := uuid.New(), uuid.New()
	// Put the selective execution at the upper edge of the execution_id key
	// range so ClickHouse needs exactly the final sparse-index granule.
	targetExecutionID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	otherExecutionID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	if err := client.Exec(ctx, `/* sa.test.workflow_fixture */
		INSERT INTO squad_aegis.workflow_execution_logs (
			execution_id, workflow_id, server_id, event_time,
			trigger_event_type, trigger_event_data, status,
			step_name, step_type, step_order, step_status,
			step_input, step_output, step_error, step_duration_ms,
			variables, metadata
		)
		SELECT
			if(number = 1000000, ?, ?), ?, ?,
			now64(3) - toIntervalMillisecond(number % 100000),
			'test', '{}', 'COMPLETED',
			'step', 'test', toUInt32(number % 10), 'COMPLETED',
			'{}', '{}', NULL, 1, '{}', '{}'
		FROM numbers(2000000)
	`, targetExecutionID, otherExecutionID, workflowID, serverID); err != nil {
		t.Fatal(err)
	}

	rows, err := client.Query(ctx, `/* sa.test.workflow_explain */
		EXPLAIN indexes = 1
		SELECT entry_id
		FROM squad_aegis.workflow_execution_logs
		WHERE server_id = ? AND workflow_id = ? AND execution_id = ?
		ORDER BY step_order, event_time, entry_id
		LIMIT 101
	`, serverID, workflowID, targetExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(planLines, "\n")
	match := regexp.MustCompile(`(?s)PrimaryKey.*?Granules: ([0-9]+)/([0-9]+)`).FindStringSubmatch(plan)
	if len(match) != 3 {
		t.Fatalf("workflow primary key was not used:\n%s", plan)
	}
	kept, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatal(err)
	}
	total, err := strconv.Atoi(match[2])
	if err != nil {
		t.Fatal(err)
	}
	if kept > 2 || total < 200 {
		t.Fatalf("workflow lookup primary-key range kept %d/%d granules", kept, total)
	}

	var readRows atomic.Uint64
	queryCtx := clickhousedriver.Context(ctx, clickhousedriver.WithProgress(func(progress *clickhousedriver.Progress) {
		readRows.Add(progress.Rows)
	}))
	dataRows, err := client.Query(queryCtx, `/* sa.test.workflow_lookup */
		SELECT entry_id
		FROM squad_aegis.workflow_execution_logs
		WHERE server_id = ? AND workflow_id = ? AND execution_id = ?
		ORDER BY step_order, event_time, entry_id
		LIMIT 101
	`, serverID, workflowID, targetExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	for dataRows.Next() {
		var entryID uuid.UUID
		if err := dataRows.Scan(&entryID); err != nil {
			t.Fatal(err)
		}
	}
	if err := dataRows.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readRows.Load(); got > 8192 {
		t.Fatalf("workflow lookup read %d rows, want at most one 8,192-row granule", got)
	}
}

func TestIdentityLookupReadBounds(t *testing.T) {
	client := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	snapshotID := uuid.New()
	exactValue := "7656119" + strings.ReplaceAll(uuid.NewString(), "-", "")
	nameNeedle := "selective-name-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if err := client.Exec(cleanupCtx, `ALTER TABLE squad_aegis.player_identity_lookup_snapshots DROP PARTITION ?`, snapshotID); err != nil {
			t.Errorf("drop identity performance fixture: %v", err)
		}
	})

	if err := client.Exec(ctx, `/* sa.test.identity_fixture */
		INSERT INTO squad_aegis.player_identity_lookup_snapshots (
			snapshot_id, identifier_type, identifier_value, canonical_id
		)
		SELECT
			?,
			if(number < 500000, 'steam', 'name'),
			multiIf(
				number = 250000, ?,
				number = 750000, concat('prefix-', ?, '-suffix'),
				concat(if(number < 500000, 'steam-', 'ordinary-player-'), toString(number))
			),
			concat('canonical-', toString(number))
		FROM numbers(1000000)
	`, snapshotID, exactValue, nameNeedle); err != nil {
		t.Fatal(err)
	}

	readRowsFor := func(query string, args ...interface{}) (uint64, int) {
		t.Helper()
		var readRows atomic.Uint64
		queryCtx := clickhousedriver.Context(ctx, clickhousedriver.WithProgress(func(progress *clickhousedriver.Progress) {
			readRows.Add(progress.Rows)
		}))
		rows, err := client.Query(queryCtx, query, args...)
		if err != nil {
			t.Fatal(err)
		}
		returned := 0
		for rows.Next() {
			var canonicalID string
			if err := rows.Scan(&canonicalID); err != nil {
				t.Fatal(err)
			}
			returned++
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		return readRows.Load(), returned
	}

	exactRead, exactReturned := readRowsFor(`/* sa.test.identity_exact */
		SELECT canonical_id
		FROM squad_aegis.player_identity_lookup_snapshots
		WHERE snapshot_id = ? AND identifier_type = 'steam' AND identifier_value = ?
		LIMIT 1
	`, snapshotID, exactValue)
	if exactReturned != 1 || exactRead > 8192 {
		t.Fatalf("exact identity lookup returned %d and read %d rows, want one result and at most 8,192 rows", exactReturned, exactRead)
	}

	nameRead, nameReturned := readRowsFor(`/* sa.test.identity_name */
		SELECT canonical_id
		FROM squad_aegis.player_identity_lookup_snapshots
		WHERE snapshot_id = ?
		  AND identifier_type = 'name'
		  AND lowerUTF8(identifier_value) LIKE ?
		LIMIT 20
	`, snapshotID, "%"+strings.ToLower(nameNeedle)+"%")
	if nameReturned != 1 || nameRead > 16384 {
		t.Fatalf("name identity lookup returned %d and read %d rows, want one result and at most 16,384 rows", nameReturned, nameRead)
	}
}

func workflowEntryIDs(logs []models.WorkflowExecutionLog) []uuid.UUID {
	ids := make([]uuid.UUID, len(logs))
	for index := range logs {
		ids[index] = logs[index].EntryID
	}
	return ids
}
