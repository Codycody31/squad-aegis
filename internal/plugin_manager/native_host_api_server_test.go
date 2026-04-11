package plugin_manager

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
	"go.codycody31.dev/squad-aegis/pkg/pluginrpc"
)

// recordingLogAPI is the minimal LogAPI surface for these tests.
type recordingLogAPI struct {
	calls int
}

func (r *recordingLogAPI) Info(string, map[string]interface{})         { r.calls++ }
func (r *recordingLogAPI) Warn(string, map[string]interface{})         { r.calls++ }
func (r *recordingLogAPI) Error(string, error, map[string]interface{}) { r.calls++ }
func (r *recordingLogAPI) Debug(string, map[string]interface{})        { r.calls++ }

func TestHostAPIDispatcherRateLimiterBlocksExcessCalls(t *testing.T) {
	log := &recordingLogAPI{}
	disp := &hostAPIDispatcher{
		apis:    &PluginAPIs{LogAPI: log},
		limiter: rate.NewLimiter(rate.Every(time.Hour), 2), // 2 burst, 1 token/hour refill
	}

	payload, _ := json.Marshal(map[string]interface{}{"message": "hi"})
	req := pluginrpc.HostAPIRequest{Target: "log.Info", Payload: payload}

	allowedCount := 0
	limitedCount := 0
	for i := 0; i < 5; i++ {
		var reply pluginrpc.HostAPIResponse
		if err := disp.Call(req, &reply); err != nil {
			t.Fatalf("Call() transport error = %v", err)
		}
		if strings.Contains(reply.Error, "rate limit exceeded") {
			limitedCount++
		} else {
			allowedCount++
		}
	}
	if allowedCount != 2 {
		t.Fatalf("allowedCount = %d, want 2 (burst size)", allowedCount)
	}
	if limitedCount != 3 {
		t.Fatalf("limitedCount = %d, want 3", limitedCount)
	}
	if log.calls != 2 {
		t.Fatalf("underlying LogAPI.Info calls = %d, want 2 (rate-limited calls should not reach the API)", log.calls)
	}
}

func TestHostAPIDispatcherNoLimiterAllowsBurst(t *testing.T) {
	log := &recordingLogAPI{}
	disp := &hostAPIDispatcher{apis: &PluginAPIs{LogAPI: log}}

	payload, _ := json.Marshal(map[string]interface{}{"message": "hi"})
	req := pluginrpc.HostAPIRequest{Target: "log.Info", Payload: payload}

	for i := 0; i < 20; i++ {
		var reply pluginrpc.HostAPIResponse
		if err := disp.Call(req, &reply); err != nil {
			t.Fatalf("Call() error = %v", err)
		}
		if reply.Error != "" {
			t.Fatalf("Call() reply.Error = %q, want empty (rate limiter disabled)", reply.Error)
		}
	}
	if log.calls != 20 {
		t.Fatalf("underlying LogAPI.Info calls = %d, want 20", log.calls)
	}
}

func TestBuildHostAPIRateLimiterRespectsConfig(t *testing.T) {
	prev := config.Config
	t.Cleanup(func() { config.Config = prev })

	cases := []struct {
		name        string
		rate        float64
		burst       int
		wantNil     bool
		wantAllowed int
	}{
		{name: "positive values", rate: 100, burst: 5, wantNil: false, wantAllowed: 5},
		{name: "zero rate disables", rate: 0, burst: 5, wantNil: true},
		{name: "negative burst disables", rate: 100, burst: -1, wantNil: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Struct{}
			cfg.Plugins.HostAPIRatePerSec = tc.rate
			cfg.Plugins.HostAPIBurst = tc.burst
			config.Config = &cfg

			limiter := buildHostAPIRateLimiter()
			if tc.wantNil {
				if limiter != nil {
					t.Fatalf("buildHostAPIRateLimiter() = %v, want nil", limiter)
				}
				return
			}
			if limiter == nil {
				t.Fatal("buildHostAPIRateLimiter() = nil, want limiter")
			}
			allowed := 0
			for i := 0; i < tc.wantAllowed+3; i++ {
				if limiter.AllowN(nowFromLimiter(limiter), 1) {
					allowed++
				}
			}
			if allowed != tc.wantAllowed {
				t.Fatalf("allowed = %d, want %d (burst size)", allowed, tc.wantAllowed)
			}
		})
	}
}

// nowFromLimiter returns the limiter's current time anchor. Needed because
// rate.Limiter.AllowN uses wall-clock and we want the calls to stack up
// inside a single test tick.
func nowFromLimiter(l *rate.Limiter) time.Time {
	return time.Now()
}
