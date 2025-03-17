package intervalled_broadcasts

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// IntervalledBroadcastsRegistrar implements the ExtensionRegistrar interface
type IntervalledBroadcastsRegistrar struct{}

func (r IntervalledBroadcastsRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
