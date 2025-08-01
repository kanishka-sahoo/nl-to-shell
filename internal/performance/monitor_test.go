package performance

import (
	"testing"
	"time"
)

func TestMonitor_BasicOperations(t *testing.T) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           100,
		CollectionInterval:   0, // Disable automatic collection for tests
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	if !monitor.IsEnabled() {
		t.Fatal("Monitor should be enabled")
	}

	// Test recording a counter
	monitor.RecordCounter("test.counter", 5, map[string]string{"tag": "value"})

	// Test recording a gauge
	monitor.RecordGauge("test.gauge", 10.5, "units", map[string]string{"type": "test"})

	// Test recording a duration
	monitor.RecordDuration("test.duration", 100*time.Millisecond, nil)

	// Get all metrics
	metrics := monitor.GetMetrics()
	if len(metrics) != 3 {
		t.Fatalf("Expected 3 metrics, got %d", len(metrics))
	}

	// Test getting metrics by name
	counterMetrics := monitor.GetMetricsByName("test.counter")
	if len(counterMetrics) != 1 {
		t.Fatalf("Expected 1 counter metric, got %d", len(counterMetrics))
	}

	if counterMetrics[0].Value != 5 {
		t.Fatalf("Expected counter value 5, got %f", counterMetrics[0].Value)
	}

	if counterMetrics[0].Unit != "count" {
		t.Fatalf("Expected counter unit 'count', got %s", counterMetrics[0].Unit)
	}
}

func TestMonitor_Timer(t *testing.T) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           100,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	// Start a timer
	timer := monitor.StartTimer("test.operation", map[string]string{"operation": "test"})

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	// Stop the timer
	duration := timer.Stop()

	if duration < 10*time.Millisecond {
		t.Fatalf("Expected duration >= 10ms, got %v", duration)
	}

	// Check that metric was recorded
	metrics := monitor.GetMetricsByName("test.operation")
	if len(metrics) != 1 {
		t.Fatalf("Expected 1 timer metric, got %d", len(metrics))
	}

	if metrics[0].Unit != "ms" {
		t.Fatalf("Expected timer unit 'ms', got %s", metrics[0].Unit)
	}

	if metrics[0].Value < 10 {
		t.Fatalf("Expected timer value >= 10ms, got %f", metrics[0].Value)
	}
}

func TestMonitor_MaxMetrics(t *testing.T) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           5, // Small limit for testing
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	// Add more metrics than the limit
	for i := 0; i < 10; i++ {
		monitor.RecordCounter("test.counter", float64(i), nil)
	}

	metrics := monitor.GetMetrics()
	if len(metrics) != 5 {
		t.Fatalf("Expected 5 metrics (max limit), got %d", len(metrics))
	}

	// Should have the most recent metrics (5-9)
	for i, metric := range metrics {
		expectedValue := float64(5 + i)
		if metric.Value != expectedValue {
			t.Fatalf("Expected metric value %f, got %f", expectedValue, metric.Value)
		}
	}
}

func TestMonitor_GetMetricsSince(t *testing.T) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           100,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	// Record some metrics
	monitor.RecordCounter("test.before", 1, nil)

	// Mark time
	since := time.Now()
	time.Sleep(1 * time.Millisecond) // Ensure timestamp difference

	// Record more metrics
	monitor.RecordCounter("test.after", 2, nil)

	// Get metrics since the marked time
	recentMetrics := monitor.GetMetricsSince(since)
	if len(recentMetrics) != 1 {
		t.Fatalf("Expected 1 recent metric, got %d", len(recentMetrics))
	}

	if recentMetrics[0].Name != "test.after" {
		t.Fatalf("Expected metric name 'test.after', got %s", recentMetrics[0].Name)
	}
}

func TestMonitor_Stats(t *testing.T) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           100,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	// Record some metrics
	monitor.RecordCounter("test.counter", 1, nil)
	monitor.RecordCounter("test.counter", 2, nil)
	monitor.RecordGauge("test.gauge", 5, "units", nil)

	stats := monitor.GetStats()
	if stats.TotalMetrics != 3 {
		t.Fatalf("Expected 3 total metrics, got %d", stats.TotalMetrics)
	}

	if !stats.Enabled {
		t.Fatal("Stats should show monitor as enabled")
	}

	if stats.MetricCounts["test.counter"] != 2 {
		t.Fatalf("Expected 2 counter metrics, got %d", stats.MetricCounts["test.counter"])
	}

	if stats.MetricCounts["test.gauge"] != 1 {
		t.Fatalf("Expected 1 gauge metric, got %d", stats.MetricCounts["test.gauge"])
	}
}

func TestMonitor_ClearMetrics(t *testing.T) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           100,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	// Record some metrics
	monitor.RecordCounter("test.counter", 1, nil)
	monitor.RecordGauge("test.gauge", 5, "units", nil)

	// Verify metrics exist
	if len(monitor.GetMetrics()) != 2 {
		t.Fatal("Expected 2 metrics before clear")
	}

	// Clear metrics
	monitor.ClearMetrics()

	// Verify metrics are cleared
	if len(monitor.GetMetrics()) != 0 {
		t.Fatal("Expected 0 metrics after clear")
	}
}

func TestMemoryCollector(t *testing.T) {
	collector := NewMemoryCollector()

	if collector.Name() != "memory" {
		t.Fatalf("Expected collector name 'memory', got %s", collector.Name())
	}

	metrics := collector.Collect()
	if len(metrics) == 0 {
		t.Fatal("Expected memory metrics to be collected")
	}

	// Check for expected memory metrics
	expectedMetrics := []string{
		"memory.heap_alloc",
		"memory.heap_sys",
		"memory.heap_idle",
		"memory.heap_inuse",
		"memory.stack_inuse",
		"memory.sys",
		"memory.gc_count",
		"memory.gc_pause_total",
	}

	metricNames := make(map[string]bool)
	for _, metric := range metrics {
		metricNames[metric.Name] = true
	}

	for _, expectedName := range expectedMetrics {
		if !metricNames[expectedName] {
			t.Fatalf("Expected memory metric %s not found", expectedName)
		}
	}
}

func TestGoroutineCollector(t *testing.T) {
	collector := NewGoroutineCollector()

	if collector.Name() != "goroutines" {
		t.Fatalf("Expected collector name 'goroutines', got %s", collector.Name())
	}

	metrics := collector.Collect()
	if len(metrics) != 1 {
		t.Fatalf("Expected 1 goroutine metric, got %d", len(metrics))
	}

	metric := metrics[0]
	if metric.Name != "goroutines.count" {
		t.Fatalf("Expected metric name 'goroutines.count', got %s", metric.Name)
	}

	if metric.Unit != "count" {
		t.Fatalf("Expected metric unit 'count', got %s", metric.Unit)
	}

	if metric.Value <= 0 {
		t.Fatalf("Expected positive goroutine count, got %f", metric.Value)
	}
}

func TestMonitor_DisabledMonitor(t *testing.T) {
	config := &MonitorConfig{
		Enabled:              false,
		MaxMetrics:           100,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	if monitor.IsEnabled() {
		t.Fatal("Monitor should be disabled")
	}

	// Try to record metrics - should be no-ops
	monitor.RecordCounter("test.counter", 1, nil)
	monitor.RecordGauge("test.gauge", 5, "units", nil)

	// Start a timer - should be a no-op
	timer := monitor.StartTimer("test.timer", nil)
	timer.Stop()

	// Should have no metrics
	metrics := monitor.GetMetrics()
	if len(metrics) != 0 {
		t.Fatalf("Expected 0 metrics for disabled monitor, got %d", len(metrics))
	}
}
