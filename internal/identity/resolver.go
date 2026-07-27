package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
)

// PlayerIdentity represents a consolidated player identity
type PlayerIdentity struct {
	CanonicalID    string
	PrimarySteamID string
	PrimaryEOSID   string
	PrimaryEpicID  string
	PrimaryName    string
	AllSteamIDs    []string
	AllEOSIDs      []string
	AllEpicIDs     []string
	AllNames       []string
	TotalSessions  uint64
	FirstSeen      time.Time
	LastSeen       time.Time
}

// IdentifierPair represents a steam/eos pair from a single join event
type IdentifierPair struct {
	Steam      string
	EOS        string
	Epic       string
	Name       string
	FirstSeen  time.Time
	LastSeen   time.Time
	SessionCnt uint64
}

// LookupEntry represents an entry in the identity lookup table
type LookupEntry struct {
	Type        string
	Value       string
	CanonicalID string
}

// Resolver computes transitive player identities using Union-Find algorithm
type Resolver struct {
	clickhouse *clickhouse.Client
}

// NewResolver creates a new identity resolver
func NewResolver(ch *clickhouse.Client) *Resolver {
	return &Resolver{
		clickhouse: ch,
	}
}

// UnionFind implements the Union-Find (Disjoint Set Union) data structure
// with path compression and union by rank for efficient transitive closure
type UnionFind struct {
	parent map[string]string
	rank   map[string]int
}

// NewUnionFind creates a new Union-Find structure
func NewUnionFind() *UnionFind {
	return &UnionFind{
		parent: make(map[string]string),
		rank:   make(map[string]int),
	}
}

// Add adds an element to the Union-Find structure
func (uf *UnionFind) Add(x string) {
	if _, exists := uf.parent[x]; !exists {
		uf.parent[x] = x
		uf.rank[x] = 0
	}
}

// Find returns the root of the set containing x, with path compression
func (uf *UnionFind) Find(x string) string {
	if uf.parent[x] != x {
		uf.parent[x] = uf.Find(uf.parent[x]) // Path compression
	}
	return uf.parent[x]
}

// Union merges the sets containing x and y using union by rank
func (uf *UnionFind) Union(x, y string) {
	rootX, rootY := uf.Find(x), uf.Find(y)
	if rootX == rootY {
		return
	}
	// Union by rank
	if uf.rank[rootX] < uf.rank[rootY] {
		uf.parent[rootX] = rootY
	} else if uf.rank[rootX] > uf.rank[rootY] {
		uf.parent[rootY] = rootX
	} else {
		uf.parent[rootY] = rootX
		uf.rank[rootX]++
	}
}

// GetAllElements returns all elements in the Union-Find structure
func (uf *UnionFind) GetAllElements() []string {
	elements := make([]string, 0, len(uf.parent))
	for k := range uf.parent {
		elements = append(elements, k)
	}
	return elements
}

// ComputeIdentities computes all player identities and writes them to ClickHouse
func (r *Resolver) ComputeIdentities(ctx context.Context) error {
	startTime := time.Now()
	log.Info().Msg("Starting player identity computation")

	// Step 1: Fetch all unique steam/eos pairs with aggregated data
	pairs, err := r.fetchAllIdentifierPairs(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch identifier pairs")
		return err
	}
	log.Info().Int("pairs", len(pairs)).Msg("Fetched identifier pairs")

	// Step 2: Build Union-Find structure and link identifiers
	uf := NewUnionFind()
	for _, pair := range pairs {
		if pair.Steam != "" {
			uf.Add("steam:" + pair.Steam)
		}
		if pair.EOS != "" {
			uf.Add("eos:" + pair.EOS)
		}
		if pair.Epic != "" {
			uf.Add("epic:" + pair.Epic)
		}
		// Link all identifiers observed in the same record.
		if pair.Steam != "" && pair.EOS != "" {
			uf.Union("steam:"+pair.Steam, "eos:"+pair.EOS)
		}
		if pair.Steam != "" && pair.Epic != "" {
			uf.Union("steam:"+pair.Steam, "epic:"+pair.Epic)
		}
		if pair.EOS != "" && pair.Epic != "" {
			uf.Union("eos:"+pair.EOS, "epic:"+pair.Epic)
		}
	}
	log.Info().Int("elements", len(uf.parent)).Msg("Built Union-Find structure")

	// Step 3: Group identifiers by canonical root
	groups := r.groupByCanonical(uf)
	log.Info().Int("groups", len(groups)).Msg("Grouped identifiers by canonical root")

	// Step 4: Build player identities from groups
	identities := r.buildIdentities(pairs, groups)
	log.Info().Int("identities", len(identities)).Msg("Built player identities")

	// Step 5: Write to ClickHouse
	if err := r.writeIdentities(ctx, identities); err != nil {
		log.Error().Err(err).Msg("Failed to write identities to ClickHouse")
		return err
	}

	duration := time.Since(startTime)
	log.Info().
		Dur("duration", duration).
		Int("identities", len(identities)).
		Msg("Completed player identity computation")

	return nil
}

