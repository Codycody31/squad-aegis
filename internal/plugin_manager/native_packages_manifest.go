package plugin_manager

import (
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

func clonePluginPackageTargets(targets []PluginPackageTarget) []PluginPackageTarget {
	if len(targets) == 0 {
		return nil
	}

	cloned := make([]PluginPackageTarget, len(targets))
	copy(cloned, targets)
	return cloned
}

func cloneRequiredCapabilities(capabilities []string) []string {
	if len(capabilities) == 0 {
		return nil
	}

	cloned := make([]string, len(capabilities))
	copy(cloned, capabilities)
	return cloned
}

func hostNativePluginCapabilitiesSet() map[string]struct{} {
	capabilities := make(map[string]struct{}, len(nativePluginHostCapabilities))
	for _, capability := range NativePluginHostCapabilities() {
		capabilities[capability] = struct{}{}
	}
	return capabilities
}

func missingRequiredCapabilities(required []string) []string {
	if len(required) == 0 {
		return nil
	}

	hostCapabilities := hostNativePluginCapabilitiesSet()
	missingSet := make(map[string]struct{})
	for _, capability := range required {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		if _, ok := hostCapabilities[capability]; !ok {
			missingSet[capability] = struct{}{}
		}
	}

	missing := make([]string, 0, len(missingSet))
	for capability := range missingSet {
		missing = append(missing, capability)
	}
	sort.Strings(missing)
	return missing
}

func isCompatibleHostTarget(target PluginPackageTarget) bool {
	return target.MinHostAPIVersion <= NativePluginHostAPIVersion && len(missingRequiredCapabilities(target.RequiredCapabilities)) == 0
}

func betterPluginTarget(left, right PluginPackageTarget) bool {
	if left.MinHostAPIVersion != right.MinHostAPIVersion {
		return left.MinHostAPIVersion > right.MinHostAPIVersion
	}
	if len(left.RequiredCapabilities) != len(right.RequiredCapabilities) {
		return len(left.RequiredCapabilities) > len(right.RequiredCapabilities)
	}
	return left.LibraryPath < right.LibraryPath
}

func preferredPluginTarget(targets []PluginPackageTarget) PluginPackageTarget {
	if len(targets) == 0 {
		return PluginPackageTarget{}
	}

	hostOS := runtime.GOOS
	hostArch := runtime.GOARCH
	var bestCompatible *PluginPackageTarget
	var bestHostTarget *PluginPackageTarget
	var bestLinuxAMD64 *PluginPackageTarget

	for _, target := range targets {
		target.RequiredCapabilities = cloneRequiredCapabilities(target.RequiredCapabilities)

		if target.TargetOS == hostOS && target.TargetArch == hostArch {
			if bestHostTarget == nil || betterPluginTarget(target, *bestHostTarget) {
				targetCopy := target
				bestHostTarget = &targetCopy
			}
			if isCompatibleHostTarget(target) && (bestCompatible == nil || betterPluginTarget(target, *bestCompatible)) {
				targetCopy := target
				bestCompatible = &targetCopy
			}
		}

		if target.TargetOS == "linux" && target.TargetArch == "amd64" {
			if bestLinuxAMD64 == nil || betterPluginTarget(target, *bestLinuxAMD64) {
				targetCopy := target
				bestLinuxAMD64 = &targetCopy
			}
		}
	}

	if bestCompatible != nil {
		return *bestCompatible
	}
	if bestHostTarget != nil {
		return *bestHostTarget
	}
	if bestLinuxAMD64 != nil {
		return *bestLinuxAMD64
	}
	return targets[0]
}

func selectedManifestTarget(manifest PluginPackageManifest) (PluginPackageTarget, error) {
	targets := clonePluginPackageTargets(manifest.Targets)
	if len(targets) == 0 {
		return PluginPackageTarget{}, fmt.Errorf("plugin manifest is missing targets")
	}

	hostOS := runtime.GOOS
	hostArch := runtime.GOARCH
	var osMatches []PluginPackageTarget
	var archMatches []PluginPackageTarget
	var compatible []PluginPackageTarget
	minRequiredHostAPI := 0
	missingCapabilities := make(map[string]struct{})

	for _, target := range targets {
		target.RequiredCapabilities = cloneRequiredCapabilities(target.RequiredCapabilities)
		if target.TargetOS != hostOS {
			continue
		}
		osMatches = append(osMatches, target)
		if target.TargetArch != hostArch {
			continue
		}
		archMatches = append(archMatches, target)

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

	if len(osMatches) == 0 {
		return PluginPackageTarget{}, fmt.Errorf("plugin does not support host OS %s", hostOS)
	}
	if len(archMatches) == 0 {
		return PluginPackageTarget{}, fmt.Errorf("plugin does not support host architecture %s on %s", hostArch, hostOS)
	}

	missingCapabilityList := make([]string, 0, len(missingCapabilities))
	for capability := range missingCapabilities {
		missingCapabilityList = append(missingCapabilityList, capability)
	}
	sort.Strings(missingCapabilityList)

	if minRequiredHostAPI > 0 && len(missingCapabilityList) > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"plugin has no compatible target for %s/%s: requires host API version >= %d and capabilities %s (host API version is %d)",
			hostOS,
			hostArch,
			minRequiredHostAPI,
			strings.Join(missingCapabilityList, ", "),
			NativePluginHostAPIVersion,
		)
	}
	if minRequiredHostAPI > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"plugin requires host API version >= %d, but host provides %d",
			minRequiredHostAPI,
			NativePluginHostAPIVersion,
		)
	}
	if len(missingCapabilityList) > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"plugin requires unsupported host capabilities: %s",
			strings.Join(missingCapabilityList, ", "),
		)
	}

	return PluginPackageTarget{}, fmt.Errorf("plugin has no compatible target for %s/%s", hostOS, hostArch)
}

