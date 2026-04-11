package plugin_manager

import (
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

func selectedConnectorManifestTarget(manifest ConnectorPackageManifest) (PluginPackageTarget, error) {
	return selectedManifestTarget(manifest.asPluginManifest())
}

func validateConnectorManifest(manifest *ConnectorPackageManifest) error {
	if strings.TrimSpace(manifest.ConnectorID) == "" {
		return fmt.Errorf("connector manifest is missing connector_id")
	}
	if manifest.Name == "" {
		return fmt.Errorf("connector manifest is missing name")
	}
	if manifest.Version == "" {
		return fmt.Errorf("connector manifest is missing version")
	}
	if _, _, err := pluginRuntimeSegments(manifest.ConnectorID, manifest.Version); err != nil {
		return err
	}
	if manifest.EntrySymbol == "" {
		manifest.EntrySymbol = nativeConnectorEntrySymbol
	}
	if manifest.EntrySymbol != nativeConnectorEntrySymbol {
		return fmt.Errorf("unsupported connector entry symbol %s", manifest.EntrySymbol)
	}

	targets := clonePluginPackageTargets(manifest.Targets)
	if len(targets) == 0 {
		return fmt.Errorf("connector manifest is missing targets")
	}

	seenTargets := make(map[string]struct{}, len(targets))
	for index, target := range targets {
		if target.MinHostAPIVersion <= 0 {
			return fmt.Errorf("connector manifest target %d is missing min_host_api_version", index)
		}
		if target.TargetOS == "" {
			return fmt.Errorf("connector manifest target %d is missing target_os", index)
		}
		if target.TargetArch == "" {
			return fmt.Errorf("connector manifest target %d is missing target_arch", index)
		}
		if strings.TrimSpace(target.LibraryPath) == "" {
			return fmt.Errorf("connector manifest target %d is missing library_path", index)
		}
		if strings.TrimSpace(target.SHA256) == "" {
			return fmt.Errorf("connector manifest target %d is missing sha256", index)
		}
		requiredCapabilitySet := make(map[string]struct{}, len(target.RequiredCapabilities))
		requiredCapabilities := cloneRequiredCapabilities(target.RequiredCapabilities)
		sort.Strings(requiredCapabilities)
		for _, capability := range requiredCapabilities {
			capability = strings.TrimSpace(capability)
			if capability == "" {
				return fmt.Errorf("connector manifest target %d has an empty required capability", index)
			}
			if _, exists := requiredCapabilitySet[capability]; exists {
				return fmt.Errorf("connector manifest target %d repeats required capability %s", index, capability)
			}
			requiredCapabilitySet[capability] = struct{}{}
		}

		key := target.TargetOS + "|" + target.TargetArch + "|" + strconv.Itoa(target.MinHostAPIVersion) + "|" + strings.Join(requiredCapabilities, ",")
		if _, exists := seenTargets[key]; exists {
			return fmt.Errorf("connector manifest has a duplicate target for %s/%s with min_host_api_version %d and capabilities %s", target.TargetOS, target.TargetArch, target.MinHostAPIVersion, strings.Join(requiredCapabilities, ","))
		}
		seenTargets[key] = struct{}{}
	}

	return nil
}

func validateConnectorCompatibility(manifest ConnectorPackageManifest, target PluginPackageTarget) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("native connectors are only supported on Linux")
	}
	if target.TargetOS != runtime.GOOS {
		return fmt.Errorf("connector targets %s but host is %s", target.TargetOS, runtime.GOOS)
	}
	if target.TargetArch != runtime.GOARCH {
		return fmt.Errorf("connector targets %s but host architecture is %s", target.TargetArch, runtime.GOARCH)
	}
	if target.MinHostAPIVersion > NativeConnectorHostAPIVersion {
		return fmt.Errorf("connector requires host API version >= %d, but host provides %d", target.MinHostAPIVersion, NativeConnectorHostAPIVersion)
	}
	if missing := missingRequiredCapabilities(target.RequiredCapabilities); len(missing) > 0 {
		return fmt.Errorf("connector requires unsupported host capabilities: %s", strings.Join(missing, ", "))
	}
	return nil
}
