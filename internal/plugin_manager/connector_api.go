package plugin_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const maxConnectorInvokeDataJSONBytes = 256 * 1024

type connectorAPI struct {
	pm       *PluginManager
	pluginID string
}

func newConnectorAPI(pm *PluginManager, pluginID string) ConnectorAPI {
	return &connectorAPI{pm: pm, pluginID: pluginID}
}

func (api *connectorAPI) Call(ctx context.Context, connectorID string, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error) {
	if err := api.pm.checkPluginConnectorScope(api.pluginID, connectorID); err != nil {
		return &ConnectorInvokeResponse{V: ConnectorWireProtocolV1, OK: false, Error: err.Error()}, nil
	}
	return api.pm.invokeConnector(ctx, connectorID, req)
}

// checkPluginConnectorScope rejects calls to connectors the plugin did not
// declare in RequiredConnectors or OptionalConnectors. References are
// canonicalized via ResolveConnectorInstanceKey so a plugin that declared a
// LegacyID can still reach the connector under its current ID.
func (pm *PluginManager) checkPluginConnectorScope(pluginID, connectorID string) error {
	definition, err := pm.registry.GetPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("plugin %s definition not found: %w", pluginID, err)
	}
	requestedKey, ok := pm.ResolveConnectorInstanceKey(connectorID)
	if !ok {
		requestedKey = strings.TrimSpace(connectorID)
	}
	declared := make([]string, 0, len(definition.RequiredConnectors)+len(definition.OptionalConnectors))
	declared = append(declared, definition.RequiredConnectors...)
	declared = append(declared, definition.OptionalConnectors...)
	for _, ref := range declared {
		key, ok := pm.ResolveConnectorInstanceKey(ref)
		if !ok {
			key = strings.TrimSpace(ref)
		}
		if key != "" && key == requestedKey {
			return nil
		}
	}
	return fmt.Errorf("plugin %s did not declare connector %s in RequiredConnectors or OptionalConnectors", pluginID, connectorID)
}

// shouldExposePluginAPI is true for bundled plugins; native plugins must
// declare the matching api.* capability in their manifest target.
func (pm *PluginManager) shouldExposePluginAPI(pluginID string, capability string) bool {
	if pm == nil || pm.registry == nil {
		return false
	}
	definition, err := pm.registry.GetPlugin(pluginID)
	if err != nil {
		return false
	}
	enriched := pm.enrichPluginDefinition(*definition)
	if enriched.Source != PluginSourceNative {
		return true
	}
	requestedCapability := strings.TrimSpace(capability)
	if requestedCapability == "" {
		return false
	}
	for _, capability := range enriched.RequiredCapabilities {
		if strings.TrimSpace(capability) == requestedCapability {
			return true
		}
	}
	return false
}

// shouldExposeConnectorAPI is true for bundled plugins; native plugins must declare api.connector in their manifest target.
func (pm *PluginManager) shouldExposeConnectorAPI(pluginID string) bool {
	return pm.shouldExposePluginAPI(pluginID, NativePluginCapabilityAPIConnector)
}

// ResolveConnectorInstanceKey maps a canonical or legacy connector reference to the instance key used in pm.connectors.
func (pm *PluginManager) ResolveConnectorInstanceKey(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}

	pm.connectorMu.RLock()
	if _, ok := pm.connectors[ref]; ok {
		pm.connectorMu.RUnlock()
		return ref, true
	}
	pm.connectorMu.RUnlock()

	def, err := pm.connectorRegistry.GetConnector(ref)
	if err != nil {
		return "", false
	}

	pm.connectorMu.RLock()
	defer pm.connectorMu.RUnlock()

	keys := []string{def.ConnectorInstanceStorageKey()}
	keys = append(keys, def.LegacyIDs...)
	seen := make(map[string]struct{})
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		if _, ok := pm.connectors[k]; ok {
			return k, true
		}
	}

	return "", false
}

func connectorInvokeDataSize(data map[string]interface{}) int {
	if len(data) == 0 {
		return 0
	}
	b, err := json.Marshal(data)
	if err != nil {
		return maxConnectorInvokeDataJSONBytes + 1
	}
	return len(b)
}

func (pm *PluginManager) invokeConnector(ctx context.Context, connectorRef string, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error) {
	out := &ConnectorInvokeResponse{V: ConnectorWireProtocolV1, OK: false}
	if req == nil {
		out.Error = "request is nil"
		return out, nil
	}
	if req.V != ConnectorWireProtocolV1 {
		out.Error = fmt.Sprintf("unsupported connector invoke protocol version %q (supported: %q)", req.V, ConnectorWireProtocolV1)
		return out, nil
	}
	if connectorInvokeDataSize(req.Data) > maxConnectorInvokeDataJSONBytes {
		out.Error = "request data exceeds maximum size"
		return out, nil
	}

	instanceKey, ok := pm.ResolveConnectorInstanceKey(connectorRef)
	if !ok {
		out.Error = fmt.Sprintf("connector %q is not available", connectorRef)
		return out, nil
	}

	pm.connectorMu.RLock()
	instance := pm.connectors[instanceKey]
	pm.connectorMu.RUnlock()

	if instance == nil {
		out.Error = fmt.Sprintf("connector %q is not available", connectorRef)
		return out, nil
	}
	if instance.Status != ConnectorStatusRunning {
		out.Error = fmt.Sprintf("connector %q is not running", connectorRef)
		return out, nil
	}

	invokable, ok := instance.Connector.(InvokableConnector)
	if !ok {
		out.Error = "connector does not support JSON invoke"
		return out, nil
	}

	callCtx := ctx
	if _, hasDeadline := callCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(callCtx, 30*time.Second)
		defer cancel()
	}

	resp, err := invokable.Invoke(callCtx, req)
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}
	if resp == nil {
		out.Error = "connector returned nil response"
		return out, nil
	}
	if resp.V == "" {
		resp.V = ConnectorWireProtocolV1
	}
	return resp, nil
}
