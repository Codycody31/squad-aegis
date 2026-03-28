package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/models"
)

func TestBuildServerAdminListWhereClauseWithoutSearch(t *testing.T) {
	t.Parallel()

	serverID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	whereClause, args := buildServerAdminListWhereClause(serverID, "   ")

	if strings.Contains(whereClause, "ILIKE") {
		t.Fatalf("where clause unexpectedly contains search filters: %s", whereClause)
	}
	if len(args) != 1 {
		t.Fatalf("arg count = %d, want 1", len(args))
	}
	if got := args[0]; got != serverID {
		t.Fatalf("server arg = %v, want %v", got, serverID)
	}
}

func TestBuildServerAdminListWhereClauseCastsSteamIDsToText(t *testing.T) {
	t.Parallel()

	serverID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	whereClause, args := buildServerAdminListWhereClause(serverID, "76561198000000001")

	if !strings.Contains(whereClause, "COALESCE(sa.steam_id::text, '') ILIKE $2") {
		t.Fatalf("where clause is missing cast for direct admin steam_id: %s", whereClause)
	}
	if !strings.Contains(whereClause, "COALESCE(u.steam_id::text, '') ILIKE $2") {
		t.Fatalf("where clause is missing cast for linked user steam_id: %s", whereClause)
	}
	if strings.Contains(whereClause, "COALESCE(sa.steam_id, '')") {
		t.Fatalf("where clause still contains invalid bigint/string coalesce: %s", whereClause)
	}
	if len(args) != 2 {
		t.Fatalf("arg count = %d, want 2", len(args))
	}
	if got := args[1]; got != "%76561198000000001%" {
		t.Fatalf("search arg = %v, want %q", got, "%76561198000000001%")
	}
}

func TestServerAdminListItemMarshalsSteamIDAsString(t *testing.T) {
	t.Parallel()

	steamID := int64(76561197982049570)

	payload, err := json.Marshal(newServerAdminListItem(models.ServerAdmin{
		SteamId: &steamID,
	}))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	got := string(payload)
	if !strings.Contains(got, `"steam_id":"76561197982049570"`) {
		t.Fatalf("marshaled payload missing string steam_id: %s", got)
	}
	if strings.Contains(got, `"steam_id":76561197982049570`) {
		t.Fatalf("marshaled payload still encoded steam_id as number: %s", got)
	}
}
