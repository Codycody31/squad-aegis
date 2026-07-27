package identity

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
)

func TestSnapshotPublicationAndFailedRefreshIntegration(t *testing.T) {
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
		Password: os.Getenv("CLICKHOUSE_TEST_PASSWORD"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ClickHouse client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	aliasSuffix := uuid.NewString()
	batch, err := client.PrepareBatch(ctx, `
		INSERT INTO squad_aegis.server_join_succeeded_events (
			id, event_time, server_id, chain_id, player_suffix, ip,
			steam, eos, epic, ingested_at
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.Append(
		uuid.New(),
		time.Now().UTC(),
		uuid.New(),
		"integration-"+aliasSuffix,
		"Snapshot Test",
		"192.0.2.1",
		"7656119"+aliasSuffix[:5],
		"eos-"+aliasSuffix,
		"epic-"+aliasSuffix,
		time.Now().UTC(),
	); err != nil {
		t.Fatal(err)
	}
	if err := batch.Send(); err != nil {
		t.Fatal(err)
	}

	resolver := NewResolver(client)
	if err := resolver.ComputeIdentities(ctx); err != nil {
		t.Fatal(err)
	}

	currentSnapshot, identityCount, lookupCount := currentSnapshotState(t, ctx, client)
	if identityCount == 0 || lookupCount == 0 {
		t.Fatalf("published counts = identities:%d lookups:%d, want non-zero", identityCount, lookupCount)
	}

	var storedIdentities, storedLookups uint64
	if err := client.QueryRow(ctx, `
		SELECT
			(SELECT count() FROM squad_aegis.player_identity_snapshots WHERE snapshot_id = ?),
			(SELECT count() FROM squad_aegis.player_identity_lookup_snapshots WHERE snapshot_id = ?)
	`, currentSnapshot, currentSnapshot).Scan(&storedIdentities, &storedLookups); err != nil {
		t.Fatal(err)
	}
	if storedIdentities != identityCount || storedLookups != lookupCount {
		t.Fatalf(
			"published pointer does not match both tables: pointer=%d/%d tables=%d/%d",
			identityCount,
			lookupCount,
			storedIdentities,
			storedLookups,
		)
	}

	// A second successful publication exercises partition retention. The
	// bootstrap snapshot plus two refreshes would leave three partitions if
	// the bounded cleanup were not working.
	if err := resolver.ComputeIdentities(ctx); err != nil {
		t.Fatalf("second identity publication: %v", err)
	}
	secondSnapshot, secondIdentityCount, secondLookupCount := currentSnapshotState(t, ctx, client)
	if secondSnapshot == currentSnapshot {
		t.Fatalf("second publication reused snapshot %s", secondSnapshot)
	}
	currentSnapshot = secondSnapshot
	identityCount = secondIdentityCount
	lookupCount = secondLookupCount

	var identityPartitions, lookupPartitions uint64
	if err := client.QueryRow(ctx, `
		SELECT
			(
				SELECT uniqExact(snapshot_id)
				FROM squad_aegis.player_identity_snapshots
			),
			(
				SELECT uniqExact(snapshot_id)
				FROM squad_aegis.player_identity_lookup_snapshots
			)
	`).Scan(&identityPartitions, &lookupPartitions); err != nil {
		t.Fatal(err)
	}
	if identityPartitions > 2 || lookupPartitions > 2 {
		t.Fatalf(
			"snapshot retention left %d identity and %d lookup partitions, want at most two",
			identityPartitions,
			lookupPartitions,
		)
	}

	temporaryTable := "player_identity_lookup_snapshots_unavailable_" + aliasSuffix[:8]
	if err := client.Exec(ctx, fmt.Sprintf(
		"RENAME TABLE squad_aegis.player_identity_lookup_snapshots TO squad_aegis.%s",
		temporaryTable,
	)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer restoreCancel()
		if err := client.Exec(restoreCtx, fmt.Sprintf(
			"RENAME TABLE squad_aegis.%s TO squad_aegis.player_identity_lookup_snapshots",
			temporaryTable,
		)); err != nil {
			t.Errorf("restore lookup snapshot table: %v", err)
		}
	}()

	if err := resolver.ComputeIdentities(ctx); err == nil {
		t.Fatal("identity refresh succeeded while the lookup snapshot table was unavailable")
	}
	afterFailure, _, _ := currentSnapshotState(t, ctx, client)
	if afterFailure != currentSnapshot {
		t.Fatalf("failed refresh changed current snapshot from %s to %s", currentSnapshot, afterFailure)
	}
}

func currentSnapshotState(
	t *testing.T,
	ctx context.Context,
	client *clickhouse.Client,
) (uuid.UUID, uint64, uint64) {
	t.Helper()

	var snapshotID uuid.UUID
	var identityCount, lookupCount uint64
	if err := client.QueryRow(ctx, `
		SELECT snapshot_id, identity_count, lookup_count
		FROM squad_aegis.player_identity_current
		ORDER BY published_at DESC
		LIMIT 1
	`).Scan(&snapshotID, &identityCount, &lookupCount); err != nil {
		t.Fatal(err)
	}
	return snapshotID, identityCount, lookupCount
}
