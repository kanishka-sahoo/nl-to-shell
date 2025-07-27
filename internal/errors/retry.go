package errors

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// RetryPolicy defines the retry behavior
type RetryPolicy struct {
	MaxAttempts        int               // Maximum number of retry attempts
	InitialDelay       time.Duration     // Initial delay between retries
	MaxDelay           time.Duration     // Maximum delay between retries
	BackoffFactor      float64           // Exponential backoff factor
	Jitter             bool              // Whether to add random jitter
	RetryableErrors    []types.ErrorType // Error types that should be retried
	NonRetryableErrors []types.ErrorType // Error types that should never be retried
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryableErrors: []types.ErrorType{
			types.ErrTypeNetwork,
			types.ErrTypeTimeout,
			types.ErrTypeProvider, // LLM provider might have temporary issues
		},
		NonRetryableErrors: []types.ErrorType{
			types.ErrTypeAuth,          // Authentication errors shouldn't be retried
			types.ErrTypePermission,    // Permission errors won't resolve with retry
			types.ErrTypeValidation,    // Validation errors are permanent
			types.ErrTypeConfiguration, // Config errors need manual intervention
		},
	}
}

// NetworkRetryPolicy returns a retry policy optimized for network operations
func NetworkRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   5,
		InitialDelay:  200 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryableErrors: []types.ErrorType{
			types.ErrTypeNetwork,
			types.ErrTypeTimeout,
		},
		NonRetryableErrors: []types.ErrorType{
			types.ErrTypeAuth,
			types.ErrTypePermission,
			types.ErrTypeValidation,
		},
	}
}

// ProviderRetryPolicy returns a retry policy optimized for LLM provider operations
func ProviderRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   4,
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.5,
		Jitter:        true,
		RetryableErrors: []types.ErrorType{
			types.ErrTypeProvider,
			types.ErrTypeNetwork,
			types.ErrTypeTimeout,
		},
		NonRetryableErrors: []types.ErrorType{
			types.ErrTypeAuth,
			types.ErrTypeValidation,
		},
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func(ctx context.Context, attempt int) error

// RetryResult contains the result of a retry operation
type RetryResult struct {
	Success    bool
	Attempts   int
	TotalDelay time.Duration
	LastError  error
	AllErrors  []error
}

// Retrier handles retry logic with various policies
type Retrier struct {
	policy *RetryPolicy
	logger Logger
}

// NewRetrier creates a new retrier with the given policy
func NewRetrier(policy *RetryPolicy) *Retrier {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	return &Retrier{
		policy: policy,
		logger: GetGlobalLogger(),
	}
}

// WithLogger sets a custom logger for the retrier
func (r *Retrier) WithLogger(logger Logger) *Retrier {
	r.logger = logger
	return r
}

