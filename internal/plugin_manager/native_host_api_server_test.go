package plugin_manager

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
	pluginrpcpb "go.codycody31.dev/squad-aegis/pkg/pluginrpc/proto"
)

// recordingLogAPI is the minimal LogAPI surface for these tests.
type recordingLogAPI struct {
	calls int
}

func (r *recordingLogAPI) Info(string, map[string]interface{})         { r.calls++ }
func (r *recordingLogAPI) Warn(string, map[string]interface{})         { r.calls++ }
func (r *recordingLogAPI) Error(string, error, map[string]interface{}) { r.calls++ }
func (r *recordingLogAPI) Debug(string, map[string]interface{})        { r.calls++ }

type recordingDiscordAPI struct {
	sendEmbedCalls int
}

func (r *recordingDiscordAPI) SendMessage(string, string) (string, error) {
	return "message-id", nil
}

func (r *recordingDiscordAPI) SendEmbed(string, *DiscordEmbed) (string, error) {
	r.sendEmbedCalls++
	return "message-id", nil
}

func TestHostAPIDispatcherRateLimiterBlocksExcessCalls(t *testing.T) {
	logAPI := &recordingLogAPI{}
	disp := &hostAPIDispatcher{
		apis:    &PluginAPIs{LogAPI: logAPI},
		limiter: rate.NewLimiter(rate.Every(time.Hour), 2), // 2 burst, 1 token/hour refill
		sem:     make(chan struct{}, maxConcurrentHostAPICalls),
	}

	fields, _ := json.Marshal(map[string]interface{}{})
	req := &pluginrpcpb.LogRequest{Message: "hi", FieldsJson: fields}

	allowedCount := 0
	limitedCount := 0
	for i := 0; i < 5; i++ {
		_, err := disp.LogInfo(context.Background(), req)
		if err == nil {
			allowedCount++
		} else if strings.Contains(err.Error(), "rate limit exceeded") {
			limitedCount++
		} else {
			t.Fatalf("LogInfo unexpected error: %v", err)
		}
	}
	if allowedCount != 2 {
		t.Fatalf("allowedCount = %d, want 2 (burst size)", allowedCount)
	}
	if limitedCount != 3 {
		t.Fatalf("limitedCount = %d, want 3", limitedCount)
	}
	if logAPI.calls != 2 {
		t.Fatalf("underlying LogAPI.Info calls = %d, want 2 (rate-limited calls should not reach the API)", logAPI.calls)
	}
}

func TestHostAPIDispatcherRejectsMissingDiscordEmbed(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "missing", payload: nil},
		{name: "empty object", payload: []byte(`{}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discord := &recordingDiscordAPI{}
			disp := &hostAPIDispatcher{
				pluginID: "com.example.plugin",
				apis:     &PluginAPIs{DiscordAPI: discord},
				sem:      make(chan struct{}, maxConcurrentHostAPICalls),
			}
			req := &pluginrpcpb.DiscordMessageRequest{ChannelId: "123", EmbedJson: tt.payload}
			_, err := disp.DiscordSendEmbed(context.Background(), req)
			if err == nil || !strings.Contains(err.Error(), "embed is required") {
				t.Fatalf("DiscordSendEmbed() error = %v, want embed required error", err)
			}
			if discord.sendEmbedCalls != 0 {
				t.Fatalf("SendEmbed calls = %d, want 0", discord.sendEmbedCalls)
			}
		})
	}
}

func TestHostAPIDispatcherNoLimiterAllowsBurst(t *testing.T) {
	logAPI := &recordingLogAPI{}
	disp := &hostAPIDispatcher{
		apis: &PluginAPIs{LogAPI: logAPI},
		sem:  make(chan struct{}, maxConcurrentHostAPICalls),
	}

	fields, _ := json.Marshal(map[string]interface{}{})
	req := &pluginrpcpb.LogRequest{Message: "hi", FieldsJson: fields}

	for i := 0; i < 20; i++ {
		if _, err := disp.LogInfo(context.Background(), req); err != nil {
			t.Fatalf("LogInfo() error = %v", err)
		}
	}
	if logAPI.calls != 20 {
		t.Fatalf("underlying LogAPI.Info calls = %d, want 20", logAPI.calls)
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
				if limiter.AllowN(time.Now(), 1) {
					allowed++
				}
			}
			if allowed != tc.wantAllowed {
				t.Fatalf("allowed = %d, want %d (burst size)", allowed, tc.wantAllowed)
			}
		})
	}
}
