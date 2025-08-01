package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/cli"
	"github.com/nl-to-shell/nl-to-shell/internal/errors"
	"github.com/nl-to-shell/nl-to-shell/internal/performance"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// Version information - will be set during build
var (
	Version   = "0.1.0-dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// exitFunc is a variable to allow mocking os.Exit in tests
var exitFunc = os.Exit

// Application represents the main application with integrated monitoring and logging
type Application struct {
	monitor *performance.Monitor
	logger  errors.Logger
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewApplication creates a new application instance with full integration
func NewApplication() *Application {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize performance monitoring
	monitorConfig := &performance.MonitorConfig{
		Enabled:              true,
		MaxMetrics:           10000,
		CollectionInterval:   30 * time.Second,
		EnableMemoryStats:    true,
		EnableGoroutineStats: true,
	}
	monitor := performance.NewMonitor(monitorConfig)

	// Initialize structured logging
	logger := errors.NewStructuredLogger(false)

	// Set global logger for error handling
	errors.SetGlobalLogger(logger)

	app := &Application{
		monitor: monitor,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Record application startup metrics
	app.recordStartupMetrics()

	return app
}

// Run executes the main application logic with comprehensive error handling and monitoring
func (app *Application) Run() error {
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start monitoring application lifecycle
	timer := app.monitor.StartTimer("application.total_runtime", map[string]string{
		"version": Version,
	})
	defer timer.Stop()

	// Record application start
	app.monitor.RecordCounter("application.starts", 1, map[string]string{
		"version": Version,
	})

	// Run CLI in a goroutine to allow for signal handling
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Handle panics gracefully
				err := &types.NLShellError{
					Type:      types.ErrTypeValidation,
					Message:   fmt.Sprintf("application panic: %v", r),
					Severity:  types.SeverityCritical,
					Timestamp: time.Now(),
					Context: map[string]interface{}{
						"panic_value": r,
						"version":     Version,
						"git_commit":  GitCommit,
					},
				}
				app.logger.LogError(err)
				errChan <- err
			}
		}()

		// Execute CLI with context
		if err := cli.ExecuteWithContext(app.ctx); err != nil {
			// Log the error with full context
			nlErr, ok := err.(*types.NLShellError)
			if !ok {
				nlErr = &types.NLShellError{
					Type:      types.ErrTypeValidation,
					Message:   err.Error(),
					Cause:     err,
					Severity:  types.SeverityError,
					Timestamp: time.Now(),
					Context: map[string]interface{}{
						"version":    Version,
						"git_commit": GitCommit,
					},
				}
			}
			app.logger.LogError(nlErr)
			errChan <- err
		} else {
			errChan <- nil
		}
	}()

	// Wait for either completion or signal
	select {
	case err := <-errChan:
		if err != nil {
			app.monitor.RecordCounter("application.errors", 1, map[string]string{
				"error_type": fmt.Sprintf("%T", err),
			})
			return err
		}
		app.monitor.RecordCounter("application.successful_completions", 1, nil)
		return nil

	case sig := <-sigChan:
		app.monitor.RecordCounter("application.signal_shutdowns", 1, map[string]string{
			"signal": sig.String(),
		})

		fmt.Fprintf(os.Stderr, "\nReceived signal %s, shutting down gracefully...\n", sig)

		// Cancel context to signal shutdown
		app.cancel()

		// Wait for CLI to finish with timeout
		shutdownTimer := time.NewTimer(5 * time.Second)
		defer shutdownTimer.Stop()

		select {
		case err := <-errChan:
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
			}
		case <-shutdownTimer.C:
			fmt.Fprintf(os.Stderr, "Shutdown timeout exceeded, forcing exit\n")
		}

		return nil
	}
}

// Shutdown performs graceful application shutdown
func (app *Application) Shutdown() {
	// Record shutdown metrics
	app.recordShutdownMetrics()

	// Close monitoring
	if app.monitor != nil {
		app.monitor.Close()
	}

	// Cancel context
	if app.cancel != nil {
		app.cancel()
	}
}

// recordStartupMetrics records application startup metrics
func (app *Application) recordStartupMetrics() {
	now := time.Now()

	// Record version information
	app.monitor.RecordGauge("application.version_info", 1, "info", map[string]string{
		"version":    Version,
		"git_commit": GitCommit,
		"build_date": BuildDate,
	})

	// Record runtime information
	app.monitor.RecordGauge("application.go_version", 1, "info", map[string]string{
		"version": runtime.Version(),
	})

	app.monitor.RecordGauge("application.startup_time", float64(now.Unix()), "timestamp", nil)

	// Record system information
	app.monitor.RecordGauge("application.cpu_count", float64(runtime.NumCPU()), "count", nil)
	app.monitor.RecordGauge("application.max_procs", float64(runtime.GOMAXPROCS(0)), "count", nil)
}

// recordShutdownMetrics records application shutdown metrics
func (app *Application) recordShutdownMetrics() {
	app.monitor.RecordGauge("application.shutdown_time", float64(time.Now().Unix()), "timestamp", nil)

	// Record final memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	app.monitor.RecordGauge("application.final_memory_alloc", float64(memStats.Alloc), "bytes", nil)
	app.monitor.RecordGauge("application.final_gc_count", float64(memStats.NumGC), "count", nil)
}

func main() {
	// Create application with full integration
	app := NewApplication()
	defer app.Shutdown()

	// Run application with comprehensive error handling
	if err := app.Run(); err != nil {
		// Final error logging
		fmt.Fprintf(os.Stderr, "Application failed: %v\n", err)
		exitFunc(1)
	}
}
