package logwatcher_manager

import (
	"testing"
	"time"
)

func TestLogParsingMetrics(t *testing.T) {
	metrics := NewLogParsingMetrics()

	// Test initial state
	initial := metrics.GetMetrics()
	if initial["linesPerMinute"].(float64) != 0 {
		t.Errorf("Expected 0 lines per minute initially, got %v", initial["linesPerMinute"])
	}
	if initial["matchingLinesPerMinute"].(float64) != 0 {
		t.Errorf("Expected 0 matching lines per minute initially, got %v", initial["matchingLinesPerMinute"])
	}

	// Record some lines
	metrics.RecordLineProcessed()
	metrics.RecordLineProcessed()
	metrics.RecordMatchingLine(10 * time.Millisecond)

	// Check metrics
	current := metrics.GetMetrics()
	if current["linesPerMinute"].(float64) != 2 {
		t.Errorf("Expected 2 lines per minute, got %v", current["linesPerMinute"])
	}
	if current["matchingLinesPerMinute"].(float64) != 1 {
		t.Errorf("Expected 1 matching line per minute, got %v", current["matchingLinesPerMinute"])
	}
	if current["totalLines"].(int64) != 2 {
		t.Errorf("Expected 2 total lines, got %v", current["totalLines"])
	}
	if current["totalMatchingLines"].(int64) != 1 {
		t.Errorf("Expected 1 total matching line, got %v", current["totalMatchingLines"])
	}

	// Check latency calculation
	latency := current["matchingLatency"].(float64)
	if latency < 9 || latency > 11 { // Should be around 10ms
		t.Errorf("Expected latency around 10ms, got %v", latency)
	}
}

func TestMetricsCleanup(t *testing.T) {
	metrics := NewLogParsingMetrics()

	// Add some old entries by manipulating time
	now := time.Now()
	oldTime := now.Add(-2 * time.Minute)

	// Manually add old entries
	metrics.mu.Lock()
	metrics.lastMinuteLines = []time.Time{oldTime, now}
	metrics.lastMinuteMatchingLines = []time.Time{oldTime, now}
	metrics.mu.Unlock()

	// Get metrics should clean up old entries
	current := metrics.GetMetrics()

	if current["linesPerMinute"].(float64) != 1 {
		t.Errorf("Expected 1 line per minute after cleanup, got %v", current["linesPerMinute"])
	}
	if current["matchingLinesPerMinute"].(float64) != 1 {
		t.Errorf("Expected 1 matching line per minute after cleanup, got %v", current["matchingLinesPerMinute"])
	}
}
