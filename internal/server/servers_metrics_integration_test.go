package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
)

func TestServerMetricsQueriesRunAsFourStatementPipeline(t *testing.T) {
	client := serverTestClickHouseClient(t)

	api := &Server{Dependencies: &Dependencies{Clickhouse: client}}
	end := time.Now().UTC().Truncate(time.Second)
	metrics, err := api.getMetricsFromClickHouse(context.Background(), uuid.New(), "1h", 5, end.Add(-time.Hour), end)
	if err != nil {
		t.Fatalf("getMetricsFromClickHouse() error = %v", err)
	}
	if metrics.Period != "1h" {
		t.Fatalf("period = %q, want 1h", metrics.Period)
	}
	if len(metrics.ChatActivity) == 0 || len(metrics.ConnectionStats) == 0 {
		t.Fatal("empty ranges must still return zero-filled event series")
	}
}

func TestPlayerNameBatchLookupUsesBothIdentifierTypes(t *testing.T) {
	client := serverTestClickHouseClient(t)
	ctx := context.Background()
	api := &Server{Dependencies: &Dependencies{Clickhouse: client}}
	steamID := "76561198" + strconv.FormatInt(time.Now().UnixNano()%1000000000, 10)
	eosID := "batch-" + uuid.NewString()

	if err := client.Exec(ctx, `
		INSERT INTO squad_aegis.server_join_succeeded_events (
			id, event_time, server_id, chain_id, player_suffix, ip,
			steam, eos, epic, ingested_at
		) VALUES (?, now64(3), ?, ?, 'Batch Name', NULL, ?, ?, NULL, now())
	`, uuid.New(), uuid.New(), uuid.NewString(), steamID, eosID); err != nil {
		t.Fatal(err)
	}

	names, err := api.lookupPlayerNamesBatchByIdentifiers(ctx, []PlayerIdentifier{
		{Value: steamID, IsSteam: true},
		{Value: eosID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if names["steam:"+steamID] != "Batch Name" {
		t.Fatalf("Steam name = %q, want Batch Name", names["steam:"+steamID])
	}
	if names["eos:"+eosID] != "Batch Name" {
		t.Fatalf("EOS name = %q, want Batch Name", names["eos:"+eosID])
	}
}

func TestSudoMetricsHybridQueriesAndCoverageBoundary(t *testing.T) {
	client := serverTestClickHouseClient(t)
	ctx := context.Background()
	api := &Server{Dependencies: &Dependencies{Clickhouse: client}}
	serverID := uuid.New()
	now := time.Now().UTC()

	if err := client.Exec(ctx, `
		INSERT INTO squad_aegis.server_player_chat_messages
			(message_id, server_id, player_name, steam_id, eos_id, sent_at, chat_type, message)
		VALUES (?, ?, 'hybrid-test', 1, '', ?, 'all', 'hybrid-test')
	`, uuid.New(), serverID, now); err != nil {
		t.Fatal(err)
	}
	if err := client.Exec(ctx, `
		INSERT INTO squad_aegis.server_event_daily_counts
			(day, server_id, event_type, event_count)
		VALUES (today() - 1, ?, 'chat', 7)
	`, serverID); err != nil {
		t.Fatal(err)
	}

	activities, err := api.querySudoServerActivities(ctx, 3)
	if err != nil {
		t.Fatalf("raw-boundary activities: %v", err)
	}
	if got := activityForServer(activities, serverID); got == nil || got.TotalEvents != 1 || got.ChatMessages != 1 {
		t.Fatalf("raw-boundary activity = %#v, want one raw chat event", got)
	}

	if err := client.Exec(ctx, `
		INSERT INTO squad_aegis.sql_rollup_state
		SELECT rollup_name, argMax(created_at, updated_at), toNullable(today() - 1), 1, now64(3) + INTERVAL 10 SECOND
		FROM squad_aegis.sql_rollup_state
		WHERE rollup_name = 'server_event_daily_counts'
		GROUP BY rollup_name
	`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Exec(context.Background(), `
			INSERT INTO squad_aegis.sql_rollup_state
			SELECT rollup_name, argMax(created_at, updated_at), toNullable(today() + 1), 0, now64(3) + INTERVAL 20 SECOND
			FROM squad_aegis.sql_rollup_state
			WHERE rollup_name = 'server_event_daily_counts'
			GROUP BY rollup_name
		`)
	})

	activities, err = api.querySudoServerActivities(ctx, 3)
	if err != nil {
		t.Fatalf("rollup-boundary activities: %v", err)
	}
	if got := activityForServer(activities, serverID); got == nil || got.TotalEvents != 8 || got.ChatMessages != 8 {
		t.Fatalf("rollup-boundary activity = %#v, want exactly eight events without double counting", got)
	}

	rows, err := client.Query(ctx, sudoMetricsTimelineQuery, 2)
	if err != nil {
		t.Fatalf("timeline query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var metric string
		var day time.Time
		var value int64
		if err := rows.Scan(&metric, &day, &value); err != nil {
			t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestHealthyIdentitySnapshotReturnsEmptyWithoutFallback(t *testing.T) {
	client := serverTestClickHouseClient(t)
	api := &Server{Dependencies: &Dependencies{Clickhouse: client}}
	search := "no-such-player-" + uuid.NewString()

	players, err := api.searchPlayersFromIdentityTable(context.Background(), search, "%"+search+"%", 20)
	if err != nil {
		t.Fatalf("healthy empty snapshot search returned an error: %v", err)
	}
	if len(players) != 0 {
		t.Fatalf("healthy empty snapshot search returned %d rows", len(players))
	}

	_, err = api.getPlayerFromIdentityTable(context.Background(), search, false)
	if !errors.Is(err, errPlayerNotFoundInSnapshot) {
		t.Fatalf("identity lookup error = %v, want errPlayerNotFoundInSnapshot", err)
	}
}

func TestCanonicalPlayerStatisticsQuery(t *testing.T) {
	client := serverTestClickHouseClient(t)
	api := &Server{Dependencies: &Dependencies{Clickhouse: client}}
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodGet, "/api/players/stats", nil)

	api.PlayersStats(requestContext)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PlayersStats status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestPlayerTeamkillMetricsNormalizedQuery(t *testing.T) {
	client := serverTestClickHouseClient(t)
	api := &Server{Dependencies: &Dependencies{Clickhouse: client}}
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodGet, "/api/players/76561198000000000", nil)

	metrics, err := api.getPlayerTeamkillMetrics(requestContext.Request.Context(), ResolvedPlayerIdentity{
		SteamIDs: []string{"76561198000000000"}, Found: true,
	}, 0)
	if err != nil {
		t.Fatalf("getPlayerTeamkillMetrics() error = %v", err)
	}
	if metrics.TotalTeamkills != 0 || metrics.TotalTeamWounds != 0 || metrics.TotalTeamDamage != 0 {
		t.Fatalf("empty database returned non-zero metrics: %#v", metrics)
	}
}

func TestPlayerProfileQueriesUseResolvedIdentity(t *testing.T) {
	client := serverTestClickHouseClient(t)
	api := &Server{Dependencies: &Dependencies{Clickhouse: client}}
	ctx := context.Background()
	identity := ResolvedPlayerIdentity{
		SteamIDs: []string{"76561198000000000"},
		EOSIDs:   []string{"00000000000000000000000000000000"},
		Found:    true,
	}

	if _, err := api.getPlayerStatistics(ctx, identity); err != nil {
		t.Fatalf("getPlayerStatistics() error = %v", err)
	}
	if _, err := api.getPlayerRecentActivity(ctx, identity, 20); err != nil {
		t.Fatalf("getPlayerRecentActivity() error = %v", err)
	}
	if _, err := api.getPlayerChatHistory(ctx, identity, 20); err != nil {
		t.Fatalf("getPlayerChatHistory() error = %v", err)
	}
	if _, err := api.getPlayerViolations(ctx, identity); err != nil {
		t.Fatalf("getPlayerViolations() error = %v", err)
	}
	if _, err := api.getPlayerRecentServers(ctx, identity); err != nil {
		t.Fatalf("getPlayerRecentServers() error = %v", err)
	}
	if _, err := api.getPlayerViolationSummary(ctx, identity); err != nil {
		t.Fatalf("getPlayerViolationSummary() error = %v", err)
	}
	if _, err := api.getPlayerNameHistory(ctx, identity); err != nil {
		t.Fatalf("getPlayerNameHistory() error = %v", err)
	}
	if _, err := api.getPlayerWeaponStats(ctx, identity); err != nil {
		t.Fatalf("getPlayerWeaponStats() error = %v", err)
	}
}

func TestPlayerSessionizationAndCursorBoundaries(t *testing.T) {
	client := serverTestClickHouseClient(t)
	ctx := context.Background()
	steamID := "session-" + uuid.NewString()
	base := time.Now().UTC().Truncate(time.Second).Add(-3 * time.Hour)
	serverA, serverB, serverC, serverD := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	type sessionEvent struct {
		connected bool
		at        time.Time
		serverID  uuid.UUID
		key       uuid.UUID
	}
	events := []sessionEvent{
		{true, base, serverA, uuid.MustParse("10000000-0000-0000-0000-000000000001")},
		{true, base.Add(5 * time.Minute), serverB, uuid.MustParse("20000000-0000-0000-0000-000000000001")},
		{false, base.Add(10 * time.Minute), serverA, uuid.MustParse("10000000-0000-0000-0000-000000000002")},
		{false, base.Add(15 * time.Minute), serverB, uuid.MustParse("20000000-0000-0000-0000-000000000002")},
		{true, base.Add(20 * time.Minute), serverA, uuid.MustParse("10000000-0000-0000-0000-000000000003")},
		{true, base.Add(30 * time.Minute), serverA, uuid.MustParse("10000000-0000-0000-0000-000000000004")},
		{false, base.Add(40 * time.Minute), serverA, uuid.MustParse("10000000-0000-0000-0000-000000000005")},
		{true, base.Add(50 * time.Minute), serverC, uuid.MustParse("30000000-0000-0000-0000-000000000001")},
		{false, base.Add(50 * time.Minute), serverC, uuid.MustParse("30000000-0000-0000-0000-000000000002")},
		{true, time.Now().UTC().Truncate(time.Second).Add(-10 * time.Minute), serverD, uuid.MustParse("40000000-0000-0000-0000-000000000001")},
	}
	for i, event := range events {
		chainID := "session-event-" + strconv.Itoa(i) + "-" + uuid.NewString()
		if event.connected {
			if err := client.Exec(ctx, `
				INSERT INTO squad_aegis.server_player_connected_events
					(event_time, server_id, chain_id, player_controller, ip, steam, eos, epic, player_suffix, cursor_key)
				VALUES (?, ?, ?, 'controller', '127.0.0.1', ?, '', '', 'Session Test', ?)
			`, event.at, event.serverID, chainID, steamID, event.key); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := client.Exec(ctx, `
			INSERT INTO squad_aegis.server_player_disconnected_events
				(event_time, server_id, chain_id, player_controller, player_suffix, team, ip, steam, eos, epic, cursor_key)
			VALUES (?, ?, ?, 'controller', 'Session Test', '', '127.0.0.1', ?, '', '', ?)
		`, event.at, event.serverID, chainID, steamID, event.key); err != nil {
			t.Fatal(err)
		}
	}

	type sessionRow struct {
		connectTime    time.Time
		disconnectTime *time.Time
		serverID       string
		cursorKey      string
	}
	readPage := func(cursor *playerSessionCursorValues, limit int) []sessionRow {
		t.Helper()
		query := playerSessionsBaseQuery("steam IN (?)")
		args := []interface{}{[]string{steamID}, []string{steamID}}
		if cursor != nil {
			query += " WHERE (connect_time, cursor_key) < (?, ?)"
			args = append(args, cursor.ConnectTime, cursor.CursorKey)
		}
		query += " ORDER BY connect_time DESC, cursor_key DESC LIMIT ?"
		args = append(args, limit)
		rows, err := client.Query(ctx, query, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		result := []sessionRow{}
		for rows.Next() {
			var row sessionRow
			var ip string
			if err := rows.Scan(&row.connectTime, &row.disconnectTime, &row.serverID, &ip, &row.cursorKey); err != nil {
				t.Fatal(err)
			}
			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result
	}

	first := readPage(nil, 3)
	if len(first) != 3 {
		t.Fatalf("first session page length = %d, want 3", len(first))
	}
	lastKey, err := uuid.Parse(first[len(first)-1].cursorKey)
	if err != nil {
		t.Fatal(err)
	}
	second := readPage(&playerSessionCursorValues{ConnectTime: first[len(first)-1].connectTime, CursorKey: lastKey}, 10)
	combined := append(append([]sessionRow{}, first...), second...)
	if len(combined) != 6 {
		t.Fatalf("session page total = %d, want 6", len(combined))
	}
	seen := make(map[string]bool, len(combined))
	for _, row := range combined {
		if seen[row.cursorKey] {
			t.Fatalf("duplicate session cursor key %s", row.cursorKey)
		}
		seen[row.cursorKey] = true
	}

	assertDisconnect := func(key string, want *time.Time) {
		t.Helper()
		for _, row := range combined {
			if row.cursorKey != key {
				continue
			}
			if want == nil {
				if row.disconnectTime != nil {
					t.Fatalf("session %s disconnect = %v, want nil", key, row.disconnectTime)
				}
				return
			}
			if row.disconnectTime == nil || !row.disconnectTime.Equal(*want) {
				t.Fatalf("session %s disconnect = %v, want %v", key, row.disconnectTime, *want)
			}
			return
		}
		t.Fatalf("session %s not found", key)
	}
	aDisconnect := base.Add(10 * time.Minute)
	bDisconnect := base.Add(15 * time.Minute)
	reconnectDisconnect := base.Add(40 * time.Minute)
	tiedDisconnect := base.Add(50 * time.Minute)
	assertDisconnect(events[0].key.String(), &aDisconnect)
	assertDisconnect(events[1].key.String(), &bDisconnect)
	assertDisconnect(events[4].key.String(), nil)
	assertDisconnect(events[5].key.String(), &reconnectDisconnect)
	assertDisconnect(events[7].key.String(), &tiedDisconnect)
	assertDisconnect(events[9].key.String(), nil)

	classify := func(key string) SessionHistoryEntry {
		t.Helper()
		for _, row := range combined {
			if row.cursorKey == key {
				entry := SessionHistoryEntry{ConnectTime: row.connectTime}
				classifyPlayerSession(&entry, row.disconnectTime, time.Now().UTC())
				return entry
			}
		}
		t.Fatalf("session %s not found", key)
		return SessionHistoryEntry{}
	}
	if missing := classify(events[4].key.String()); !missing.MissingDisconnect || missing.Ongoing {
		t.Fatalf("old unpaired session classification = %#v, want missing disconnect", missing)
	}
	if ongoing := classify(events[9].key.String()); !ongoing.Ongoing || ongoing.MissingDisconnect {
		t.Fatalf("recent unpaired session classification = %#v, want ongoing", ongoing)
	}
}

func activityForServer(activities []ServerActivityResponse, serverID uuid.UUID) *ServerActivityResponse {
	for i := range activities {
		if activities[i].ServerID == serverID.String() {
			return &activities[i]
		}
	}
	return nil
}

func serverTestClickHouseClient(t *testing.T) *clickhouse.Client {
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

	client, err := clickhouse.NewClient(clickhouse.Config{
		Host:     host,
		Port:     port,
		Database: "squad_aegis",
		Username: "default",
		Password: os.Getenv("CLICKHOUSE_TEST_PASSWORD"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
