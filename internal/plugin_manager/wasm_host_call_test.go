package plugin_manager

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

type wasmTestServer struct {
	id uuid.UUID
}

func (s wasmTestServer) GetServerID() uuid.UUID {
	return s.id
}

func (s wasmTestServer) GetServerInfo() (*ServerInfo, error) {
	return &ServerInfo{ID: s.id, Name: "test"}, nil
}

func (s wasmTestServer) GetPlayers() ([]*PlayerInfo, error) {
	return nil, nil
}

func (s wasmTestServer) GetAdmins() ([]*AdminInfo, error) {
	return nil, nil
}

func (s wasmTestServer) GetSquads() ([]*SquadInfo, error) {
	return nil, nil
}

func TestWasmHostDispatchServer_GetServerID(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	p := &wasmPlugin{
		def: PluginDefinition{
			RequiredCapabilities: []string{NativePluginCapabilityAPIServer},
		},
		apis: &PluginAPIs{ServerAPI: wasmTestServer{id: id}},
	}
	data, err := p.wasmHostDispatchServer("GetServerID", nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := data.(map[string]string)
	if !ok || m["server_id"] != id.String() {
		t.Fatalf("unexpected data: %v", data)
	}
}

func TestWasmHostDispatchDatabase_RoundTrip(t *testing.T) {
	t.Parallel()
	store := make(map[string]string)
	p := &wasmPlugin{
		def: PluginDefinition{
			RequiredCapabilities: []string{NativePluginCapabilityAPIDatabase},
		},
		apis: &PluginAPIs{DatabaseAPI: wasmTestDB{store: store}},
	}
	if _, err := p.wasmHostDispatchDatabase("SetPluginData", json.RawMessage(`{"key":"k","value":"v"}`)); err != nil {
		t.Fatal(err)
	}
	data, err := p.wasmHostDispatchDatabase("GetPluginData", json.RawMessage(`{"key":"k"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := data.(map[string]string)
	if m["value"] != "v" {
		t.Fatalf("got %v", m)
	}
}

type wasmTestDB struct {
	store map[string]string
}

func (d wasmTestDB) GetPluginData(key string) (string, error) {
	return d.store[key], nil
}

func (d wasmTestDB) SetPluginData(key, value string) error {
	d.store[key] = value
	return nil
}

func (d wasmTestDB) DeletePluginData(key string) error {
	delete(d.store, key)
	return nil
}

func TestWasmHostCategoryCapabilityCompleteness(t *testing.T) {
	t.Parallel()
	required := []string{
		NativePluginCapabilityAPIServer,
		NativePluginCapabilityAPIRCON,
		NativePluginCapabilityAPIDatabase,
		NativePluginCapabilityAPIRule,
		NativePluginCapabilityAPIAdmin,
		NativePluginCapabilityAPIDiscord,
		NativePluginCapabilityAPIEvent,
	}
	for _, cap := range required {
		found := false
		for _, v := range wasmHostCategoryCapability {
			if v == cap {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("capability %s not mapped in wasmHostCategoryCapability", cap)
		}
	}
}

func TestConnectorDefinitionHasCapability(t *testing.T) {
	t.Parallel()
	def := ConnectorDefinition{
		ID:                   "x",
		RequiredCapabilities: []string{NativePluginCapabilityAPILog},
	}
	if !connectorDefinitionHasCapability(def, NativePluginCapabilityAPILog) {
		t.Fatal("expected log capability")
	}
	if connectorDefinitionHasCapability(def, NativePluginCapabilityAPIRCON) {
		t.Fatal("unexpected rcon")
	}
}
