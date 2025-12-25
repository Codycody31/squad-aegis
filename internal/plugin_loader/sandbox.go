package plugin_loader

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_sdk"
)

// SandboxConfig defines resource limits for plugins
type SandboxConfig struct {
	MaxMemoryMB     int64         // Maximum memory in MB
	MaxGoroutines   int32         // Maximum number of goroutines
	CPUTimeLimit    time.Duration // Maximum CPU time
	EnableMonitoring bool          // Enable resource monitoring
}

// DefaultSandboxConfig returns default sandbox configuration
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		MaxMemoryMB:      512,  // 512 MB
		MaxGoroutines:    100,  // 100 goroutines
		CPUTimeLimit:     0,    // No CPU time limit by default
		EnableMonitoring: true,
	}
}

// PluginSandbox wraps a plugin instance with resource monitoring and limits
type PluginSandbox struct {
	pluginID         string
	config           SandboxConfig
	gateway          *plugin_sdk.FeatureGateway
	
	// Resource tracking
	startTime        time.Time
	initialMemBytes  uint64
	goroutineCount   atomic.Int32
	violated         atomic.Bool
	violationReason  string
	
	// Control
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.RWMutex
	
	// Monitoring
	monitorTicker    *time.Ticker
	monitorDone      chan struct{}
}

// NewPluginSandbox creates a new plugin sandbox
func NewPluginSandbox(pluginID string, config SandboxConfig, gateway *plugin_sdk.FeatureGateway) *PluginSandbox {
	ctx, cancel := context.WithCancel(context.Background())
	
	sandbox := &PluginSandbox{
		pluginID:    pluginID,
		config:      config,
		gateway:     gateway,
		startTime:   time.Now(),
		ctx:         ctx,
		cancel:      cancel,
		monitorDone: make(chan struct{}),
	}
	
	// Capture initial memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	sandbox.initialMemBytes = memStats.Alloc
	
	return sandbox
}

// Start begins resource monitoring
func (ps *PluginSandbox) Start() {
	if !ps.config.EnableMonitoring {
		return
	}
	
	// Start monitoring goroutine
	ps.monitorTicker = time.NewTicker(5 * time.Second)
	go ps.monitorResources()
	
	log.Info().
		Str("plugin_id", ps.pluginID).
		Int64("max_memory_mb", ps.config.MaxMemoryMB).
		Int32("max_goroutines", ps.config.MaxGoroutines).
		Msg("Plugin sandbox monitoring started")
}

// Stop stops resource monitoring
func (ps *PluginSandbox) Stop() {
	if ps.monitorTicker != nil {
		ps.monitorTicker.Stop()
	}
	
	ps.cancel()
	
	if ps.config.EnableMonitoring {
		close(ps.monitorDone)
	}
	
	log.Info().Str("plugin_id", ps.pluginID).Msg("Plugin sandbox stopped")
}

// monitorResources continuously monitors plugin resource usage
func (ps *PluginSandbox) monitorResources() {
	for {
		select {
		case <-ps.ctx.Done():
			return
		case <-ps.monitorDone:
			return
		case <-ps.monitorTicker.C:
			ps.checkResourceLimits()
		}
	}
}

// checkResourceLimits checks if plugin has exceeded any resource limits
func (ps *PluginSandbox) checkResourceLimits() {
	// Check goroutine count
	goroutineCount := ps.gateway.GetGoroutineCount()
	if ps.config.MaxGoroutines > 0 && goroutineCount > ps.config.MaxGoroutines {
		ps.violate(fmt.Sprintf("goroutine limit exceeded: %d > %d", goroutineCount, ps.config.MaxGoroutines))
		return
	}
	
	// Check memory usage (approximate)
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	currentMemBytes := memStats.Alloc
	
	// Estimate plugin memory usage (this is approximate since Go doesn't provide per-plugin memory tracking)
	// We track the delta from when the plugin started
	estimatedMemBytes := int64(0)
	if currentMemBytes > ps.initialMemBytes {
		estimatedMemBytes = int64(currentMemBytes - ps.initialMemBytes)
	}
	estimatedMemMB := estimatedMemBytes / (1024 * 1024)
	
	if ps.config.MaxMemoryMB > 0 && estimatedMemMB > ps.config.MaxMemoryMB {
		ps.violate(fmt.Sprintf("memory limit exceeded: ~%d MB > %d MB", estimatedMemMB, ps.config.MaxMemoryMB))
		return
	}
	
	// Check CPU time (if configured)
	if ps.config.CPUTimeLimit > 0 {
		elapsed := time.Since(ps.startTime)
		if elapsed > ps.config.CPUTimeLimit {
			ps.violate(fmt.Sprintf("CPU time limit exceeded: %s > %s", elapsed, ps.config.CPUTimeLimit))
			return
		}
	}
	
	// Log resource usage periodically
	log.Debug().
		Str("plugin_id", ps.pluginID).
		Int32("goroutines", goroutineCount).
		Int64("estimated_memory_mb", estimatedMemMB).
		Msg("Plugin resource usage")
}

