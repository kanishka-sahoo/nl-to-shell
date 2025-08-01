package performance

import (
	"runtime"
	"sync"
	"time"
)

// Metric represents a performance metric
type Metric struct {
	Name      string                 `json:"name"`
	Value     float64                `json:"value"`
	Unit      string                 `json:"unit"`
	Timestamp time.Time              `json:"timestamp"`
	Tags      map[string]string      `json:"tags,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Timer represents a timing measurement
type Timer struct {
	name      string
	startTime time.Time
	tags      map[string]string
	monitor   *Monitor
}

// Stop stops the timer and records the duration
func (t *Timer) Stop() time.Duration {
	duration := time.Since(t.startTime)
	t.monitor.RecordDuration(t.name, duration, t.tags)
	return duration
}

// Monitor provides performance monitoring capabilities
type Monitor struct {
	metrics    []Metric
	mutex      sync.RWMutex
	enabled    bool
	maxMetrics int
	collectors []MetricCollector
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// MonitorConfig represents configuration for the performance monitor
type MonitorConfig struct {
	Enabled              bool          `json:"enabled"`
	MaxMetrics           int           `json:"max_metrics"`
	CollectionInterval   time.Duration `json:"collection_interval"`
	EnableMemoryStats    bool          `json:"enable_memory_stats"`
	EnableGoroutineStats bool          `json:"enable_goroutine_stats"`
}

// DefaultMonitorConfig returns a default monitor configuration
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           10000,
		CollectionInterval:   30 * time.Second,
		EnableMemoryStats:    true,
		EnableGoroutineStats: true,
	}
}

// NewMonitor creates a new performance monitor
func NewMonitor(config *MonitorConfig) *Monitor {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	monitor := &Monitor{
		metrics:    make([]Metric, 0, config.MaxMetrics),
		enabled:    config.Enabled,
		maxMetrics: config.MaxMetrics,
		collectors: make([]MetricCollector, 0),
		stopCh:     make(chan struct{}),
	}

	// Add default collectors
	if config.EnableMemoryStats {
		monitor.AddCollector(NewMemoryCollector())
	}
	if config.EnableGoroutineStats {
		monitor.AddCollector(NewGoroutineCollector())
	}

	// Start collection goroutine
	if config.Enabled && config.CollectionInterval > 0 {
		monitor.wg.Add(1)
		go monitor.collectionLoop(config.CollectionInterval)
	}

	return monitor
}

// IsEnabled returns whether monitoring is enabled
func (m *Monitor) IsEnabled() bool {
	return m.enabled
}

// StartTimer starts a new timer for measuring duration
func (m *Monitor) StartTimer(name string, tags map[string]string) *Timer {
	if !m.enabled {
		return &Timer{monitor: m} // Return a no-op timer
	}

	return &Timer{
		name:      name,
		startTime: time.Now(),
		tags:      tags,
		monitor:   m,
	}
}

// RecordDuration records a duration metric
func (m *Monitor) RecordDuration(name string, duration time.Duration, tags map[string]string) {
	if !m.enabled {
		return
	}

	m.RecordMetric(Metric{
		Name:      name,
		Value:     float64(duration.Nanoseconds()) / 1e6, // Convert to milliseconds
		Unit:      "ms",
		Timestamp: time.Now(),
		Tags:      tags,
	})
}

// RecordCounter records a counter metric
func (m *Monitor) RecordCounter(name string, value float64, tags map[string]string) {
	if !m.enabled {
		return
	}

	m.RecordMetric(Metric{
		Name:      name,
		Value:     value,
		Unit:      "count",
		Timestamp: time.Now(),
		Tags:      tags,
	})
}

// RecordGauge records a gauge metric
func (m *Monitor) RecordGauge(name string, value float64, unit string, tags map[string]string) {
	if !m.enabled {
		return
	}

	m.RecordMetric(Metric{
		Name:      name,
		Value:     value,
		Unit:      unit,
		Timestamp: time.Now(),
		Tags:      tags,
	})
}

// RecordMetric records a custom metric
func (m *Monitor) RecordMetric(metric Metric) {
	if !m.enabled {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Add timestamp if not set
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	// Add to metrics slice
	m.metrics = append(m.metrics, metric)

	// Trim if we exceed max metrics
	if len(m.metrics) > m.maxMetrics {
		// Remove oldest metrics (simple FIFO)
		copy(m.metrics, m.metrics[len(m.metrics)-m.maxMetrics:])
		m.metrics = m.metrics[:m.maxMetrics]
	}
}

// GetMetrics returns all recorded metrics
func (m *Monitor) GetMetrics() []Metric {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy to avoid race conditions
	metrics := make([]Metric, len(m.metrics))
	copy(metrics, m.metrics)
	return metrics
}

// GetMetricsByName returns metrics filtered by name
func (m *Monitor) GetMetricsByName(name string) []Metric {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var filtered []Metric
	for _, metric := range m.metrics {
		if metric.Name == name {
			filtered = append(filtered, metric)
		}
	}
	return filtered
}

// GetMetricsSince returns metrics recorded since the given time
func (m *Monitor) GetMetricsSince(since time.Time) []Metric {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var filtered []Metric
	for _, metric := range m.metrics {
		if metric.Timestamp.After(since) {
			filtered = append(filtered, metric)
		}
	}
	return filtered
}

// GetStats returns summary statistics
func (m *Monitor) GetStats() MonitorStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := MonitorStats{
		TotalMetrics: len(m.metrics),
		Enabled:      m.enabled,
	}

	if len(m.metrics) > 0 {
		stats.OldestMetric = m.metrics[0].Timestamp
		stats.NewestMetric = m.metrics[len(m.metrics)-1].Timestamp
	}

	// Count metrics by name
	stats.MetricCounts = make(map[string]int)
	for _, metric := range m.metrics {
		stats.MetricCounts[metric.Name]++
	}

	return stats
}

// AddCollector adds a metric collector
func (m *Monitor) AddCollector(collector MetricCollector) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.collectors = append(m.collectors, collector)
}

// ClearMetrics clears all recorded metrics
func (m *Monitor) ClearMetrics() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.metrics = m.metrics[:0]
}

// Close stops the monitor and cleanup resources
func (m *Monitor) Close() {
	close(m.stopCh)
	m.wg.Wait()
}

// collectionLoop runs periodic metric collection
func (m *Monitor) collectionLoop(interval time.Duration) {
	defer m.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.collectMetrics()
		case <-m.stopCh:
			return
		}
	}
}

// collectMetrics collects metrics from all registered collectors
func (m *Monitor) collectMetrics() {
	m.mutex.RLock()
	collectors := make([]MetricCollector, len(m.collectors))
	copy(collectors, m.collectors)
	m.mutex.RUnlock()

	for _, collector := range collectors {
		metrics := collector.Collect()
		for _, metric := range metrics {
			m.RecordMetric(metric)
		}
	}
}

// MonitorStats represents monitoring statistics
type MonitorStats struct {
	TotalMetrics int            `json:"total_metrics"`
	Enabled      bool           `json:"enabled"`
	OldestMetric time.Time      `json:"oldest_metric,omitempty"`
	NewestMetric time.Time      `json:"newest_metric,omitempty"`
	MetricCounts map[string]int `json:"metric_counts"`
}

// MetricCollector defines the interface for metric collectors
type MetricCollector interface {
	Collect() []Metric
	Name() string
}

// MemoryCollector collects memory usage metrics
type MemoryCollector struct{}

// NewMemoryCollector creates a new memory collector
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{}
}

// Name returns the collector name
func (mc *MemoryCollector) Name() string {
	return "memory"
}

// Collect collects memory metrics
func (mc *MemoryCollector) Collect() []Metric {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	now := time.Now()
	return []Metric{
		{
			Name:      "memory.heap_alloc",
			Value:     float64(memStats.HeapAlloc),
			Unit:      "bytes",
			Timestamp: now,
		},
		{
			Name:      "memory.heap_sys",
			Value:     float64(memStats.HeapSys),
			Unit:      "bytes",
			Timestamp: now,
		},
		{
			Name:      "memory.heap_idle",
			Value:     float64(memStats.HeapIdle),
			Unit:      "bytes",
			Timestamp: now,
		},
		{
			Name:      "memory.heap_inuse",
			Value:     float64(memStats.HeapInuse),
			Unit:      "bytes",
			Timestamp: now,
		},
		{
			Name:      "memory.stack_inuse",
			Value:     float64(memStats.StackInuse),
			Unit:      "bytes",
			Timestamp: now,
		},
		{
			Name:      "memory.sys",
			Value:     float64(memStats.Sys),
			Unit:      "bytes",
			Timestamp: now,
		},
		{
			Name:      "memory.gc_count",
			Value:     float64(memStats.NumGC),
			Unit:      "count",
			Timestamp: now,
		},
		{
			Name:      "memory.gc_pause_total",
			Value:     float64(memStats.PauseTotalNs) / 1e6, // Convert to milliseconds
			Unit:      "ms",
			Timestamp: now,
		},
	}
}

// GoroutineCollector collects goroutine metrics
type GoroutineCollector struct{}

// NewGoroutineCollector creates a new goroutine collector
func NewGoroutineCollector() *GoroutineCollector {
	return &GoroutineCollector{}
}

// Name returns the collector name
func (gc *GoroutineCollector) Name() string {
	return "goroutines"
}

// Collect collects goroutine metrics
func (gc *GoroutineCollector) Collect() []Metric {
	return []Metric{
		{
			Name:      "goroutines.count",
			Value:     float64(runtime.NumGoroutine()),
			Unit:      "count",
			Timestamp: time.Now(),
		},
	}
}