// fetchAllIdentifierPairs fetches all unique steam/eos/epic/name combinations
// from player event tables.
func (r *Resolver) fetchAllIdentifierPairs(ctx context.Context) ([]IdentifierPair, error) {
	query := `
		WITH all_identifier_pairs AS (
			-- Join succeeded (primary source with names)
			SELECT steam, eos, epic, player_suffix as name, event_time
			FROM squad_aegis.server_join_succeeded_events
			WHERE steam != '' OR eos != '' OR epic != ''
			UNION ALL
			-- Connected events
			SELECT steam, eos, epic, '' as name, event_time
			FROM squad_aegis.server_player_connected_events
			WHERE steam != '' OR eos != '' OR epic != ''
			UNION ALL
			-- Disconnected events
			SELECT steam, eos, epic, player_suffix as name, event_time
			FROM squad_aegis.server_player_disconnected_events
			WHERE steam != '' OR eos != '' OR epic != ''
			UNION ALL
			-- Possess events
			SELECT player_steam as steam, player_eos as eos, player_epic as epic, player_suffix as name, event_time
			FROM squad_aegis.server_player_possess_events
			WHERE player_steam != '' OR player_eos != '' OR player_epic != ''
			UNION ALL
			-- Died events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE attacker_steam != '' OR attacker_eos != ''
			UNION ALL
			-- Died events (victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE victim_steam != '' OR victim_eos != ''
			UNION ALL
			-- Wounded events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE attacker_steam != '' OR attacker_eos != ''
			UNION ALL
			-- Wounded events (victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE victim_steam != '' OR victim_eos != ''
			UNION ALL
			-- Damaged events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE attacker_steam != '' OR attacker_eos != ''
			UNION ALL
			-- Damaged events (victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE victim_steam != '' OR victim_eos != ''
			UNION ALL
			-- Revived events (reviver)
			SELECT reviver_steam as steam, reviver_eos as eos, '' as epic, reviver_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE reviver_steam != '' OR reviver_eos != ''
			UNION ALL
			-- Revived events (victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE victim_steam != '' OR victim_eos != ''
		),
		normalized_identifier_pairs AS (
			SELECT
				COALESCE(steam, '') as steam,
				COALESCE(eos, '') as eos,
				COALESCE(epic, '') as epic,
				name,
				event_time
			FROM all_identifier_pairs
		)
		SELECT
			steam,
			eos,
			epic,
			argMax(name, if(name != '', event_time, toDateTime64('1970-01-01', 3, 'UTC'))) as name,
			min(event_time) as first_seen,
			max(event_time) as last_seen,
			count() as session_count
		FROM normalized_identifier_pairs
		WHERE steam != '' OR eos != '' OR epic != ''
		GROUP BY steam, eos, epic
	`

	rows, err := r.clickhouse.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []IdentifierPair
	for rows.Next() {
		var p IdentifierPair
		if err := rows.Scan(&p.Steam, &p.EOS, &p.Epic, &p.Name, &p.FirstSeen, &p.LastSeen, &p.SessionCnt); err != nil {
			return nil, err
		}
		pairs = append(pairs, p)
	}

	return pairs, rows.Err()
}

