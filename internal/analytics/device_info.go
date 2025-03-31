package analytics

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"go.codycody31.dev/squad-aegis/shared/config"
)

// DeviceInfo represents information about the device running the application
type DeviceInfo struct {
	OS          string                 `json:"os"`
	OSArch      string                 `json:"os_arch"`
	OSVersion   string                 `json:"os_version"`
	DeviceName  string                 `json:"device_name"`
	CPUCount    int                    `json:"cpu_count"`
	MemoryTotal uint64                 `json:"memory_total"`
	Metrics     map[string]interface{} `json:"metrics"`
}

// GetDeviceInfo collects comprehensive information about the device
func GetDeviceInfo(anonymous bool) DeviceInfo {
	info := DeviceInfo{
		OS:       runtime.GOOS,
		OSArch:   runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
		Metrics:  make(map[string]interface{}),
	}

	if !anonymous {
		info.DeviceName = getDeviceName()
	}

	// Add OS-specific information
	switch runtime.GOOS {
	case "linux":
		info.OSVersion = getLinuxVersion()
		info.MemoryTotal = getLinuxMemoryTotal()
	case "darwin":
		info.OSVersion = getDarwinVersion()
		info.MemoryTotal = getDarwinMemoryTotal()
	case "windows":
		info.OSVersion = getWindowsVersion()
		info.MemoryTotal = getWindowsMemoryTotal()
	}

	// Add common metrics
	if !anonymous {
		info.Metrics["hostname"] = getHostname()
	}
	info.Metrics["container"] = isRunningInContainer()
	info.Metrics["env"] = getEnvironment()

	return info
}

// getDeviceName returns a human-readable device name
func getDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// getHostname returns the system hostname
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// isRunningInContainer checks if the application is running in a container
func isRunningInContainer() bool {
	// Check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check cgroup
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		return bytes.Contains(data, []byte("docker")) || bytes.Contains(data, []byte("lxc"))
	}

	return config.Config.App.InContainer
}

// getEnvironment returns the current environment
func getEnvironment() string {
	if config.Config.App.IsDevelopment {
		return "development"
	}
	return "production"
}

// getLinuxVersion returns the Linux kernel version
func getLinuxVersion() string {
	if data, err := os.ReadFile("/proc/version"); err == nil {
		return string(bytes.Split(data, []byte(" "))[2])
	}
	return "unknown"
}

// getDarwinVersion returns the macOS version
func getDarwinVersion() string {
	cmd := exec.Command("sw_vers", "-productVersion")
	if output, err := cmd.Output(); err == nil {
		return string(bytes.TrimSpace(output))
	}
	return "unknown"
}

// getWindowsVersion returns the Windows version
func getWindowsVersion() string {
	cmd := exec.Command("cmd", "/c", "ver")
	if output, err := cmd.Output(); err == nil {
		return string(bytes.TrimSpace(output))
	}
	return "unknown"
}

// getLinuxMemoryTotal returns total memory in bytes on Linux
func getLinuxMemoryTotal() uint64 {
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if mem, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
						return mem * 1024 // Convert from KB to bytes
					}
				}
			}
		}
	}
	return 0
}

// getDarwinMemoryTotal returns total memory in bytes on macOS
func getDarwinMemoryTotal() uint64 {
	cmd := exec.Command("sysctl", "hw.memsize")
	if output, err := cmd.Output(); err == nil {
		fields := strings.Fields(string(output))
		if len(fields) >= 2 {
			if mem, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				return mem
			}
		}
	}
	return 0
}

// getWindowsMemoryTotal returns total memory in bytes on Windows
func getWindowsMemoryTotal() uint64 {
	cmd := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			if mem, err := strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64); err == nil {
				return mem
			}
		}
	}
	return 0
}