// violate marks the sandbox as violated and triggers shutdown
func (ps *PluginSandbox) violate(reason string) {
	if ps.violated.Load() {
		return // Already violated
	}
	
	ps.violated.Store(true)
	ps.mu.Lock()
	ps.violationReason = reason
	ps.mu.Unlock()
	
	log.Error().
		Str("plugin_id", ps.pluginID).
		Str("reason", reason).
		Msg("Plugin sandbox limit violated - initiating shutdown")
	
	// Cancel context to signal plugin to stop
	ps.cancel()
}

// IsViolated returns whether the sandbox limits have been violated
func (ps *PluginSandbox) IsViolated() bool {
	return ps.violated.Load()
}

// GetViolationReason returns the reason for sandbox violation
func (ps *PluginSandbox) GetViolationReason() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.violationReason
}

// GetResourceUsage returns current resource usage statistics
func (ps *PluginSandbox) GetResourceUsage() ResourceUsage {
	goroutineCount := ps.gateway.GetGoroutineCount()
	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	currentMemBytes := memStats.Alloc
	
	estimatedMemBytes := int64(0)
	if currentMemBytes > ps.initialMemBytes {
		estimatedMemBytes = int64(currentMemBytes - ps.initialMemBytes)
	}
	
	return ResourceUsage{
		GoroutineCount:     goroutineCount,
		EstimatedMemoryMB:  estimatedMemBytes / (1024 * 1024),
		Uptime:             time.Since(ps.startTime),
		IsViolated:         ps.IsViolated(),
		ViolationReason:    ps.GetViolationReason(),
	}
}

// ResourceUsage represents current resource usage of a plugin
type ResourceUsage struct {
	GoroutineCount    int32         `json:"goroutine_count"`
	EstimatedMemoryMB int64         `json:"estimated_memory_mb"`
	Uptime            time.Duration `json:"uptime"`
	IsViolated        bool          `json:"is_violated"`
	ViolationReason   string        `json:"violation_reason,omitempty"`
}

// EnforceLimits performs an immediate check of resource limits
func (ps *PluginSandbox) EnforceLimits() error {
	ps.checkResourceLimits()
	
	if ps.IsViolated() {
		return fmt.Errorf("sandbox limits violated: %s", ps.GetViolationReason())
	}
	
	return nil
}

// SetMaxMemoryMB updates the maximum memory limit
func (ps *PluginSandbox) SetMaxMemoryMB(maxMemoryMB int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.config.MaxMemoryMB = maxMemoryMB
	
	log.Info().
		Str("plugin_id", ps.pluginID).
		Int64("max_memory_mb", maxMemoryMB).
		Msg("Updated plugin memory limit")
}

// SetMaxGoroutines updates the maximum goroutine limit
func (ps *PluginSandbox) SetMaxGoroutines(maxGoroutines int32) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.config.MaxGoroutines = maxGoroutines
	
	log.Info().
		Str("plugin_id", ps.pluginID).
		Int32("max_goroutines", maxGoroutines).
		Msg("Updated plugin goroutine limit")
}

// GetContext returns the sandbox context (cancelled when limits violated)
func (ps *PluginSandbox) GetContext() context.Context {
	return ps.ctx
}

// GetConfig returns the current sandbox configuration
func (ps *PluginSandbox) GetConfig() SandboxConfig {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.config
}

