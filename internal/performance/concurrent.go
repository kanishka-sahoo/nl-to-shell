package performance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConcurrentManager manages concurrent operations with performance monitoring
type ConcurrentManager struct {
	monitor       *Monitor
	maxConcurrent int
	semaphore     chan struct{}
	activeOps     sync.Map
	opCounter     int64
	mutex         sync.Mutex
}

// ConcurrentConfig represents configuration for concurrent operations
type ConcurrentConfig struct {
	MaxConcurrent int  `json:"max_concurrent"`
	EnableMetrics bool `json:"enable_metrics"`
}

// DefaultConcurrentConfig returns a default concurrent configuration
func DefaultConcurrentConfig() *ConcurrentConfig {
	return &ConcurrentConfig{
		MaxConcurrent: 10,
		EnableMetrics: true,
	}
}

// NewConcurrentManager creates a new concurrent manager
func NewConcurrentManager(config *ConcurrentConfig, monitor *Monitor) *ConcurrentManager {
	if config == nil {
		config = DefaultConcurrentConfig()
	}

	return &ConcurrentManager{
		monitor:       monitor,
		maxConcurrent: config.MaxConcurrent,
		semaphore:     make(chan struct{}, config.MaxConcurrent),
	}
}

// Operation represents a concurrent operation
type Operation struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	StartTime time.Time         `json:"start_time"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// Execute executes a function with concurrency control and monitoring
func (cm *ConcurrentManager) Execute(ctx context.Context, name string, tags map[string]string, fn func(context.Context) error) error {
	// Acquire semaphore slot
	select {
	case cm.semaphore <- struct{}{}:
		defer func() { <-cm.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// Create operation
	cm.mutex.Lock()
	cm.opCounter++
	opID := cm.opCounter
	cm.mutex.Unlock()

	op := &Operation{
		ID:        opID,
		Name:      name,
		StartTime: time.Now(),
		Tags:      tags,
	}

	// Track active operation
	cm.activeOps.Store(opID, op)
	defer cm.activeOps.Delete(opID)

	// Start timer
	var timer *Timer
	if cm.monitor != nil {
		timer = cm.monitor.StartTimer("concurrent.operation.duration", map[string]string{
			"operation": name,
		})
		if tags != nil {
			for k, v := range tags {
				timer.tags[k] = v
			}
		}
	}

	// Record operation start
	if cm.monitor != nil {
		cm.monitor.RecordCounter("concurrent.operation.started", 1, map[string]string{
			"operation": name,
		})
	}

	// Execute function
	err := fn(ctx)

	// Stop timer
	if timer != nil {
		timer.Stop()
	}

	// Record operation completion
	if cm.monitor != nil {
		status := "success"
		if err != nil {
			status = "error"
		}

		cm.monitor.RecordCounter("concurrent.operation.completed", 1, map[string]string{
			"operation": name,
			"status":    status,
		})

		// Record active operations count
		activeCount := cm.GetActiveOperationsCount()
		cm.monitor.RecordGauge("concurrent.operations.active", float64(activeCount), "count", nil)
	}

	return err
}

// ExecuteWithTimeout executes a function with a timeout
func (cm *ConcurrentManager) ExecuteWithTimeout(name string, timeout time.Duration, tags map[string]string, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return cm.Execute(ctx, name, tags, fn)
}

// ExecuteBatch executes multiple operations concurrently
func (cm *ConcurrentManager) ExecuteBatch(ctx context.Context, operations []BatchOperation) []BatchResult {
	results := make([]BatchResult, len(operations))
	var wg sync.WaitGroup

	for i, op := range operations {
		wg.Add(1)
		go func(index int, operation BatchOperation) {
			defer wg.Done()

			err := cm.Execute(ctx, operation.Name, operation.Tags, operation.Fn)
			results[index] = BatchResult{
				Index: index,
				Name:  operation.Name,
				Error: err,
			}
		}(i, op)
	}

	wg.Wait()
	return results
}

// GetActiveOperations returns all currently active operations
func (cm *ConcurrentManager) GetActiveOperations() []*Operation {
	var operations []*Operation
	cm.activeOps.Range(func(key, value interface{}) bool {
		if op, ok := value.(*Operation); ok {
			operations = append(operations, op)
		}
		return true
	})
	return operations
}

// GetActiveOperationsCount returns the number of active operations
func (cm *ConcurrentManager) GetActiveOperationsCount() int {
	count := 0
	cm.activeOps.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// GetStats returns concurrent manager statistics
func (cm *ConcurrentManager) GetStats() ConcurrentStats {
	activeOps := cm.GetActiveOperations()

	stats := ConcurrentStats{
		MaxConcurrent:    cm.maxConcurrent,
		ActiveOperations: len(activeOps),
		AvailableSlots:   cm.maxConcurrent - len(activeOps),
	}

	// Calculate operation statistics
	if len(activeOps) > 0 {
		operationCounts := make(map[string]int)
		var oldestStart time.Time
		first := true

		for _, op := range activeOps {
			operationCounts[op.Name]++
			if first || op.StartTime.Before(oldestStart) {
				oldestStart = op.StartTime
				first = false
			}
		}

		stats.OperationCounts = operationCounts
		stats.OldestOperationAge = time.Since(oldestStart)
	}

	return stats
}

// BatchOperation represents an operation to be executed in a batch
type BatchOperation struct {
	Name string
	Tags map[string]string
	Fn   func(context.Context) error
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Index int
	Name  string
	Error error
}

// ConcurrentStats represents statistics for concurrent operations
type ConcurrentStats struct {
	MaxConcurrent      int            `json:"max_concurrent"`
	ActiveOperations   int            `json:"active_operations"`
	AvailableSlots     int            `json:"available_slots"`
	OperationCounts    map[string]int `json:"operation_counts,omitempty"`
	OldestOperationAge time.Duration  `json:"oldest_operation_age,omitempty"`
}

// WorkerPool represents a pool of workers for processing tasks
type WorkerPool struct {
	workers  int
	taskCh   chan Task
	resultCh chan TaskResult
	stopCh   chan struct{}
	wg       sync.WaitGroup
	monitor  *Monitor
	started  bool
	mutex    sync.Mutex
}

// Task represents a task to be processed by the worker pool
type Task struct {
	ID   string
	Name string
	Data interface{}
	Fn   func(interface{}) (interface{}, error)
	Tags map[string]string
}

// TaskResult represents the result of a task
type TaskResult struct {
	TaskID   string
	Result   interface{}
	Error    error
	Duration time.Duration
}

// WorkerPoolConfig represents configuration for a worker pool
type WorkerPoolConfig struct {
	Workers    int `json:"workers"`
	BufferSize int `json:"buffer_size"`
}

// DefaultWorkerPoolConfig returns a default worker pool configuration
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		Workers:    5,
		BufferSize: 100,
	}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config *WorkerPoolConfig, monitor *Monitor) *WorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	return &WorkerPool{
		workers:  config.Workers,
		taskCh:   make(chan Task, config.BufferSize),
		resultCh: make(chan TaskResult, config.BufferSize),
		stopCh:   make(chan struct{}),
		monitor:  monitor,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() error {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	if wp.started {
		return fmt.Errorf("worker pool already started")
	}

	// Start workers
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}

	wp.started = true
	return nil
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	if !wp.started {
		return
	}

	close(wp.stopCh)
	wp.wg.Wait()
	wp.started = false
}

// Submit submits a task to the worker pool
func (wp *WorkerPool) Submit(task Task) error {
	select {
	case wp.taskCh <- task:
		if wp.monitor != nil {
			wp.monitor.RecordCounter("worker_pool.task.submitted", 1, map[string]string{
				"task": task.Name,
			})
		}
		return nil
	default:
		return fmt.Errorf("worker pool task buffer full")
	}
}

// Results returns the result channel
func (wp *WorkerPool) Results() <-chan TaskResult {
	return wp.resultCh
}

// worker processes tasks from the task channel
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case task := <-wp.taskCh:
			startTime := time.Now()

			// Record task start
			if wp.monitor != nil {
				wp.monitor.RecordCounter("worker_pool.task.started", 1, map[string]string{
					"task":      task.Name,
					"worker_id": fmt.Sprintf("%d", id),
				})
			}

			// Process task
			result, err := task.Fn(task.Data)
			duration := time.Since(startTime)

			// Create result
			taskResult := TaskResult{
				TaskID:   task.ID,
				Result:   result,
				Error:    err,
				Duration: duration,
			}

			// Send result
			select {
			case wp.resultCh <- taskResult:
			default:
				// Result buffer full - this shouldn't happen in normal operation
			}

			// Record task completion
			if wp.monitor != nil {
				status := "success"
				if err != nil {
					status = "error"
				}

				tags := map[string]string{
					"task":      task.Name,
					"worker_id": fmt.Sprintf("%d", id),
					"status":    status,
				}

				wp.monitor.RecordCounter("worker_pool.task.completed", 1, tags)
				wp.monitor.RecordDuration("worker_pool.task.duration", duration, tags)
			}

		case <-wp.stopCh:
			return
		}
	}
}
