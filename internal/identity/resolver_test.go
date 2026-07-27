package identity

import (
	"reflect"
	"testing"
	"time"
)

func TestUnionFindLinksTransitiveAliases(t *testing.T) {
	uf := NewUnionFind()
	for _, identifier := range []string{"steam:1", "eos:a", "epic:x", "steam:2"} {
		uf.Add(identifier)
	}

	uf.Union("steam:1", "eos:a")
	uf.Union("eos:a", "epic:x")

	root := uf.Find("steam:1")
	for _, identifier := range []string{"eos:a", "epic:x"} {
		if got := uf.Find(identifier); got != root {
			t.Fatalf("Find(%q) = %q, want transitive root %q", identifier, got, root)
		}
	}
	if uf.Find("steam:2") == root {
		t.Fatal("unlinked Steam identifier was merged into the alias group")
	}
}

func TestGenerateCanonicalIDIsIndependentOfInputOrder(t *testing.T) {
	first := generateCanonicalID(
		[]string{"7656112", "7656111"},
		[]string{"eos-b", "eos-a"},
		[]string{"epic-b", "epic-a"},
	)
	second := generateCanonicalID(
		[]string{"7656111", "7656112"},
		[]string{"eos-a", "eos-b"},
		[]string{"epic-a", "epic-b"},
	)

	if first != second {
		t.Fatalf("canonical IDs differ by input ordering: %q != %q", first, second)
	}
}

func TestBuildIdentitiesDeduplicatesPairsAndChoosesLatestName(t *testing.T) {
	resolver := &Resolver{}
	firstSeen := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	lastSeen := firstSeen.Add(time.Hour)

	pairs := []IdentifierPair{
		{
			Steam:      "7656111",
			EOS:        "eos-a",
			Name:       "Old Name",
			FirstSeen:  firstSeen,
			LastSeen:   firstSeen.Add(time.Minute),
			SessionCnt: 2,
		},
		{
			Steam:      "7656111",
			EOS:        "eos-a",
			Name:       "Duplicate Aggregate",
			FirstSeen:  firstSeen,
			LastSeen:   firstSeen.Add(30 * time.Minute),
			SessionCnt: 2,
		},
		{
			EOS:        "eos-a",
			Epic:       "epic-a",
			Name:       "Latest Name",
			FirstSeen:  firstSeen.Add(10 * time.Minute),
			LastSeen:   lastSeen,
			SessionCnt: 3,
		},
	}

	uf := NewUnionFind()
	for _, identifier := range []string{"steam:7656111", "eos:eos-a", "epic:epic-a"} {
		uf.Add(identifier)
	}
	uf.Union("steam:7656111", "eos:eos-a")
	uf.Union("eos:eos-a", "epic:epic-a")

	identities := resolver.buildIdentities(pairs, resolver.groupByCanonical(uf))
	if len(identities) != 1 {
		t.Fatalf("identity count = %d, want 1", len(identities))
	}

	identity := identities[0]
	if identity.PrimaryName != "Latest Name" {
		t.Fatalf("primary name = %q, want Latest Name", identity.PrimaryName)
	}
	if identity.TotalSessions != 5 {
		t.Fatalf("total sessions = %d, want duplicate pair counted once (5)", identity.TotalSessions)
	}
	if !identity.FirstSeen.Equal(firstSeen) || !identity.LastSeen.Equal(lastSeen) {
		t.Fatalf("unexpected identity time range: %s - %s", identity.FirstSeen, identity.LastSeen)
	}
	if !reflect.DeepEqual(identity.AllSteamIDs, []string{"7656111"}) ||
		!reflect.DeepEqual(identity.AllEOSIDs, []string{"eos-a"}) ||
		!reflect.DeepEqual(identity.AllEpicIDs, []string{"epic-a"}) {
		t.Fatalf("unexpected aliases: steam=%v eos=%v epic=%v",
			identity.AllSteamIDs,
			identity.AllEOSIDs,
			identity.AllEpicIDs,
		)
	}
}
