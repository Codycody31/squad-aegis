// Package commands provides functionality for managing and executing Squad game server commands.
// It includes definitions for both admin and public commands, command categorization,
// and access control for command execution.

package commands

import "slices"

// CommandType represents the type of command (admin or public)
type CommandType int

const (
	// PublicCommand represents commands that can be executed by regular users
	PublicCommand CommandType = 0
	// AdminCommand represents commands that require admin privileges
	AdminCommand CommandType = 1
)

// Command represents a basic command structure
type Command struct {
	Name         string
	Category     string
	Parameters   []string
	SupportsRCON bool
}

// CommandInfo provides detailed information about a command
type CommandInfo struct {
	SupportsRCON bool
	Name         string
	Category     string
	Syntax       string
	Description  string
	CommandType  CommandType // 0 for public, 1 for admin
}

// CommandMatrix is a list of all commands that can be executed by the server
var CommandMatrix = []CommandInfo{
	// Admin commands
	{true, "AdminKick", "kick", "AdminKick <NameOrSteamId> <KickReason>", "Kicks a player from the server.", AdminCommand},
	{true, "AdminKickById", "kick", "AdminKickById <PlayerId> <KickReason>", "Kicks a player with Id from the server.", AdminCommand},
	{true, "AdminBan", "ban", "AdminBan <NameOrSteamId> <BanLength> <BanReason>", "Bans a player from the server for a length of time. 0 = Perm, 1d = 1 Day, 1M = 1 Month, etc.", AdminCommand},
	{true, "AdminBanById", "ban", "AdminBanById <PlayerId> <BanLength> <BanReason>", "Bans player with Id from the server for length of time. 0 = Perm, 1d = 1 Day, 1M = 1 Month, etc.", AdminCommand},
	{true, "AdminBroadcast", "chat", "AdminBroadcast <Message>", "Send system message to all players on the server.", AdminCommand},
	{false, "ChatToAdmin", "chat", "ChatToAdmin <Message>", "Send system message to all admins on the server.", PublicCommand},
	{true, "AdminEndMatch", "pause", "AdminEndMatch", "Tell the server to immediately end the match.", AdminCommand},
	{false, "AdminPauseMatch", "pause", "AdminPauseMatch", "Tell the server to put the match on hold.", AdminCommand},
	{false, "AdminUnpauseMatch", "pause", "AdminUnpauseMatch", "Tell the server to take off the hold.", AdminCommand},
	{true, "AdminChangeLayer", "changemap", "AdminChangeLayer <LayerName>", "Change the layer and travel to it immediately.", AdminCommand},
	{true, "AdminSetNextLayer", "changemap", "AdminSetNextLayer <LayerName>", "Set the next layer to travel to after this match ends.", AdminCommand},
	{true, "AdminSetMaxNumPlayers", "config", "AdminSetMaxNumPlayers <NumPlayers>", "Set the maximum number of players for this server.", AdminCommand},
	{true, "AdminSetServerPassword", "private", "AdminSetServerPassword <Password>", "Set the password for a server or use \"\" to remove it.", AdminCommand},
	{true, "AdminSlomo", "cheat", "AdminSlomo <TimeDilation>", "Set the clock speed on the server 0.1 is 10% of normal speed 2.0 is twice the normal speed.", AdminCommand},
	{true, "AdminForceTeamChange", "forceteamchange", "AdminForceTeamChange <NameOrSteamId>", "Changes a player's team.", AdminCommand},
	{true, "AdminForceTeamChangeById", "forceteamchange", "AdminForceTeamChangeById <PlayerId>", "Changes a player with a certain id's team.", AdminCommand},
	{false, "AdminForceAllDeployableAvailability", "cheat", "AdminForceAllDeployableAvailability <0>|<1>", "Sets the server to ignore placement rules for deployables.", AdminCommand},
	{false, "AdminNoRespawnTimer", "cheat", "AdminNoRespawnTimer <0>|<1>", "Layer based setting, disables respawn timer.", AdminCommand},
	{false, "AdminNoTeamChangeTimer", "cheat", "AdminNoTeamChangeTimer <0>|<1>", "Layer based setting, disables team change timer.", AdminCommand},
	{false, "AdminDisableVehicleClaiming", "changemap", "AdminDisableVehicleClaiming <0>|<1>", "Sets the server to disable vehicle claiming.", AdminCommand},
	{false, "AdminForceAllRoleAvailability", "cheat", "AdminForceAllRoleAvailability <0>|<1>", "Sets the server to ignore kit restrictions.", AdminCommand},
	{false, "AdminNetTestStart", "debug", "AdminNetTestStart", "Starts the network test and prints it to the clients logs.", AdminCommand},
	{false, "AdminNetTestStop", "debug", "AdminNetTestStop", "Stops the network test and prints it to the clients logs.", AdminCommand},
	{true, "AdminListDisconnectedPlayers", "kick", "AdminListDisconnectedPlayers", "List recently disconnected player ids with associated player name and SteamId.", AdminCommand},
	{false, "TraceViewToggle", "FeatureTest", "TraceViewToggle", "Runs a trace from center of screen out to any objects and displays information about that object.", PublicCommand},
	{false, "AdminCreateVehicle", "FeatureTest", "AdminCreateVehicle <Vehiclelink>", "Allows you to spawn a vehicle on an unlicensed servers or on a local server.", AdminCommand},
	{true, "AdminDemoteCommander", "kick", "AdminDemoteCommander <PlayerName>", "Demote a commander specified by player name or Steam Id.", AdminCommand},
	{true, "AdminDemoteCommanderById", "kick", "AdminDemoteCommander <PlayerId>", "Demote a commander with Id from the server.", AdminCommand},
	{true, "AdminDisbandSquad", "kick", "AdminDisbandSquad <TeamNumber = [1|2]> <SquadIndex>", "Disbands the specified Squad (Which team 1 or 2 you will see on the team screen).", AdminCommand},
	{true, "AdminRemovePlayerFromSquad", "kick", "AdminRemovePlayerFromSquad <PlayerName>", "Remove a player from their squad without kicking them.", AdminCommand},
	{true, "AdminRemovePlayerFromSquadById", "kick", "AdminRemovePlayerFromSquad <PlayerId>", "Remove a player from their squad without kicking them via Id.", AdminCommand},
	{true, "AdminWarn", "kick", "AdminWarn <NameOrSteamId> <WarnReason>", "Warns a player from the server for being abusive.", AdminCommand},
	{true, "AdminWarnById", "kick", "AdminWarnById <PlayerId> <WarnReason>", "Warns a player with Id from the server for being abusive.", AdminCommand},
	{false, "AdminForceNetUpdateOnClientSaturation", "debug", "AdminForceNetUpdateOnClientSaturation <0>|<1>", "If true, when a connection becomes saturated, all remaining actors that couldn't complete replication will have ForceNetUpdate called on them.", AdminCommand},
	{false, "AdminProfileServer", "debug", "AdminProfileServer <SecondsToProfileFor> <0>|<1>", "Starts profiling on the server for a fixed length of time, after which the profiling data is saved to disk.", AdminCommand},
	{true, "AdminRestartMatch", "pause", "AdminRestartMatch", "Tell the server to restart the match.", AdminCommand},
	{false, "AdminSetPublicQueueLimit", "config", "AdminSetPublicQueueLimit <value>", "=25 will cap public queue at 25. =0 means that there won't be public queue so non admins and all other players won't be able to join. =-1 is unlimited queue.", AdminCommand},

	// Public commands
	{true, "ListCommands", "help", "ListCommands <1>|<0>", "Prints out the information for all commands in the game. ", PublicCommand},
	{true, "ShowCommandInfo", "help", "ShowCommandInfo <CommandName>", "Prints out the information for a specific command.", PublicCommand},
	{true, "ShowNextMap", "info", "ShowNextMap", "Shows the next map in rotation.", PublicCommand},
	{true, "ShowCurrentMap", "info", "ShowCurrentMap", "Shows the current map.", PublicCommand},
	{true, "ListPlayers", "info", "ListPlayers", "Lists all players on the server.", PublicCommand},
	{true, "ListSquads", "info", "ListSquads", "Lists all squads on the server.", PublicCommand},
	{true, "ListLayers", "info", "ListLayers", "Prints out the list of available layers.", PublicCommand},
}

// GetCommandsByType returns commands filtered by their type
func GetCommandsByType(commandType CommandType) []CommandInfo {
	var filtered []CommandInfo
	for _, cmd := range CommandMatrix {
		if cmd.CommandType == commandType {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

// GetCommandsByCategory returns commands filtered by their category
func GetCommandsByCategory(category string) []CommandInfo {
	var filtered []CommandInfo
	for _, cmd := range CommandMatrix {
		if cmd.Category == category {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

// GetCommandByName returns a command by its name
func GetCommandByName(name string) (CommandInfo, bool) {
	for _, cmd := range CommandMatrix {
		if cmd.Name == name {
			return cmd, true
		}
	}
	return CommandInfo{}, false
}

// CommandsCanExecute returns a list of commands that the user can execute
func CommandsCanExecute(permissions []string, supportsRCON bool) []CommandInfo {
	commands := []CommandInfo{}

	for _, command := range CommandMatrix {
		if command.SupportsRCON == supportsRCON && slices.Contains(permissions, command.Name) {
			commands = append(commands, command)
		}
	}

	return commands
}