// groupByCanonical groups all identifiers by their canonical root
func (r *Resolver) groupByCanonical(uf *UnionFind) map[string][]string {
	groups := make(map[string][]string)
	for element := range uf.parent {
		root := uf.Find(element)
		groups[root] = append(groups[root], element)
	}
	return groups
}

// buildIdentities creates PlayerIdentity objects from grouped identifiers
func (r *Resolver) buildIdentities(pairs []IdentifierPair, groups map[string][]string) []PlayerIdentity {
	// Create lookup maps for efficient access
	steamToData := make(map[string][]IdentifierPair)
	eosToData := make(map[string][]IdentifierPair)
	epicToData := make(map[string][]IdentifierPair)

	for _, p := range pairs {
		if p.Steam != "" {
			steamToData[p.Steam] = append(steamToData[p.Steam], p)
		}
		if p.EOS != "" {
			eosToData[p.EOS] = append(eosToData[p.EOS], p)
		}
		if p.Epic != "" {
			epicToData[p.Epic] = append(epicToData[p.Epic], p)
		}
	}

	identities := make([]PlayerIdentity, 0, len(groups))

	for _, members := range groups {
		identity := PlayerIdentity{}

		// Collect all steam IDs, EOS IDs from this group
		steamSet := make(map[string]struct{})
		eosSet := make(map[string]struct{})
		epicSet := make(map[string]struct{})
		nameSet := make(map[string]struct{})
		seenPairs := make(map[string]struct{})
		var totalSessions uint64
		var firstSeen, lastSeen time.Time
		var latestName string
		var latestNameTime time.Time

		for _, member := range members {
			consumePairs := func(memberPairs []IdentifierPair) {
				for _, p := range memberPairs {
					pairKey := p.Steam + "|" + p.EOS + "|" + p.Epic
					if _, seen := seenPairs[pairKey]; seen {
						continue
					}
					seenPairs[pairKey] = struct{}{}

					if p.Name != "" {
						nameSet[p.Name] = struct{}{}
						if p.LastSeen.After(latestNameTime) {
							latestNameTime = p.LastSeen
							latestName = p.Name
						}
					}
					totalSessions += p.SessionCnt
					if firstSeen.IsZero() || p.FirstSeen.Before(firstSeen) {
						firstSeen = p.FirstSeen
					}
					if p.LastSeen.After(lastSeen) {
						lastSeen = p.LastSeen
					}
				}
			}

			if len(member) > 6 && member[:6] == "steam:" {
				steamID := member[6:]
				steamSet[steamID] = struct{}{}
				consumePairs(steamToData[steamID])
			} else if len(member) > 4 && member[:4] == "eos:" {
				eosID := member[4:]
				eosSet[eosID] = struct{}{}
				consumePairs(eosToData[eosID])
			} else if len(member) > 5 && member[:5] == "epic:" {
				epicID := member[5:]
				epicSet[epicID] = struct{}{}
				consumePairs(epicToData[epicID])
			}
		}

		// Convert sets to sorted slices
		identity.AllSteamIDs = setToSortedSlice(steamSet)
		identity.AllEOSIDs = setToSortedSlice(eosSet)
		identity.AllEpicIDs = setToSortedSlice(epicSet)
		identity.AllNames = setToSortedSlice(nameSet)

		// Set primary identifiers (prefer non-empty, use first sorted)
		if len(identity.AllSteamIDs) > 0 {
			identity.PrimarySteamID = identity.AllSteamIDs[0]
		}
		if len(identity.AllEOSIDs) > 0 {
			identity.PrimaryEOSID = identity.AllEOSIDs[0]
		}
		if len(identity.AllEpicIDs) > 0 {
			identity.PrimaryEpicID = identity.AllEpicIDs[0]
		}
		identity.PrimaryName = latestName
		if identity.PrimaryName == "" && len(identity.AllNames) > 0 {
			identity.PrimaryName = identity.AllNames[0]
		}

		identity.TotalSessions = totalSessions
		identity.FirstSeen = firstSeen
		identity.LastSeen = lastSeen

		// Generate canonical ID from all linked identifiers
		identity.CanonicalID = generateCanonicalID(identity.AllSteamIDs, identity.AllEOSIDs, identity.AllEpicIDs)

		identities = append(identities, identity)
	}

	return identities
}

