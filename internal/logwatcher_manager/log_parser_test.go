package logwatcher_manager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

type testEventStore struct {
	mu           sync.Mutex
	serverID     uuid.UUID
	joinRequests map[string]*JoinRequestData
	playerData   map[string]*PlayerData
	sessionData  map[string]*SessionData
	roundWinner  *RoundWinnerData
	roundLoser   *RoundLoserData
	wonData      *WonData
}

func newTestEventStore(serverID uuid.UUID) *testEventStore {
	return &testEventStore{
		serverID:     serverID,
		joinRequests: make(map[string]*JoinRequestData),
		playerData:   make(map[string]*PlayerData),
		sessionData:  make(map[string]*SessionData),
	}
}

func (s *testEventStore) GetServerID() uuid.UUID { return s.serverID }

func (s *testEventStore) StoreJoinRequest(chainID string, playerData *JoinRequestData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.joinRequests[chainID] = playerData
}

func (s *testEventStore) GetJoinRequest(chainID string) (*JoinRequestData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.joinRequests[chainID]
	if ok {
		delete(s.joinRequests, chainID)
	}
	return value, ok
}

func (s *testEventStore) StorePlayerData(playerID string, data *PlayerData) {
	if playerID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playerData[playerID] = data
}

func (s *testEventStore) GetPlayerData(playerID string) (*PlayerData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.playerData[playerID]
	return value, ok
}

func (s *testEventStore) RemovePlayerData(playerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.playerData, playerID)
	return nil
}

func (s *testEventStore) StoreSessionData(key string, data *SessionData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionData[key] = data
}

func (s *testEventStore) GetSessionData(key string) (*SessionData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.sessionData[key]
	return value, ok
}

func (s *testEventStore) StoreRoundWinner(data *RoundWinnerData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roundWinner = data
}

func (s *testEventStore) StoreRoundLoser(data *RoundLoserData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roundLoser = data
}

func (s *testEventStore) GetRoundWinner(remove bool) (*RoundWinnerData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roundWinner == nil {
		return nil, false
	}
	value := s.roundWinner
	if remove {
		s.roundWinner = nil
	}
	return value, true
}

func (s *testEventStore) GetRoundLoser(remove bool) (*RoundLoserData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roundLoser == nil {
		return nil, false
	}
	value := s.roundLoser
	if remove {
		s.roundLoser = nil
	}
	return value, true
}

func (s *testEventStore) StoreWonData(data *WonData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wonData = data
}

func (s *testEventStore) GetWonData() (*WonData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wonData == nil {
		return nil, false
	}
	return s.wonData, true
}

func (s *testEventStore) ClearNewGameData() {}

func (s *testEventStore) CheckTeamkill(victimName string, attackerEOSID string) bool {
	return false
}

func (s *testEventStore) GetPlayerInfoByName(name string) (*event_manager.PlayerInfo, bool) {
	return nil, false
}

func (s *testEventStore) GetPlayerInfoByIdentifier(playerID string) (*event_manager.PlayerInfo, bool) {
	return nil, false
}

func (s *testEventStore) GetPlayerInfoByEOSID(eosID string) (*event_manager.PlayerInfo, bool) {
	return nil, false
}

func (s *testEventStore) GetPlayerInfoByController(controllerID string) (*event_manager.PlayerInfo, bool) {
	return nil, false
}

func waitForEvent(t *testing.T, ch <-chan event_manager.Event) event_manager.Event {
	t.Helper()

	select {
	case event := <-ch:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return event_manager.Event{}
	}
}

