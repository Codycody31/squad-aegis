package analytics

import (
	"sync"
)

var (
	metricsCollector *MetricsCollector
	metricsOnce      sync.Once
)

// MetricsCollector collects and tracks various metrics
type MetricsCollector struct {
	Countly *Countly
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(countly *Countly) *MetricsCollector {
	metricsOnce.Do(func() {
		metricsCollector = &MetricsCollector{
			Countly: countly,
		}
	})
	return metricsCollector
}

func (m *MetricsCollector) GetCountly() *Countly {
	return m.Countly
}
