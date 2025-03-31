package analytics

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/shared/config"
	"go.codycody31.dev/squad-aegis/version"
)

var (
	instance *Countly
	once     sync.Once
)

// Countly represents the Countly analytics client
type Countly struct {
	appKey        string
	host          string
	deviceID      string
	httpClient    *http.Client
	mu            sync.RWMutex
	events        []Event
	batchSize     int
	flushInterval time.Duration
	stopChan      chan struct{}
	anonymous     bool
}

type Consent struct {
	Sessions bool `json:"sessions"`
	Events   bool `json:"events"`
	Location bool `json:"location"`
}

// Event represents a Countly event
type Event struct {
	Key          string                 `json:"key"`
	Count        int                    `json:"count"`
	Sum          float64                `json:"sum,omitempty"`
	Segmentation map[string]interface{} `json:"segmentation,omitempty"`
	Timestamp    int64                  `json:"timestamp"`
}

// NewCountly creates a new Countly analytics instance
func NewCountly(appKey, host string, anonymous bool) *Countly {
	once.Do(func() {
		instance = &Countly{
			appKey:        appKey,
			host:          host,
			deviceID:      generateDeviceID(),
			httpClient:    &http.Client{Timeout: 5 * time.Second},
			events:        make([]Event, 0),
			batchSize:     100,
			flushInterval: 30 * time.Second,
			stopChan:      make(chan struct{}),
			anonymous:     anonymous,
		}
		go instance.periodicFlush()
	})
	return instance
}

// TrackEvent tracks an event with optional segmentation data
func (c *Countly) TrackEvent(key string, count int, sum float64, segmentation map[string]interface{}) {
	if c == nil {
		return
	}

	event := Event{
		Key:          key,
		Count:        count,
		Sum:          sum,
		Segmentation: segmentation,
		Timestamp:    time.Now().Unix(),
	}

	c.mu.Lock()
	c.events = append(c.events, event)
	eventCount := len(c.events)
	c.mu.Unlock()

	if eventCount >= c.batchSize {
		c.flushEvents()
	}
}

func (c *Countly) BeginSession() {
	deviceInfo := GetDeviceInfo(c.anonymous)

	payload := map[string]interface{}{
		"begin_session": 1,
		"metrics": map[string]interface{}{
			"_os":          deviceInfo.OS,
			"os_arch":      deviceInfo.OSArch,
			"_os_version":  deviceInfo.OSVersion,
			"_device":      deviceInfo.DeviceName,
			"_app_version": version.String(),
			"cpu_count":    deviceInfo.CPUCount,
			"memory_total": deviceInfo.MemoryTotal,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling telemetry events")
		return
	}

	url := fmt.Sprintf("%s/i?app_key=%s&device_id=%s&sdk_name=golang", c.host, c.appKey, c.deviceID)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().Err(err).Msg("Error sending telemetry events to Countly")
		return
	}

	defer resp.Body.Close()
}

func (c *Countly) UpdateSession() {
	payload := map[string]interface{}{
		"session_duration": 120,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling telemetry events")
		return
	}

	url := fmt.Sprintf("%s/i?app_key=%s&device_id=%s&sdk_name=golang", c.host, c.appKey, c.deviceID)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().Err(err).Msg("Error sending telemetry events to Countly")
		return
	}

	defer resp.Body.Close()
}

func (c *Countly) EndSession() {
	payload := map[string]interface{}{
		"end_session": 1,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling telemetry events")
		return
	}

	url := fmt.Sprintf("%s/i?app_key=%s&device_id=%s&sdk_name=golang", c.host, c.appKey, c.deviceID)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().Err(err).Msg("Error sending telemetry events to Countly")
		return
	}

	defer resp.Body.Close()
}

func (c *Countly) Consent(consent Consent) {
	payload := map[string]interface{}{
		"consent": consent,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling telemetry events")
		return
	}

	url := fmt.Sprintf("%s/i?app_key=%s&device_id=%s&sdk_name=golang", c.host, c.appKey, c.deviceID)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().Err(err).Msg("Error sending telemetry events to Countly")
		return
	}

	defer resp.Body.Close()
}

func (c *Countly) TrackCrash(crashData map[string]interface{}) {
	payload := map[string]interface{}{
		"crash": crashData,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling telemetry events")
		return
	}

	url := fmt.Sprintf("%s/i?app_key=%s&device_id=%s&sdk_name=golang", c.host, c.appKey, c.deviceID)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().Err(err).Msg("Error sending telemetry events to Countly")
		return
	}

	defer resp.Body.Close()
}

// flushEvents sends the batched events to Countly
func (c *Countly) flushEvents() {
	c.mu.Lock()
	if len(c.events) == 0 {
		c.mu.Unlock()
		return
	}

	events := make([]Event, len(c.events))
	copy(events, c.events)
	c.events = c.events[:0]
	c.mu.Unlock()

	payload := map[string]interface{}{
		"events": events,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling telemetry events")
		return
	}

	url := fmt.Sprintf("%s/i?app_key=%s&device_id=%s&sdk_name=golang", c.host, c.appKey, c.deviceID)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().Err(err).Msg("Error sending telemetry events to Countly")
		return
	}

	defer resp.Body.Close()
}

// generateDeviceID generates a unique device ID and stores it in a config file
func generateDeviceID() string {
	var configPath string

	if config.Config.App.InContainer {
		configPath = "/etc/squad-aegis"
		if err := os.MkdirAll(configPath, 0755); err != nil {
			log.Error().Err(err).Msg("Failed to create config directory")
			return generateFallbackID()
		}
	} else {
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				log.Error().Err(err).Msg("Failed to get user home directory")
				return generateFallbackID()
			}
			configDir = homeDir + "/.config"
		}
		configPath = configDir + "/squad-aegis"
	}

	if err := os.MkdirAll(configPath, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create config directory")
		return generateFallbackID()
	}

	deviceIDFile := configPath + "/device_id"

	// Try to read existing device ID
	if data, err := os.ReadFile(deviceIDFile); err == nil {
		return string(bytes.TrimSpace(data))
	}

	// Generate new uuid
	deviceID := uuid.New().String()

	// Store the device ID
	if err := os.WriteFile(deviceIDFile, []byte(deviceID), 0644); err != nil {
		log.Error().Err(err).Msg("Failed to store device ID")
		return generateFallbackID()
	}

	log.Info().Msgf("Generated new telemetry device ID: %s", deviceID)
	return deviceID
}

// generateFallbackID generates a fallback device ID when we can't access the config directory
func generateFallbackID() string {
	hostname, _ := os.Hostname()
	hash := sha256.New()
	hash.Write([]byte(hostname))
	hash.Write([]byte(time.Now().String()))
	return fmt.Sprintf("fallback-%x", hash.Sum(nil))
}

// periodicFlush periodically flushes events if they've been sitting too long
func (c *Countly) periodicFlush() {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if len(c.events) > 0 {
				// Check if any events are older than the flush interval
				oldestEvent := c.events[0]
				if time.Since(time.Unix(oldestEvent.Timestamp, 0)) >= c.flushInterval {
					c.mu.Unlock()
					c.flushEvents()
				} else {
					c.mu.Unlock()
				}
			} else {
				c.mu.Unlock()
			}
		case <-c.stopChan:
			return
		}
	}
}

// Close stops the periodic flush goroutine and flushes any remaining events
func (c *Countly) Close() {
	if c == nil {
		return
	}
	close(c.stopChan)
	c.flushEvents()
}