// Retry executes the given function with retry logic
func (r *Retrier) Retry(ctx context.Context, fn RetryableFunc) *RetryResult {
	result := &RetryResult{
		AllErrors: make([]error, 0),
	}

	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		result.Attempts = attempt

		// Execute the function
		err := fn(ctx, attempt)
		if err == nil {
			result.Success = true
			return result
		}

		result.LastError = err
		result.AllErrors = append(result.AllErrors, err)

		// Check if we should retry this error
		if !r.shouldRetry(err, attempt) {
			break
		}

		// Don't delay after the last attempt
		if attempt < r.policy.MaxAttempts {
			delay := r.calculateDelay(attempt)
			result.TotalDelay += delay

			// Log the retry attempt
			if r.logger != nil {
				if nlErr, ok := err.(*types.NLShellError); ok {
					retryErr := NewInternalError(
						fmt.Sprintf("Retrying operation after failure (attempt %d/%d)", attempt, r.policy.MaxAttempts),
						nlErr,
					).WithContext("retry_attempt", attempt).
						WithContext("next_delay", delay.String()).
						WithContext("original_error", err.Error())
					retryErr.Severity = types.SeverityWarning
					r.logger.LogErrorWithContext(ctx, retryErr)
				}
			}

			// Wait before retrying
			select {
			case <-ctx.Done():
				result.LastError = NewTimeoutError("retry cancelled due to context cancellation", ctx.Err())
				return result
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	return result
}

// shouldRetry determines if an error should be retried
func (r *Retrier) shouldRetry(err error, attempt int) bool {
	if attempt >= r.policy.MaxAttempts {
		return false
	}

	// Check if it's an NLShellError
	var nlErr *types.NLShellError
	if !AsNLShellError(err, &nlErr) {
		// For non-NLShellError, only retry network-like errors
		return false
	}

	// Check non-retryable errors first (they take precedence)
	for _, nonRetryableType := range r.policy.NonRetryableErrors {
		if nlErr.Type == nonRetryableType {
			return false
		}
	}

	// Check retryable errors
	for _, retryableType := range r.policy.RetryableErrors {
		if nlErr.Type == retryableType {
			return true
		}
	}

	// Default to not retrying
	return false
}

// calculateDelay calculates the delay for the given attempt
func (r *Retrier) calculateDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	delay := float64(r.policy.InitialDelay) * math.Pow(r.policy.BackoffFactor, float64(attempt-1))

	// Apply maximum delay limit
	if delay > float64(r.policy.MaxDelay) {
		delay = float64(r.policy.MaxDelay)
	}

	// Add jitter if enabled
	if r.policy.Jitter {
		// Add up to 25% random jitter
		jitter := delay * 0.25 * rand.Float64()
		delay += jitter
	}

	return time.Duration(delay)
}

// RecoveryStrategy defines how to recover from specific error types
type RecoveryStrategy interface {
	CanRecover(err *types.NLShellError) bool
	Recover(ctx context.Context, err *types.NLShellError) error
}

// RecoveryManager manages recovery strategies for different error types
type RecoveryManager struct {
	strategies map[types.ErrorType][]RecoveryStrategy
	logger     Logger
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager() *RecoveryManager {
	return &RecoveryManager{
		strategies: make(map[types.ErrorType][]RecoveryStrategy),
		logger:     GetGlobalLogger(),
	}
}

// WithLogger sets a custom logger for the recovery manager
func (rm *RecoveryManager) WithLogger(logger Logger) *RecoveryManager {
	rm.logger = logger
	return rm
}

// RegisterStrategy registers a recovery strategy for a specific error type
func (rm *RecoveryManager) RegisterStrategy(errorType types.ErrorType, strategy RecoveryStrategy) {
	if rm.strategies[errorType] == nil {
		rm.strategies[errorType] = make([]RecoveryStrategy, 0)
	}
	rm.strategies[errorType] = append(rm.strategies[errorType], strategy)
}

// TryRecover attempts to recover from an error using registered strategies
func (rm *RecoveryManager) TryRecover(ctx context.Context, err *types.NLShellError) error {
	if err == nil {
		return nil
	}

	strategies, exists := rm.strategies[err.Type]
	if !exists || len(strategies) == 0 {
		return err // No recovery strategies available
	}

	// Try each strategy in order
	for i, strategy := range strategies {
		if strategy.CanRecover(err) {
			if rm.logger != nil {
				recoveryErr := NewInternalError(
					fmt.Sprintf("Attempting recovery for %s error using strategy %d", err.Type.String(), i+1),
					err,
				).WithContext("recovery_strategy", fmt.Sprintf("%T", strategy)).
					WithContext("original_error", err.Error())
				recoveryErr.Severity = types.SeverityInfo
				rm.logger.LogErrorWithContext(ctx, recoveryErr)
			}

			if recoveryErr := strategy.Recover(ctx, err); recoveryErr == nil {
				// Recovery successful
				if rm.logger != nil {
					successErr := NewInternalError(
						fmt.Sprintf("Successfully recovered from %s error", err.Type.String()),
						nil,
					).WithContext("recovery_strategy", fmt.Sprintf("%T", strategy)).
						WithContext("original_error", err.Error())
					successErr.Severity = types.SeverityInfo
					rm.logger.LogErrorWithContext(ctx, successErr)
				}
				return nil
			}
		}
	}

	// No recovery strategy worked
	return err
}

// Built-in recovery strategies

// ConfigReloadStrategy attempts to recover from configuration errors by reloading config
type ConfigReloadStrategy struct {
	configReloader func(ctx context.Context) error
}

// NewConfigReloadStrategy creates a new config reload strategy
func NewConfigReloadStrategy(reloader func(ctx context.Context) error) *ConfigReloadStrategy {
	return &ConfigReloadStrategy{
		configReloader: reloader,
	}
}

// CanRecover checks if this strategy can recover from the error
func (s *ConfigReloadStrategy) CanRecover(err *types.NLShellError) bool {
	return err.Type == types.ErrTypeConfiguration && s.configReloader != nil
}

// Recover attempts to recover by reloading configuration
func (s *ConfigReloadStrategy) Recover(ctx context.Context, err *types.NLShellError) error {
	return s.configReloader(ctx)
}

// ProviderFallbackStrategy attempts to recover from provider errors by switching providers
type ProviderFallbackStrategy struct {
	fallbackProviders []string
	switchProvider    func(ctx context.Context, provider string) error
}

// NewProviderFallbackStrategy creates a new provider fallback strategy
func NewProviderFallbackStrategy(fallbackProviders []string, switchFn func(ctx context.Context, provider string) error) *ProviderFallbackStrategy {
	return &ProviderFallbackStrategy{
		fallbackProviders: fallbackProviders,
		switchProvider:    switchFn,
	}
}

// CanRecover checks if this strategy can recover from the error
func (s *ProviderFallbackStrategy) CanRecover(err *types.NLShellError) bool {
	return err.Type == types.ErrTypeProvider && len(s.fallbackProviders) > 0 && s.switchProvider != nil
}

// Recover attempts to recover by switching to a fallback provider
func (s *ProviderFallbackStrategy) Recover(ctx context.Context, err *types.NLShellError) error {
	for _, provider := range s.fallbackProviders {
		if switchErr := s.switchProvider(ctx, provider); switchErr == nil {
			return nil // Successfully switched to fallback provider
		}
	}
	return err // All fallback providers failed
}

// GracefulDegradationStrategy provides graceful degradation for non-critical failures
type GracefulDegradationStrategy struct {
	degradedModeHandler func(ctx context.Context, err *types.NLShellError) error
	criticalErrorTypes  []types.ErrorType
}

// NewGracefulDegradationStrategy creates a new graceful degradation strategy
func NewGracefulDegradationStrategy(handler func(ctx context.Context, err *types.NLShellError) error) *GracefulDegradationStrategy {
	return &GracefulDegradationStrategy{
		degradedModeHandler: handler,
		criticalErrorTypes: []types.ErrorType{
			types.ErrTypeSafety,     // Safety errors are always critical
			types.ErrTypeInternal,   // Internal errors are critical
			types.ErrTypePermission, // Permission errors can't be degraded
		},
	}
}

// CanRecover checks if this strategy can recover from the error
func (s *GracefulDegradationStrategy) CanRecover(err *types.NLShellError) bool {
	// Don't degrade for critical error types
	for _, criticalType := range s.criticalErrorTypes {
		if err.Type == criticalType {
			return false
		}
	}
	return s.degradedModeHandler != nil
}

// Recover attempts to recover by entering degraded mode
func (s *GracefulDegradationStrategy) Recover(ctx context.Context, err *types.NLShellError) error {
	return s.degradedModeHandler(ctx, err)
}

// Helper functions

// AsNLShellError checks if an error is or wraps an NLShellError
func AsNLShellError(err error, target **types.NLShellError) bool {
	if err == nil {
		return false
	}

	// Direct type assertion
	if nlErr, ok := err.(*types.NLShellError); ok {
		*target = nlErr
		return true
	}

	// Check if it wraps an NLShellError
	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		return AsNLShellError(unwrapper.Unwrap(), target)
	}

	return false
}

// RetryWithPolicy is a convenience function for retrying with a specific policy
func RetryWithPolicy(ctx context.Context, policy *RetryPolicy, fn RetryableFunc) *RetryResult {
	retrier := NewRetrier(policy)
	return retrier.Retry(ctx, fn)
}

// RetryWithDefaultPolicy is a convenience function for retrying with the default policy
func RetryWithDefaultPolicy(ctx context.Context, fn RetryableFunc) *RetryResult {
	return RetryWithPolicy(ctx, DefaultRetryPolicy(), fn)
}

// RetryNetworkOperation is a convenience function for retrying network operations
func RetryNetworkOperation(ctx context.Context, fn RetryableFunc) *RetryResult {
	return RetryWithPolicy(ctx, NetworkRetryPolicy(), fn)
}

// RetryProviderOperation is a convenience function for retrying provider operations
func RetryProviderOperation(ctx context.Context, fn RetryableFunc) *RetryResult {
	return RetryWithPolicy(ctx, ProviderRetryPolicy(), fn)
}