// writeIdentities builds an isolated snapshot and publishes it with one pointer
// insert. Readers never observe one identity table without the matching lookup
// table, and a failed refresh leaves the previous pointer untouched.
func (r *Resolver) writeIdentities(ctx context.Context, identities []PlayerIdentity) error {
	snapshotID := uuid.New()
	published := false
	defer func() {
		if published {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := r.dropSnapshotPartitions(cleanupCtx, snapshotID); err != nil {
			log.Warn().
				Err(err).
				Stringer("snapshot_id", snapshotID).
				Msg("Failed to clean unpublished identity snapshot")
		}
	}()

	const batchSize = 1000
	for i := 0; i < len(identities); i += batchSize {
		end := i + batchSize
		if end > len(identities) {
			end = len(identities)
		}
		if err := r.insertIdentityBatch(ctx, snapshotID, identities[i:end]); err != nil {
			return fmt.Errorf("insert identity snapshot batch at row %d: %w", i, err)
		}
	}

	lookups := make([]LookupEntry, 0, len(identities)*4)
	for _, identity := range identities {
		for _, steamID := range identity.AllSteamIDs {
			lookups = append(lookups, LookupEntry{"steam", steamID, identity.CanonicalID})
		}
		for _, eosID := range identity.AllEOSIDs {
			lookups = append(lookups, LookupEntry{"eos", eosID, identity.CanonicalID})
		}
		for _, epicID := range identity.AllEpicIDs {
			lookups = append(lookups, LookupEntry{"epic", epicID, identity.CanonicalID})
		}
		for _, name := range identity.AllNames {
			lookups = append(lookups, LookupEntry{"name", name, identity.CanonicalID})
		}
	}

	for i := 0; i < len(lookups); i += batchSize {
		end := i + batchSize
		if end > len(lookups) {
			end = len(lookups)
		}
		if err := r.insertLookupBatch(ctx, snapshotID, lookups[i:end]); err != nil {
			return fmt.Errorf("insert identity lookup snapshot batch at row %d: %w", i, err)
		}
	}

	var identityCount, lookupCount uint64
	if err := r.clickhouse.QueryRow(ctx, `
		SELECT
			(
				SELECT count()
				FROM squad_aegis.player_identity_snapshots
				WHERE snapshot_id = ?
			),
			(
				SELECT count()
				FROM squad_aegis.player_identity_lookup_snapshots
				WHERE snapshot_id = ?
			)
	`, snapshotID, snapshotID).Scan(&identityCount, &lookupCount); err != nil {
		return fmt.Errorf("verify identity snapshot: %w", err)
	}
	if identityCount != uint64(len(identities)) || lookupCount != uint64(len(lookups)) {
		return fmt.Errorf(
			"identity snapshot verification failed: identities=%d/%d lookups=%d/%d",
			identityCount,
			len(identities),
			lookupCount,
			len(lookups),
		)
	}

	if err := r.clickhouse.Exec(ctx, `
		INSERT INTO squad_aegis.player_identity_current (
			pointer_key, snapshot_id, identity_count, lookup_count, published_at
		) VALUES (?, ?, ?, ?, now64(3))
	`, uint8(1), snapshotID, identityCount, lookupCount); err != nil {
		return fmt.Errorf("publish identity snapshot: %w", err)
	}
	published = true

	if err := r.retainNewestSnapshots(ctx, 2); err != nil {
		log.Warn().Err(err).Msg("Failed to prune old identity snapshots")
	}

	log.Info().
		Stringer("snapshot_id", snapshotID).
		Int("identities", len(identities)).
		Int("lookups", len(lookups)).
		Msg("Published identity snapshot")

	return nil
}

func (r *Resolver) insertIdentityBatch(
	ctx context.Context,
	snapshotID uuid.UUID,
	identities []PlayerIdentity,
) error {
	if len(identities) == 0 {
		return nil
	}

	batch, err := r.clickhouse.PrepareBatch(ctx, `
		INSERT INTO squad_aegis.player_identity_snapshots (
			snapshot_id, canonical_id,
			primary_steam_id, primary_eos_id, primary_epic_id, primary_name,
			all_steam_ids, all_eos_ids, all_epic_ids, all_names,
			total_sessions, first_seen, last_seen
		)
	`)
	if err != nil {
		return err
	}

	for _, identity := range identities {
		if err := batch.Append(
			snapshotID,
			identity.CanonicalID,
			identity.PrimarySteamID,
			identity.PrimaryEOSID,
			identity.PrimaryEpicID,
			identity.PrimaryName,
			identity.AllSteamIDs,
			identity.AllEOSIDs,
			identity.AllEpicIDs,
			identity.AllNames,
			identity.TotalSessions,
			identity.FirstSeen,
			identity.LastSeen,
		); err != nil {
			_ = batch.Abort()
			return err
		}
	}

	return batch.Send()
}

func (r *Resolver) insertLookupBatch(
	ctx context.Context,
	snapshotID uuid.UUID,
	entries []LookupEntry,
) error {
	if len(entries) == 0 {
		return nil
	}

	batch, err := r.clickhouse.PrepareBatch(ctx, `
		INSERT INTO squad_aegis.player_identity_lookup_snapshots (
			snapshot_id, identifier_type, identifier_value, canonical_id
		)
	`)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := batch.Append(snapshotID, entry.Type, entry.Value, entry.CanonicalID); err != nil {
			_ = batch.Abort()
			return err
		}
	}

	return batch.Send()
}

