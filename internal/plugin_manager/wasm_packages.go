package plugin_manager

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// WASM guest imports are documented in docs/wasm-guest-abi.md (import module aegis_host_v1).
// Capabilities match native plugins: required_capabilities on wasm/wasm targets gate host_call, log, and connector_invoke.

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

func wasmManifestTargetKey(target PluginPackageTarget) string {
	requiredCapabilities := cloneRequiredCapabilities(target.RequiredCapabilities)
	sort.Strings(requiredCapabilities)
	return target.TargetOS + "|" + target.TargetArch + "|" + strconv.Itoa(target.MinHostAPIVersion) + "|" + strings.Join(requiredCapabilities, ",")
}
