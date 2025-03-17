package connector_manager

import (
	"github.com/google/uuid"
)

// NewConnectorBase creates a new ConnectorBase with the given ID, config, and definition
func NewConnectorBase(id uuid.UUID, config map[string]interface{}, def ConnectorDefinition) ConnectorBase {
	return ConnectorBase{
		ID:         id,
		Config:     config,
		Definition: def,
	}
}

// StandardConnectorFactory creates a standard factory function for creating connector instances
func StandardConnectorFactory(
	builder func(id uuid.UUID, config map[string]interface{}, def ConnectorDefinition) (Connector, error),
	getDef func() ConnectorDefinition,
) func(id uuid.UUID, config map[string]interface{}) (Connector, error) {
	return func(id uuid.UUID, config map[string]interface{}) (Connector, error) {
		def := getDef()
		return builder(id, config, def)
	}
}
