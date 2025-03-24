package auto_kick_unassigned

import (
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// AutoKickUnassignedExtension automatically kicks players that are not in a squad after a specified amount of time
type AutoKickUnassignedExtension struct {
	extension_manager.ExtensionBase
	Manager interface{} // Stores the AutoKickManager
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:                     "auto_kick_unassigned",
		Name:                   "Auto Kick Unassigned",
		Description:            "Automatically kicks players that are not in a squad after a specified amount of time.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,

		Dependencies: extension_manager.ExtensionDependencies{
			Required: []extension_manager.DependencyType{
				extension_manager.DependencyServer,
				extension_manager.DependencyConnectors,
				extension_manager.DependencyRconManager,
			},
		},

		// Required connectors
		RequiredConnectors: []string{"logwatcher"},

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "warning_message",
					Description: "Message that will be sent to players warning them they will be kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "Join a squad, you are unassigned and will be kicked",
				},
				{
					Name:        "kick_message",
					Description: "Message to send to players when they are kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "Unassigned - automatically removed",
				},
				{
					Name:        "frequency_of_warnings",
					Description: "How often in seconds should we warn the player about being unassigned.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     30,
				},
				{
					Name:        "unassigned_timer",
					Description: "How long in seconds to wait before an unassigned player is kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     360,
				},
				{
					Name:        "player_threshold",
					Description: "Player count required for AutoKick to start kicking players, set to -1 to disable.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     93,
				},
				{
					Name:        "round_start_delay",
					Description: "Time delay in seconds from start of the round before AutoKick starts kicking again.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     900,
				},
				{
					Name:        "ignore_admins",
					Description: "If true, admins will NOT be kicked. If false, admins WILL be kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
				{
					Name:        "ignore_whitelist",
					Description: "If true, reserve slot players will NOT be kicked. If false, reserve slot players WILL be kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
			},
		},

		EventHandlers: []extension_manager.EventHandler{
			{
				Source:      "LOGWATCHER",
				Name:        "NEW_GAME",
				Description: "Handles new game events to reset timers",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*AutoKickUnassignedExtension).handleNewGame(data)
				},
			},
			{
				Source:      "LOGWATCHER",
				Name:        "PLAYER_SQUAD_CHANGE",
				Description: "Handles player squad change events",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*AutoKickUnassignedExtension).handlePlayerSquadChange(data)
				},
			},
		},
		CreateInstance: func() extension_manager.Extension {
			return &AutoKickUnassignedExtension{}
		},
	}
}