func (r *Resolver) retainNewestSnapshots(ctx context.Context, keep int) error {
	if keep < 1 {
		return fmt.Errorf("identity snapshot retention must keep at least one snapshot")
	}

	rows, err := r.clickhouse.Query(ctx, `
		SELECT snapshot_id
		FROM squad_aegis.player_identity_current
		GROUP BY snapshot_id
		ORDER BY max(published_at) DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var snapshots []uuid.UUID
	for rows.Next() {
		var snapshotID uuid.UUID
		if err := rows.Scan(&snapshotID); err != nil {
			return err
		}
		snapshots = append(snapshots, snapshotID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(snapshots) <= keep {
		return nil
	}

	for _, snapshotID := range snapshots[keep:] {
		if err := r.dropSnapshotPartitions(ctx, snapshotID); err != nil {
			return err
		}

		query := fmt.Sprintf(`
			ALTER TABLE squad_aegis.player_identity_current
			DELETE WHERE snapshot_id = toUUID('%s')
			SETTINGS mutations_sync = 1
		`, snapshotID.String())
		if err := r.clickhouse.Exec(ctx, query); err != nil {
			return fmt.Errorf("remove identity snapshot pointer %s: %w", snapshotID, err)
		}
	}

	return nil
}

func (r *Resolver) dropSnapshotPartitions(ctx context.Context, snapshotID uuid.UUID) error {
	var errs []error
	for _, table := range []string{
		"player_identity_snapshots",
		"player_identity_lookup_snapshots",
	} {
		query := fmt.Sprintf(
			"ALTER TABLE squad_aegis.%s DROP PARTITION '%s'",
			table,
			snapshotID.String(),
		)
		if err := r.clickhouse.Exec(ctx, query); err != nil {
			errs = append(errs, fmt.Errorf("drop %s partition: %w", table, err))
		}
	}
	return errors.Join(errs...)
}

// setToSortedSlice converts a string set to a sorted slice
func setToSortedSlice(set map[string]struct{}) []string {
	slice := make([]string, 0, len(set))
	for k := range set {
		if k != "" {
			slice = append(slice, k)
		}
	}
	sort.Strings(slice)
	return slice
}

// generateCanonicalID creates a stable canonical ID from all linked identifiers
func generateCanonicalID(steamIDs, eosIDs, epicIDs []string) string {
	// Combine all IDs into a sorted, stable string
	all := make([]string, 0, len(steamIDs)+len(eosIDs)+len(epicIDs))
	for _, s := range steamIDs {
		all = append(all, "s:"+s)
	}
	for _, e := range eosIDs {
		all = append(all, "e:"+e)
	}
	for _, epic := range epicIDs {
		all = append(all, "p:"+epic)
	}
	sort.Strings(all)

	// Hash to create a stable ID
	combined := ""
	for _, id := range all {
		combined += id + "|"
	}

	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for shorter ID
}