func validatePluginManifest(manifest PluginPackageManifest) error {
	if manifest.PluginID == "" {
		return fmt.Errorf("plugin manifest is missing plugin_id")
	}
	if manifest.Name == "" {
		return fmt.Errorf("plugin manifest is missing name")
	}
	if manifest.Version == "" {
		return fmt.Errorf("plugin manifest is missing version")
	}
	if _, _, err := pluginRuntimeSegments(manifest.PluginID, manifest.Version); err != nil {
		return err
	}

	if manifest.EntrySymbol == "" {
		manifest.EntrySymbol = nativePluginEntrySymbol
	}
	if manifest.EntrySymbol != nativePluginEntrySymbol {
		return fmt.Errorf("unsupported plugin entry symbol %s", manifest.EntrySymbol)
	}

	targets := clonePluginPackageTargets(manifest.Targets)
	if len(targets) == 0 {
		return fmt.Errorf("plugin manifest is missing targets")
	}

	seenTargets := make(map[string]struct{}, len(targets))
	for index, target := range targets {
		if target.MinHostAPIVersion <= 0 {
			return fmt.Errorf("plugin manifest target %d is missing min_host_api_version", index)
		}
		if target.TargetOS == "" {
			return fmt.Errorf("plugin manifest target %d is missing target_os", index)
		}
		if target.TargetArch == "" {
			return fmt.Errorf("plugin manifest target %d is missing target_arch", index)
		}
		if strings.TrimSpace(target.LibraryPath) == "" {
			return fmt.Errorf("plugin manifest target %d is missing library_path", index)
		}
		// sha256 binds the signed manifest to the .so bytes.
		if strings.TrimSpace(target.SHA256) == "" {
			return fmt.Errorf("plugin manifest target %d is missing sha256", index)
		}

		requiredCapabilitySet := make(map[string]struct{}, len(target.RequiredCapabilities))
		requiredCapabilities := cloneRequiredCapabilities(target.RequiredCapabilities)
		sort.Strings(requiredCapabilities)
		for _, capability := range requiredCapabilities {
			capability = strings.TrimSpace(capability)
			if capability == "" {
				return fmt.Errorf("plugin manifest target %d has an empty required capability", index)
			}
			if _, exists := requiredCapabilitySet[capability]; exists {
				return fmt.Errorf("plugin manifest target %d repeats required capability %s", index, capability)
			}
			requiredCapabilitySet[capability] = struct{}{}
		}

		key := target.TargetOS + "|" + target.TargetArch + "|" + strconv.Itoa(target.MinHostAPIVersion) + "|" + strings.Join(requiredCapabilities, ",")
		if _, exists := seenTargets[key]; exists {
			return fmt.Errorf("plugin manifest has a duplicate target for %s/%s with min_host_api_version %d and capabilities %s", target.TargetOS, target.TargetArch, target.MinHostAPIVersion, strings.Join(requiredCapabilities, ","))
		}
		seenTargets[key] = struct{}{}
	}

	return nil
}

func validatePluginCompatibility(manifest PluginPackageManifest, target PluginPackageTarget) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("native plugins are only supported on Linux")
	}
	if target.TargetOS != runtime.GOOS {
		return fmt.Errorf("plugin targets %s but host is %s", target.TargetOS, runtime.GOOS)
	}
	if target.TargetArch != runtime.GOARCH {
		return fmt.Errorf("plugin targets %s but host architecture is %s", target.TargetArch, runtime.GOARCH)
	}
	if target.MinHostAPIVersion > NativePluginHostAPIVersion {
		return fmt.Errorf("plugin requires host API version >= %d, but host provides %d", target.MinHostAPIVersion, NativePluginHostAPIVersion)
	}
	if missing := missingRequiredCapabilities(target.RequiredCapabilities); len(missing) > 0 {
		return fmt.Errorf("plugin requires unsupported host capabilities: %s", strings.Join(missing, ", "))
	}

	return nil
}
