package rconEvents

const (
	CHAT_MESSAGE             = "CHAT_MESSAGE"
	SQUAD_CREATED            = "SQUAD_CREATED"
	PLAYER_WARNED            = "PLAYER_WARNED"
	PLAYER_KICKED            = "PLAYER_KICKED"
	PLAYER_BANNED            = "PLAYER_BANNED"
	POSSESSED_ADMIN_CAMERA   = "POSSESSED_ADMIN_CAMERA"
	UNPOSSESSED_ADMIN_CAMERA = "UNPOSSESSED_ADMIN_CAMERA"

	/* RCON COMMANDS */

	LIST_PLAYERS     = "ListPlayers"
	LIST_SQUADS      = "ListSquads"
	SHOW_CURRENT_MAP = "ShowCurrentMap"
	SHOW_NEXT_MAP    = "ShowNextMap"
	SHOW_SERVER_INFO = "ShowServerInfo"

	/* CONNECTION */

	RECONNECTING = "reconnecting"
	CONNECTED    = "connected"
	CLOSE        = "close"
	ERROR        = "error"
	DATA         = "data"
)
