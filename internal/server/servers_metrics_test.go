package server

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestServerMetricsPipelineHasFourClickHouseRoundTrips(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "servers_metrics.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	start := strings.Index(text, "func (s *Server) getMetricsFromClickHouse(")
	end := strings.Index(text, "\nfunc metricsTimeRange(")
	if start < 0 || end <= start {
		t.Fatal("locate getMetricsFromClickHouse source")
	}
	functionBody := text[start:end]
	roundTrip := regexp.MustCompile(`\.Clickhouse\.(?:Query|QueryRow)\(`)
	if got := len(roundTrip.FindAllString(functionBody, -1)); got != 4 {
		t.Fatalf("getMetricsFromClickHouse ClickHouse calls = %d, want 4", got)
	}
	if strings.Contains(functionBody, "generateSample") {
		t.Fatal("production metrics pipeline must not generate sample data")
	}
}
