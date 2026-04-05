package plugin_manager

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// WASM plugin v1 guest contract (connector packages use the same manifest wasm_abi_version number;
// see wasm_connector.go for connector-specific exports):
//   - Import module "aegis_host_v1": log(i32 level, i32 ptr, i32 len)
//     level 0=info, 1=warn, 2=error, other=debug. Payload is UTF-8 text (JSON recommended).
//   - Optionally import connector_invoke (only if manifest required_capabilities includes api.connector):
//     connector_invoke(id_ptr, id_len, req_ptr, req_len, out_ptr, out_cap, out_written_ptr) -> i32
//     Returns 0 on success; writes ConnectorInvokeResponse JSON to out_ptr and length at out_written_ptr.
//   - Exports: memory, aegis_init(config_ptr, config_len) -> i32, aegis_start() -> i32, aegis_stop() -> i32,
//     aegis_on_event(type_ptr, type_len, payload_ptr, payload_len) -> i32 (0 = success).
//
// Denied v1: direct RCON, database, rules, admin, Discord, server mutation — use bundled/native plugins or future ABI versions.

func wasmPluginsEnabled() bool {
	return nativePluginsEnabled() && config.Config != nil && config.Config.Plugins.WasmEnabled
}

func pluginManifestKindIsWasm(manifest PluginPackageManifest) bool {
	return strings.EqualFold(strings.TrimSpace(manifest.Kind), "wasm")
}

func selectWasmPluginTarget(manifest PluginPackageManifest) (PluginPackageTarget, error) {
	targets := clonePluginPackageTargets(manifest.Targets)
	if len(targets) == 0 {
		return PluginPackageTarget{}, fmt.Errorf("plugin manifest is missing targets")
	}

	var compatible []PluginPackageTarget
	minRequiredHostAPI := 0
	missingCapabilities := make(map[string]struct{})

	for _, target := range targets {
		target.RequiredCapabilities = cloneRequiredCapabilities(target.RequiredCapabilities)
		if target.TargetOS != "wasm" || target.TargetArch != "wasm" {
			continue
		}

		if target.MinHostAPIVersion > NativePluginHostAPIVersion {
			if minRequiredHostAPI == 0 || target.MinHostAPIVersion < minRequiredHostAPI {
				minRequiredHostAPI = target.MinHostAPIVersion
			}
			continue
		}

		for _, capability := range missingRequiredCapabilities(target.RequiredCapabilities) {
			missingCapabilities[capability] = struct{}{}
		}
		if len(missingRequiredCapabilities(target.RequiredCapabilities)) > 0 {
			continue
		}

		compatible = append(compatible, target)
	}

	if len(compatible) > 0 {
		best := compatible[0]
		for _, target := range compatible[1:] {
			if betterPluginTarget(target, best) {
				best = target
			}
		}
		return best, nil
	}

	missingCapabilityList := make([]string, 0, len(missingCapabilities))
	for capability := range missingCapabilities {
		missingCapabilityList = append(missingCapabilityList, capability)
	}
	sort.Strings(missingCapabilityList)

	if minRequiredHostAPI > 0 && len(missingCapabilityList) > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"wasm plugin has no compatible target: requires host API version >= %d and capabilities %s (host API version is %d)",
			minRequiredHostAPI,
			strings.Join(missingCapabilityList, ", "),
			NativePluginHostAPIVersion,
		)
	}
	if minRequiredHostAPI > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"wasm plugin requires host API version >= %d, but host provides %d",
			minRequiredHostAPI,
			NativePluginHostAPIVersion,
		)
	}
	if len(missingCapabilityList) > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"wasm plugin requires unsupported host capabilities: %s",
			strings.Join(missingCapabilityList, ", "),
		)
	}

	return PluginPackageTarget{}, fmt.Errorf("wasm plugin has no wasm/wasm target")
}

