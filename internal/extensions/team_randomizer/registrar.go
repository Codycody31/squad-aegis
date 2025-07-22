package team_randomizer

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// TeamRandomizerRegistrar implements the ExtensionRegistrar interface
type TeamRandomizerRegistrar struct{}

func (r TeamRandomizerRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
