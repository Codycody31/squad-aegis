package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
)

// PlayerIdentity represents a consolidated player identity
type PlayerIdentity struct {
	CanonicalID    string
	PrimarySteamID string
	PrimaryEOSID   string
	PrimaryName    string
	AllSteamIDs    []string
	AllEOSIDs      []string
	AllNames       []string
	TotalSessions  uint64
	FirstSeen      time.Time
	LastSeen       time.Time
}

// IdentifierPair represents a steam/eos pair from a single join event
type IdentifierPair struct {
	Steam      string
	EOS        string
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

	if len(pairs) == 0 {
		log.Info().Msg("No identifier pairs found, skipping identity computation")
		return nil
	}

	// Step 2: Build Union-Find structure and link identifiers
	uf := NewUnionFind()
	for _, pair := range pairs {
		if pair.Steam != "" {
			uf.Add("steam:" + pair.Steam)
		}
		if pair.EOS != "" {
			uf.Add("eos:" + pair.EOS)
		}
		// Link steam and eos if both present in the same pair
		if pair.Steam != "" && pair.EOS != "" {
			uf.Union("steam:"+pair.Steam, "eos:"+pair.EOS)
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

// fetchAllIdentifierPairs fetches all unique steam/eos/name combinations from join events
func (r *Resolver) fetchAllIdentifierPairs(ctx context.Context) ([]IdentifierPair, error) {
	query := `
		SELECT
			COALESCE(steam, '') as steam,
			COALESCE(eos, '') as eos,
			any(player_suffix) as name,
			min(event_time) as first_seen,
			max(event_time) as last_seen,
			count() as session_count
		FROM squad_aegis.server_join_succeeded_events
		WHERE steam != '' OR eos != ''
		GROUP BY steam, eos
	`

	rows, err := r.clickhouse.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []IdentifierPair
	for rows.Next() {
		var p IdentifierPair
		if err := rows.Scan(&p.Steam, &p.EOS, &p.Name, &p.FirstSeen, &p.LastSeen, &p.SessionCnt); err != nil {
			log.Error().Err(err).Msg("Failed to scan identifier pair")
			continue
		}
		pairs = append(pairs, p)
	}

	return pairs, nil
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

	for _, p := range pairs {
		if p.Steam != "" {
			steamToData[p.Steam] = append(steamToData[p.Steam], p)
		}
		if p.EOS != "" {
			eosToData[p.EOS] = append(eosToData[p.EOS], p)
		}
	}

	identities := make([]PlayerIdentity, 0, len(groups))

	for _, members := range groups {
		identity := PlayerIdentity{}

		// Collect all steam IDs, EOS IDs from this group
		steamSet := make(map[string]struct{})
		eosSet := make(map[string]struct{})
		nameSet := make(map[string]struct{})
		var totalSessions uint64
		var firstSeen, lastSeen time.Time
		var latestName string
		var latestNameTime time.Time

		for _, member := range members {
			if len(member) > 6 && member[:6] == "steam:" {
				steamID := member[6:]
				steamSet[steamID] = struct{}{}
				// Get data for this steam ID
				for _, p := range steamToData[steamID] {
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
			} else if len(member) > 4 && member[:4] == "eos:" {
				eosID := member[4:]
				eosSet[eosID] = struct{}{}
				// Get data for this EOS ID
				for _, p := range eosToData[eosID] {
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
		}

		// Convert sets to sorted slices
		identity.AllSteamIDs = setToSortedSlice(steamSet)
		identity.AllEOSIDs = setToSortedSlice(eosSet)
		identity.AllNames = setToSortedSlice(nameSet)

		// Set primary identifiers (prefer non-empty, use first sorted)
		if len(identity.AllSteamIDs) > 0 {
			identity.PrimarySteamID = identity.AllSteamIDs[0]
		}
		if len(identity.AllEOSIDs) > 0 {
			identity.PrimaryEOSID = identity.AllEOSIDs[0]
		}
		identity.PrimaryName = latestName
		if identity.PrimaryName == "" && len(identity.AllNames) > 0 {
			identity.PrimaryName = identity.AllNames[0]
		}

		identity.TotalSessions = totalSessions
		identity.FirstSeen = firstSeen
		identity.LastSeen = lastSeen

		// Generate canonical ID from all linked identifiers
		identity.CanonicalID = generateCanonicalID(identity.AllSteamIDs, identity.AllEOSIDs)

		identities = append(identities, identity)
	}

	return identities
}

// writeIdentities writes player identities to ClickHouse tables using batch inserts
func (r *Resolver) writeIdentities(ctx context.Context, identities []PlayerIdentity) error {
	if len(identities) == 0 {
		return nil
	}

	// Clear existing data (we're doing a full refresh)
	if err := r.clickhouse.Exec(ctx, "TRUNCATE TABLE squad_aegis.player_identities"); err != nil {
		log.Warn().Err(err).Msg("Failed to truncate player_identities table")
	}
	if err := r.clickhouse.Exec(ctx, "TRUNCATE TABLE squad_aegis.player_identity_lookup"); err != nil {
		log.Warn().Err(err).Msg("Failed to truncate player_identity_lookup table")
	}

	// Batch insert identities (1000 at a time)
	const batchSize = 1000
	for i := 0; i < len(identities); i += batchSize {
		end := i + batchSize
		if end > len(identities) {
			end = len(identities)
		}
		batch := identities[i:end]

		if err := r.insertIdentityBatch(ctx, batch); err != nil {
			log.Error().Err(err).Int("batch_start", i).Msg("Failed to insert identity batch")
		}
	}

	// Collect all lookup entries
	var lookups []LookupEntry

	for _, identity := range identities {
		for _, steamID := range identity.AllSteamIDs {
			lookups = append(lookups, LookupEntry{"steam", steamID, identity.CanonicalID})
		}
		for _, eosID := range identity.AllEOSIDs {
			lookups = append(lookups, LookupEntry{"eos", eosID, identity.CanonicalID})
		}
		for _, name := range identity.AllNames {
			lookups = append(lookups, LookupEntry{"name", name, identity.CanonicalID})
		}
	}

	// Batch insert lookups
	for i := 0; i < len(lookups); i += batchSize {
		end := i + batchSize
		if end > len(lookups) {
			end = len(lookups)
		}
		batch := lookups[i:end]

		if err := r.insertLookupBatch(ctx, batch); err != nil {
			log.Error().Err(err).Int("batch_start", i).Msg("Failed to insert lookup batch")
		}
	}

	log.Info().
		Int("identities", len(identities)).
		Int("lookups", len(lookups)).
		Msg("Wrote identity data to ClickHouse")

	return nil
}

// insertIdentityBatch inserts a batch of identities
func (r *Resolver) insertIdentityBatch(ctx context.Context, batch []PlayerIdentity) error {
	if len(batch) == 0 {
		return nil
	}

	// Build batch insert query
	query := `
		INSERT INTO squad_aegis.player_identities (
			canonical_id, primary_steam_id, primary_eos_id, primary_name,
			all_steam_ids, all_eos_ids, all_names,
			total_sessions, first_seen, last_seen
		) VALUES `

	args := make([]interface{}, 0, len(batch)*10)
	for i, identity := range batch {
		if i > 0 {
			query += ", "
		}
		query += "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
		args = append(args,
			identity.CanonicalID,
			identity.PrimarySteamID,
			identity.PrimaryEOSID,
			identity.PrimaryName,
			identity.AllSteamIDs,
			identity.AllEOSIDs,
			identity.AllNames,
			identity.TotalSessions,
			identity.FirstSeen,
			identity.LastSeen,
		)
	}

	return r.clickhouse.Exec(ctx, query, args...)
}

// insertLookupBatch inserts a batch of lookup entries
func (r *Resolver) insertLookupBatch(ctx context.Context, batch []LookupEntry) error {
	if len(batch) == 0 {
		return nil
	}

	query := `
		INSERT INTO squad_aegis.player_identity_lookup (
			identifier_type, identifier_value, canonical_id
		) VALUES `

	args := make([]interface{}, 0, len(batch)*3)
	for i, entry := range batch {
		if i > 0 {
			query += ", "
		}
		query += "(?, ?, ?)"
		args = append(args, entry.Type, entry.Value, entry.CanonicalID)
	}

	return r.clickhouse.Exec(ctx, query, args...)
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
func generateCanonicalID(steamIDs, eosIDs []string) string {
	// Combine all IDs into a sorted, stable string
	all := make([]string, 0, len(steamIDs)+len(eosIDs))
	for _, s := range steamIDs {
		all = append(all, "s:"+s)
	}
	for _, e := range eosIDs {
		all = append(all, "e:"+e)
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
