package auto_tk_warn

import "go.codycody31.dev/squad-aegis/internal/extension_manager"

// AutoTKWarnRegistrar implements the ExtensionRegistrar interface
type AutoTKWarnRegistrar struct{}

func (r AutoTKWarnRegistrar) Define() extension_manager.ExtensionDefinition {
	return Define()
}