func TestProcessLogForEventsCarriesEpicAliasThroughJoinFlow(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	em := event_manager.NewEventManager(ctx, 10)
	defer em.Shutdown()

	serverID := uuid.New()
	store := newTestEventStore(serverID)
	subscriber := em.Subscribe(event_manager.EventFilter{
		Types: []event_manager.EventType{
			event_manager.EventTypeLogPlayerConnected,
			event_manager.EventTypeLogJoinSucceeded,
		},
	}, nil, 10)
	defer em.Unsubscribe(subscriber.ID)

	parsers := GetLogParsers()

	postLogin := `[2026.03.29-12.00.00:000][1]LogSquad: PostLogin: NewPlayer: BP_PlayerController_C /Game/Maps/Narva.PersistentLevel.PlayerController_1 (IP: 127.0.0.1 | Online IDs: EOS: 0002bb228e4d4363ada0c139b11a9ece epic: e91067b2c8bb461ebf0cdf3a01ee5b0b)`
	joinSucceeded := `[2026.03.29-12.00.05:000][1]LogNet: Join succeeded: 13Baudouin`

	ProcessLogForEvents(postLogin, serverID, parsers, em, store, nil)
	connectedEvent := waitForEvent(t, subscriber.Channel)

	connectedData, ok := connectedEvent.Data.(*event_manager.LogPlayerConnectedData)
	if !ok {
		t.Fatalf("connected event data type = %T, want *LogPlayerConnectedData", connectedEvent.Data)
	}
	if connectedData.EOSID != "0002bb228e4d4363ada0c139b11a9ece" {
		t.Fatalf("connected EOSID = %q, want normalized EOS ID", connectedData.EOSID)
	}
	if connectedData.EpicID != "e91067b2c8bb461ebf0cdf3a01ee5b0b" {
		t.Fatalf("connected EpicID = %q, want normalized Epic ID", connectedData.EpicID)
	}

	ProcessLogForEvents(joinSucceeded, serverID, parsers, em, store, nil)
	joinEvent := waitForEvent(t, subscriber.Channel)

	joinData, ok := joinEvent.Data.(*event_manager.LogJoinSucceededData)
	if !ok {
		t.Fatalf("join event data type = %T, want *LogJoinSucceededData", joinEvent.Data)
	}
	if joinData.EOSID != "0002bb228e4d4363ada0c139b11a9ece" {
		t.Fatalf("join EOSID = %q, want normalized EOS ID", joinData.EOSID)
	}
	if joinData.EpicID != "e91067b2c8bb461ebf0cdf3a01ee5b0b" {
		t.Fatalf("join EpicID = %q, want normalized Epic ID", joinData.EpicID)
	}
}

func TestProcessLogForEventsParsesEpicAliasOnPossess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	em := event_manager.NewEventManager(ctx, 10)
	defer em.Shutdown()

	serverID := uuid.New()
	store := newTestEventStore(serverID)
	subscriber := em.Subscribe(event_manager.EventFilter{
		Types: []event_manager.EventType{event_manager.EventTypeLogPlayerPossess},
	}, nil, 10)
	defer em.Unsubscribe(subscriber.ID)

	parsers := GetLogParsers()

	line := `[2026.03.29-12.01.00:000][2]LogSquadTrace: [DedicatedServer]ASQPlayerController::OnPossess(): PC=13Baudouin (Online IDs: EOS: 0002bb228e4d4363ada0c139b11a9ece epic: e91067b2c8bb461ebf0cdf3a01ee5b0b) Pawn=BP_Soldier_C`
	ProcessLogForEvents(line, serverID, parsers, em, store, nil)

	event := waitForEvent(t, subscriber.Channel)
	possessData, ok := event.Data.(*event_manager.LogPlayerPossessData)
	if !ok {
		t.Fatalf("possess event data type = %T, want *LogPlayerPossessData", event.Data)
	}
	if possessData.PlayerEOS != "0002bb228e4d4363ada0c139b11a9ece" {
		t.Fatalf("possess PlayerEOS = %q, want normalized EOS ID", possessData.PlayerEOS)
	}
	if possessData.PlayerEpic != "e91067b2c8bb461ebf0cdf3a01ee5b0b" {
		t.Fatalf("possess PlayerEpic = %q, want normalized Epic ID", possessData.PlayerEpic)
	}
}