func (pm *PluginManager) loadWasmPluginPackage(pkg *InstalledPluginPackage) error {
	manifest := pkg.Manifest

	events := make([]event_manager.EventType, 0, len(manifest.WasmEvents))
	for _, e := range manifest.WasmEvents {
		events = append(events, event_manager.EventType(e))
	}

	author := strings.TrimSpace(manifest.Author)
	if author == "" {
		author = "WebAssembly package"
	}

	defForInstance := PluginDefinition{
		ID:                     manifest.PluginID,
		Name:                   manifest.Name,
		Description:            manifest.Description,
		Version:                manifest.Version,
		Author:                 author,
		Source:                 PluginSourceWasm,
		Official:               pkg.Official,
		InstallState:           pkg.InstallState,
		Distribution:           pkg.Distribution,
		MinHostAPIVersion:      pkg.MinHostAPIVersion,
		RequiredCapabilities:   cloneRequiredCapabilities(pkg.RequiredCapabilities),
		TargetOS:               pkg.TargetOS,
		TargetArch:             pkg.TargetArch,
		RuntimePath:            pkg.RuntimePath,
		SignatureVerified:      pkg.SignatureVerified,
		Unsafe:                 pkg.Unsafe,
		AllowMultipleInstances: manifest.AllowMultipleInstances,
		RequiredConnectors:     append([]string(nil), manifest.ManifestRequiredConnectors...),
		OptionalConnectors:     append([]string(nil), manifest.ManifestOptionalConnectors...),
		ConfigSchema:           manifest.ConfigSchema,
		Events:                 events,
		LongRunning:            manifest.LongRunning,
		CreateInstance:         nil,
	}

	runtimePath := pkg.RuntimePath
	registration := defForInstance
	registration.CreateInstance = func() Plugin {
		d := defForInstance
		return newWasmPlugin(d, runtimePath)
	}

	if err := pm.registry.RegisterPlugin(registration); err != nil {
		return fmt.Errorf("failed to register wasm plugin: %w", err)
	}

	pm.nativeMu.Lock()
	pm.loadedNativePlugins[pkg.PluginID] = pkg.Version
	pm.nativeMu.Unlock()

	return nil
}

func validateWasmPluginManifestFields(manifest PluginPackageManifest) error {
	if manifest.WasmABIVersion != WasmPluginHostABIVersion {
		return fmt.Errorf("unsupported wasm_abi_version %d (host supports %d)", manifest.WasmABIVersion, WasmPluginHostABIVersion)
	}
	return nil
}

func validateWasmTargetLibrarySuffix(target PluginPackageTarget, index int) error {
	lp := strings.TrimSpace(target.LibraryPath)
	if !strings.HasSuffix(strings.ToLower(lp), ".wasm") {
		return fmt.Errorf("plugin manifest target %d library_path for wasm must end in .wasm", index)
	}
	return nil
}

// wasmAllowedManifestCapabilities rejects capabilities wasm v1 cannot implement via imports.
func wasmAllowedManifestCapabilities(caps []string, targetIndex int) error {
	denied := []string{
		NativePluginCapabilityAPIRCON,
		NativePluginCapabilityAPIServer,
		NativePluginCapabilityAPIDatabase,
		NativePluginCapabilityAPIRule,
		NativePluginCapabilityAPIAdmin,
		NativePluginCapabilityAPIDiscord,
		NativePluginCapabilityAPIEvent,
	}
	deniedSet := make(map[string]struct{}, len(denied))
	for _, d := range denied {
		deniedSet[d] = struct{}{}
	}
	for _, c := range caps {
		c = strings.TrimSpace(c)
		if c == "" {
			return fmt.Errorf("plugin manifest target %d has an empty required capability", targetIndex)
		}
		if _, bad := deniedSet[c]; bad {
			return fmt.Errorf("wasm plugin target %d cannot require capability %s in v1 (use native/bundled for full APIs)", targetIndex, c)
		}
	}
	return nil
}

func wasmManifestTargetKey(target PluginPackageTarget) string {
	requiredCapabilities := cloneRequiredCapabilities(target.RequiredCapabilities)
	sort.Strings(requiredCapabilities)
	return target.TargetOS + "|" + target.TargetArch + "|" + strconv.Itoa(target.MinHostAPIVersion) + "|" + strings.Join(requiredCapabilities, ",")
}
