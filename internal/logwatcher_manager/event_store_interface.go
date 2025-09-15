package logwatcher_manager

import (
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// EventStoreInterface defines the interface for event stores
type EventStoreInterface interface {
	// GetServerID returns the server ID this event store is tracking
	GetServerID() uuid.UUID

	// Join request methods
	StoreJoinRequest(chainID string, playerData *JoinRequestData)
	GetJoinRequest(chainID string) (*JoinRequestData, bool)

	// Player data methods
	StorePlayerData(playerID string, data *PlayerData)
	GetPlayerData(playerID string) (*PlayerData, bool)
	RemovePlayerData(playerID string) bool

	// Session data methods
	StoreSessionData(key string, data *SessionData)
	GetSessionData(key string) (*SessionData, bool)

	// Disconnected player methods
	RemoveDisconnectedPlayer(playerID string)

	// Round data methods
	StoreRoundWinner(data *RoundWinnerData)
	StoreRoundLoser(data *RoundLoserData)
	GetRoundWinner(remove bool) (*RoundWinnerData, bool)
	GetRoundLoser(remove bool) (*RoundLoserData, bool)

	// WON data methods
	StoreWonData(data *WonData)
	GetWonData() (*WonData, bool)

	// Utility methods
	ClearNewGameData()
	CheckTeamkill(victimName string, attackerEOSID string) bool

	// Player lookup methods (new interface)
	GetPlayerInfoByName(name string) (*event_manager.PlayerInfo, bool)
	GetPlayerInfoByEOSID(eosID string) (*event_manager.PlayerInfo, bool)
	GetPlayerInfoByController(controllerID string) (*event_manager.PlayerInfo, bool)
}
