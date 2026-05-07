package plugin_manager

import (
	"context"
	"strings"
	"testing"

	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

type testConnector struct{}

func (testConnector) GetDefinition() ConnectorDefinition {
	return ConnectorDefinition{ID: "com.example.stub"}
}

func (testConnector) Initialize(map[string]interface{}) error { return nil }

func (testConnector) Start(context.Context) error { return nil }

func (testConnector) Stop() error { return nil }

func (testConnector) GetStatus() ConnectorStatus { return ConnectorStatusStopped }

func (testConnector) GetConfig() map[string]interface{} { return nil }

func (testConnector) UpdateConfig(map[string]interface{}) error { return nil }

func (testConnector) GetAPI() interface{} { return nil }

func TestConnectorRegistryAliasesLegacyID(t *testing.T) {
	t.Parallel()

	reg := NewConnectorRegistry()

	err := reg.RegisterConnector(ConnectorDefinition{
		ID:        "com.squad-aegis.connectors.discord",
		LegacyIDs: []string{"discord"},
		Name:      "Discord",
		Version:   "1",
		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{},
		},
		CreateInstance: func() Connector {
			return testConnector{}
		},
	})
	if err != nil {
		t.Fatalf("RegisterConnector: %v", err)
	}

	def, err := reg.GetConnector("discord")
	if err != nil {
		t.Fatalf("GetConnector(discord): %v", err)
	}
	if def.ID != "com.squad-aegis.connectors.discord" {
		t.Fatalf("ID = %q", def.ID)
	}

	_, err = reg.CreateConnectorInstance("com.squad-aegis.connectors.discord")
	if err != nil {
		t.Fatalf("CreateConnectorInstance(canonical): %v", err)
	}
}

func TestConnectorRegistryRejectsNativeAliasCollision(t *testing.T) {
	t.Parallel()

	reg := NewConnectorRegistry()
	if err := reg.RegisterConnector(ConnectorDefinition{
		ID:        "com.example.first",
		LegacyIDs: []string{"shared"},
		Name:      "First",
		Version:   "1",
		Source:    PluginSourceNative,
		CreateInstance: func() Connector {
			return testConnector{}
		},
	}); err != nil {
		t.Fatalf("RegisterConnector(first): %v", err)
	}

	err := reg.RegisterConnector(ConnectorDefinition{
		ID:          "com.example.second",
		Name:        "Second",
		Version:     "1",
		Source:      PluginSourceNative,
		InstanceKey: "shared",
		CreateInstance: func() Connector {
			return testConnector{}
		},
	})
	if err == nil {
		t.Fatal("RegisterConnector(second) error = nil, want alias collision")
	}
	if !strings.Contains(err.Error(), "collides with connector com.example.first") {
		t.Fatalf("RegisterConnector(second) error = %q, want collision with first connector", err)
	}
}
