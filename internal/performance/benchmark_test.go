package performance

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkMonitor_RecordMetrics benchmarks metric recording performance
func BenchmarkMonitor_RecordMetrics(b *testing.B) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           100000,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			monitor.RecordCounter(fmt.Sprintf("test.metric.%d", i%100), float64(i), map[string]string{
				"tag1": "value1",
				"tag2": "value2",
			})
			i++
		}
	})
}

// BenchmarkMonitor_Timer benchmarks timer performance
func BenchmarkMonitor_Timer(b *testing.B) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           100000,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			timer := monitor.StartTimer("benchmark.operation", map[string]string{"type": "test"})
			// Simulate some work
			time.Sleep(1 * time.Microsecond)
			timer.Stop()
		}
	})
}

// BenchmarkConcurrentManager_Execute benchmarks concurrent execution
func BenchmarkConcurrentManager_Execute(b *testing.B) {
	config := &ConcurrentConfig{
		MaxConcurrent: 10,
		EnableMetrics: false, // Disable metrics for pure concurrency benchmark
	}

	manager := NewConcurrentManager(config, nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := manager.Execute(context.Background(), "benchmark_operation", nil, func(ctx context.Context) error {
				// Simulate some work
				time.Sleep(1 * time.Microsecond)
				return nil
			})
			if err != nil {
				b.Fatalf("Unexpected error: %v", err)
			}
		}
	})
}

// BenchmarkConcurrentManager_ExecuteWithMetrics benchmarks concurrent execution with metrics
func BenchmarkConcurrentManager_ExecuteWithMetrics(b *testing.B) {
	monitor := NewMonitor(&MonitorConfig{
		Enabled:              true,
		MaxMetrics:           100000,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	})
	defer monitor.Close()

	config := &ConcurrentConfig{
		MaxConcurrent: 10,
		EnableMetrics: true,
	}

	manager := NewConcurrentManager(config, monitor)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := manager.Execute(context.Background(), "benchmark_operation", map[string]string{
				"type": "benchmark",
			}, func(ctx context.Context) error {
				// Simulate some work
				time.Sleep(1 * time.Microsecond)
				return nil
			})
			if err != nil {
				b.Fatalf("Unexpected error: %v", err)
			}
		}
	})
}

// BenchmarkWorkerPool_TaskProcessing benchmarks worker pool task processing
func BenchmarkWorkerPool_TaskProcessing(b *testing.B) {
	config := &WorkerPoolConfig{
		Workers:    5,
		BufferSize: 1000,
	}

	pool := NewWorkerPool(config, nil)
	err := pool.Start()
	if err != nil {
		b.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	// Start a goroutine to consume results
	go func() {
		for range pool.Results() {
			// Consume results
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := Task{
			ID:   fmt.Sprintf("task_%d", i),
			Name: "benchmark_task",
			Data: i,
			Fn: func(data interface{}) (interface{}, error) {
				// Simulate some work
				if num, ok := data.(int); ok {
					return num * 2, nil
				}
				return nil, fmt.Errorf("invalid data")
			},
		}

		err := pool.Submit(task)
		if err != nil {
			b.Fatalf("Failed to submit task: %v", err)
		}
	}
}

// BenchmarkMemoryCollector_Collect benchmarks memory metric collection
func BenchmarkMemoryCollector_Collect(b *testing.B) {
	collector := NewMemoryCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics := collector.Collect()
		if len(metrics) == 0 {
			b.Fatal("Expected metrics to be collected")
		}
	}
}

// BenchmarkGoroutineCollector_Collect benchmarks goroutine metric collection
func BenchmarkGoroutineCollector_Collect(b *testing.B) {
	collector := NewGoroutineCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics := collector.Collect()
		if len(metrics) == 0 {
			b.Fatal("Expected metrics to be collected")
		}
	}
}

// BenchmarkMonitor_GetMetrics benchmarks metric retrieval
func BenchmarkMonitor_GetMetrics(b *testing.B) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           10000,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	// Pre-populate with metrics
	for i := 0; i < 1000; i++ {
		monitor.RecordCounter(fmt.Sprintf("test.metric.%d", i), float64(i), nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics := monitor.GetMetrics()
		if len(metrics) == 0 {
			b.Fatal("Expected metrics to be returned")
		}
	}
}

// BenchmarkMonitor_GetMetricsByName benchmarks filtered metric retrieval
func BenchmarkMonitor_GetMetricsByName(b *testing.B) {
	config := &MonitorConfig{
		Enabled:              true,
		MaxMetrics:           10000,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	}

	monitor := NewMonitor(config)
	defer monitor.Close()

	// Pre-populate with metrics
	for i := 0; i < 1000; i++ {
		monitor.RecordCounter("test.counter", float64(i), nil)
		monitor.RecordGauge("test.gauge", float64(i), "units", nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics := monitor.GetMetricsByName("test.counter")
		if len(metrics) == 0 {
			b.Fatal("Expected counter metrics to be returned")
		}
	}
}

// BenchmarkConcurrentManager_BatchExecution benchmarks batch execution
func BenchmarkConcurrentManager_BatchExecution(b *testing.B) {
	config := &ConcurrentConfig{
		MaxConcurrent: 10,
		EnableMetrics: false,
	}

	manager := NewConcurrentManager(config, nil)

	// Create batch operations
	operations := make([]BatchOperation, 10)
	for i := 0; i < 10; i++ {
		operations[i] = BatchOperation{
			Name: fmt.Sprintf("batch_op_%d", i),
			Fn: func(ctx context.Context) error {
				time.Sleep(1 * time.Microsecond)
				return nil
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := manager.ExecuteBatch(context.Background(), operations)
		if len(results) != 10 {
			b.Fatalf("Expected 10 results, got %d", len(results))
		}
	}
}
