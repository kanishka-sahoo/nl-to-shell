package performance

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentManager_Execute(t *testing.T) {
	config := &ConcurrentConfig{
		MaxConcurrent: 2,
		EnableMetrics: true,
	}

	monitor := NewMonitor(&MonitorConfig{
		Enabled:              true,
		MaxMetrics:           1000,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	})
	defer monitor.Close()

	manager := NewConcurrentManager(config, monitor)

	// Test successful execution
	executed := false
	err := manager.Execute(context.Background(), "test_operation", map[string]string{"tag": "value"}, func(ctx context.Context) error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !executed {
		t.Fatal("Function should have been executed")
	}

	// Check that metrics were recorded
	metrics := monitor.GetMetrics()
	if len(metrics) == 0 {
		t.Fatal("Expected metrics to be recorded")
	}

	// Look for operation metrics
	foundStart := false
	foundComplete := false
	foundDuration := false

	for _, metric := range metrics {
		switch metric.Name {
		case "concurrent.operation.started":
			foundStart = true
		case "concurrent.operation.completed":
			foundComplete = true
		case "concurrent.operation.duration":
			foundDuration = true
		}
	}

	if !foundStart {
		t.Fatal("Expected to find operation started metric")
	}
	if !foundComplete {
		t.Fatal("Expected to find operation completed metric")
	}
	if !foundDuration {
		t.Fatal("Expected to find operation duration metric")
	}
}

func TestConcurrentManager_ExecuteWithError(t *testing.T) {
	config := &ConcurrentConfig{
		MaxConcurrent: 2,
		EnableMetrics: true,
	}

	monitor := NewMonitor(&MonitorConfig{
		Enabled:              true,
		MaxMetrics:           1000,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	})
	defer monitor.Close()

	manager := NewConcurrentManager(config, monitor)

	// Test execution with error
	testError := errors.New("test error")
	err := manager.Execute(context.Background(), "test_operation", nil, func(ctx context.Context) error {
		return testError
	})

	if err != testError {
		t.Fatalf("Expected test error, got %v", err)
	}

	// Check that error status was recorded
	metrics := monitor.GetMetrics()
	foundErrorStatus := false

	for _, metric := range metrics {
		if metric.Name == "concurrent.operation.completed" {
			if status, ok := metric.Tags["status"]; ok && status == "error" {
				foundErrorStatus = true
				break
			}
		}
	}

	if !foundErrorStatus {
		t.Fatal("Expected to find operation completed metric with error status")
	}
}

func TestConcurrentManager_ConcurrencyLimit(t *testing.T) {
	config := &ConcurrentConfig{
		MaxConcurrent: 2,
		EnableMetrics: false,
	}

	manager := NewConcurrentManager(config, nil)

	// Start operations that will block
	started := make(chan struct{}, 2)
	proceed := make(chan struct{})

	// Start 2 operations (at the limit)
	for i := 0; i < 2; i++ {
		go func() {
			manager.Execute(context.Background(), "blocking_operation", nil, func(ctx context.Context) error {
				started <- struct{}{}
				<-proceed
				return nil
			})
		}()
	}

	// Wait for both operations to start
	<-started
	<-started

	// Verify we have 2 active operations
	activeCount := manager.GetActiveOperationsCount()
	if activeCount != 2 {
		t.Fatalf("Expected 2 active operations, got %d", activeCount)
	}

	// Try to start a third operation with a short timeout - should fail
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := manager.Execute(ctx, "third_operation", nil, func(ctx context.Context) error {
		return nil
	})

	if err != context.DeadlineExceeded {
		t.Fatalf("Expected context deadline exceeded, got %v", err)
	}

	// Release the blocking operations
	close(proceed)

	// Wait a bit for operations to complete
	time.Sleep(10 * time.Millisecond)

	// Should have no active operations now
	activeCount = manager.GetActiveOperationsCount()
	if activeCount != 0 {
		t.Fatalf("Expected 0 active operations, got %d", activeCount)
	}
}

func TestConcurrentManager_ExecuteWithTimeout(t *testing.T) {
	config := &ConcurrentConfig{
		MaxConcurrent: 5,
		EnableMetrics: false,
	}

	manager := NewConcurrentManager(config, nil)

	// Test successful execution within timeout
	err := manager.ExecuteWithTimeout("fast_operation", 100*time.Millisecond, nil, func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error for fast operation, got %v", err)
	}

	// Test timeout
	err = manager.ExecuteWithTimeout("slow_operation", 10*time.Millisecond, nil, func(ctx context.Context) error {
		select {
		case <-time.After(100 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	if err != context.DeadlineExceeded {
		t.Fatalf("Expected context deadline exceeded, got %v", err)
	}
}

func TestConcurrentManager_ExecuteBatch(t *testing.T) {
	config := &ConcurrentConfig{
		MaxConcurrent: 3,
		EnableMetrics: false,
	}

	manager := NewConcurrentManager(config, nil)

	// Create batch operations
	var counter int64
	operations := []BatchOperation{
		{
			Name: "op1",
			Fn: func(ctx context.Context) error {
				atomic.AddInt64(&counter, 1)
				return nil
			},
		},
		{
			Name: "op2",
			Fn: func(ctx context.Context) error {
				atomic.AddInt64(&counter, 2)
				return nil
			},
		},
		{
			Name: "op3",
			Fn: func(ctx context.Context) error {
				atomic.AddInt64(&counter, 3)
				return errors.New("test error")
			},
		},
	}

	// Execute batch
	results := manager.ExecuteBatch(context.Background(), operations)

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Check results
	successCount := 0
	errorCount := 0

	for _, result := range results {
		if result.Error == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	if successCount != 2 {
		t.Fatalf("Expected 2 successful operations, got %d", successCount)
	}

	if errorCount != 1 {
		t.Fatalf("Expected 1 failed operation, got %d", errorCount)
	}

	// Check that all operations executed
	finalCounter := atomic.LoadInt64(&counter)
	if finalCounter != 6 { // 1 + 2 + 3
		t.Fatalf("Expected counter to be 6, got %d", finalCounter)
	}
}

func TestConcurrentManager_GetStats(t *testing.T) {
	config := &ConcurrentConfig{
		MaxConcurrent: 3,
		EnableMetrics: false,
	}

	manager := NewConcurrentManager(config, nil)

	// Initially should have no active operations
	stats := manager.GetStats()
	if stats.MaxConcurrent != 3 {
		t.Fatalf("Expected max concurrent 3, got %d", stats.MaxConcurrent)
	}
	if stats.ActiveOperations != 0 {
		t.Fatalf("Expected 0 active operations, got %d", stats.ActiveOperations)
	}
	if stats.AvailableSlots != 3 {
		t.Fatalf("Expected 3 available slots, got %d", stats.AvailableSlots)
	}

	// Start a blocking operation
	started := make(chan struct{})
	proceed := make(chan struct{})

	go func() {
		manager.Execute(context.Background(), "blocking_op", nil, func(ctx context.Context) error {
			started <- struct{}{}
			<-proceed
			return nil
		})
	}()

	// Wait for operation to start
	<-started

	// Check stats with active operation
	stats = manager.GetStats()
	if stats.ActiveOperations != 1 {
		t.Fatalf("Expected 1 active operation, got %d", stats.ActiveOperations)
	}
	if stats.AvailableSlots != 2 {
		t.Fatalf("Expected 2 available slots, got %d", stats.AvailableSlots)
	}
	if stats.OperationCounts["blocking_op"] != 1 {
		t.Fatalf("Expected 1 blocking_op, got %d", stats.OperationCounts["blocking_op"])
	}

	// Release the operation
	close(proceed)
}

func TestWorkerPool_BasicOperations(t *testing.T) {
	config := &WorkerPoolConfig{
		Workers:    2,
		BufferSize: 10,
	}

	monitor := NewMonitor(&MonitorConfig{
		Enabled:              true,
		MaxMetrics:           1000,
		CollectionInterval:   0,
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	})
	defer monitor.Close()

	pool := NewWorkerPool(config, monitor)

	// Start the pool
	err := pool.Start()
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	// Submit a task
	task := Task{
		ID:   "test_task_1",
		Name: "test_task",
		Data: 42,
		Fn: func(data interface{}) (interface{}, error) {
			if num, ok := data.(int); ok {
				return num * 2, nil
			}
			return nil, errors.New("invalid data type")
		},
	}

	err = pool.Submit(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Get result
	select {
	case result := <-pool.Results():
		if result.TaskID != "test_task_1" {
			t.Fatalf("Expected task ID 'test_task_1', got %s", result.TaskID)
		}
		if result.Error != nil {
			t.Fatalf("Expected no error, got %v", result.Error)
		}
		if result.Result != 84 {
			t.Fatalf("Expected result 84, got %v", result.Result)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for task result")
	}

	// Check that metrics were recorded
	metrics := monitor.GetMetrics()
	foundSubmitted := false
	foundStarted := false
	foundCompleted := false

	for _, metric := range metrics {
		switch metric.Name {
		case "worker_pool.task.submitted":
			foundSubmitted = true
		case "worker_pool.task.started":
			foundStarted = true
		case "worker_pool.task.completed":
			foundCompleted = true
		}
	}

	if !foundSubmitted {
		t.Fatal("Expected to find task submitted metric")
	}
	if !foundStarted {
		t.Fatal("Expected to find task started metric")
	}
	if !foundCompleted {
		t.Fatal("Expected to find task completed metric")
	}
}

func TestWorkerPool_TaskError(t *testing.T) {
	config := &WorkerPoolConfig{
		Workers:    1,
		BufferSize: 5,
	}

	pool := NewWorkerPool(config, nil)

	err := pool.Start()
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	// Submit a task that will fail
	task := Task{
		ID:   "error_task",
		Name: "error_task",
		Data: "invalid",
		Fn: func(data interface{}) (interface{}, error) {
			return nil, errors.New("task failed")
		},
	}

	err = pool.Submit(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Get result
	select {
	case result := <-pool.Results():
		if result.TaskID != "error_task" {
			t.Fatalf("Expected task ID 'error_task', got %s", result.TaskID)
		}
		if result.Error == nil {
			t.Fatal("Expected error, got nil")
		}
		if result.Error.Error() != "task failed" {
			t.Fatalf("Expected error 'task failed', got %v", result.Error)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for task result")
	}
}

func TestWorkerPool_MultipleWorkers(t *testing.T) {
	config := &WorkerPoolConfig{
		Workers:    3,
		BufferSize: 10,
	}

	pool := NewWorkerPool(config, nil)

	err := pool.Start()
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	// Submit multiple tasks
	numTasks := 6
	for i := 0; i < numTasks; i++ {
		task := Task{
			ID:   fmt.Sprintf("task_%d", i),
			Name: "parallel_task",
			Data: i,
			Fn: func(data interface{}) (interface{}, error) {
				// Simulate some work
				time.Sleep(10 * time.Millisecond)
				return data, nil
			},
		}

		err = pool.Submit(task)
		if err != nil {
			t.Fatalf("Failed to submit task %d: %v", i, err)
		}
	}

	// Collect results
	results := make(map[string]interface{})
	for i := 0; i < numTasks; i++ {
		select {
		case result := <-pool.Results():
			if result.Error != nil {
				t.Fatalf("Task %s failed: %v", result.TaskID, result.Error)
			}
			results[result.TaskID] = result.Result
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for task results")
		}
	}

	// Verify all tasks completed
	if len(results) != numTasks {
		t.Fatalf("Expected %d results, got %d", numTasks, len(results))
	}

	// Verify results are correct
	for i := 0; i < numTasks; i++ {
		taskID := fmt.Sprintf("task_%d", i)
		if result, ok := results[taskID]; !ok {
			t.Fatalf("Missing result for task %s", taskID)
		} else if result != i {
			t.Fatalf("Expected result %d for task %s, got %v", i, taskID, result)
		}
	}
}
