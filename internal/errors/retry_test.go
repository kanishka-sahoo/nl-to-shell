package errors

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay to be 100ms, got %v", policy.InitialDelay)
	}
	if policy.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor to be 2.0, got %f", policy.BackoffFactor)
	}
	if !policy.Jitter {
		t.Error("Expected Jitter to be true")
	}

	// Check retryable errors
	expectedRetryable := []types.ErrorType{
		types.ErrTypeNetwork,
		types.ErrTypeTimeout,
		types.ErrTypeProvider,
	}
	if len(policy.RetryableErrors) != len(expectedRetryable) {
		t.Errorf("Expected %d retryable errors, got %d", len(expectedRetryable), len(policy.RetryableErrors))
	}

	// Check non-retryable errors
	expectedNonRetryable := []types.ErrorType{
		types.ErrTypeAuth,
		types.ErrTypePermission,
		types.ErrTypeValidation,
		types.ErrTypeConfiguration,
	}
	if len(policy.NonRetryableErrors) != len(expectedNonRetryable) {
		t.Errorf("Expected %d non-retryable errors, got %d", len(expectedNonRetryable), len(policy.NonRetryableErrors))
	}
}

func TestNetworkRetryPolicy(t *testing.T) {
	policy := NetworkRetryPolicy()

	if policy.MaxAttempts != 5 {
		t.Errorf("Expected MaxAttempts to be 5, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 200*time.Millisecond {
		t.Errorf("Expected InitialDelay to be 200ms, got %v", policy.InitialDelay)
	}
}

func TestProviderRetryPolicy(t *testing.T) {
	policy := ProviderRetryPolicy()

	if policy.MaxAttempts != 4 {
		t.Errorf("Expected MaxAttempts to be 4, got %d", policy.MaxAttempts)
	}
	if policy.BackoffFactor != 2.5 {
		t.Errorf("Expected BackoffFactor to be 2.5, got %f", policy.BackoffFactor)
	}
}

func TestRetrier_Retry_Success(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:     3,
		InitialDelay:    10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []types.ErrorType{types.ErrTypeNetwork},
	}
	retrier := NewRetrier(policy)

	callCount := 0
	fn := func(ctx context.Context, attempt int) error {
		callCount++
		if callCount < 2 {
			return NewNetworkError("temporary failure", nil)
		}
		return nil // Success on second attempt
	}

	ctx := context.Background()
	result := retrier.Retry(ctx, fn)

	if !result.Success {
		t.Error("Expected retry to succeed")
	}
	if result.Attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", result.Attempts)
	}
	if callCount != 2 {
		t.Errorf("Expected function to be called 2 times, got %d", callCount)
	}
}

func TestRetrier_Retry_MaxAttemptsReached(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:     2,
		InitialDelay:    10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []types.ErrorType{types.ErrTypeNetwork},
	}
	retrier := NewRetrier(policy)

	callCount := 0
	fn := func(ctx context.Context, attempt int) error {
		callCount++
		return NewNetworkError("persistent failure", nil)
	}

	ctx := context.Background()
	result := retrier.Retry(ctx, fn)

	if result.Success {
		t.Error("Expected retry to fail")
	}
	if result.Attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", result.Attempts)
	}
	if callCount != 2 {
		t.Errorf("Expected function to be called 2 times, got %d", callCount)
	}
	if len(result.AllErrors) != 2 {
		t.Errorf("Expected 2 errors in AllErrors, got %d", len(result.AllErrors))
	}
}

func TestRetrier_Retry_NonRetryableError(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:        3,
		InitialDelay:       10 * time.Millisecond,
		BackoffFactor:      2.0,
		RetryableErrors:    []types.ErrorType{types.ErrTypeNetwork},
		NonRetryableErrors: []types.ErrorType{types.ErrTypeAuth},
	}
	retrier := NewRetrier(policy)

	callCount := 0
	fn := func(ctx context.Context, attempt int) error {
		callCount++
		return NewAuthError("authentication failed", nil)
	}

	ctx := context.Background()
	result := retrier.Retry(ctx, fn)

	if result.Success {
		t.Error("Expected retry to fail")
	}
	if result.Attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", result.Attempts)
	}
	if callCount != 1 {
		t.Errorf("Expected function to be called 1 time, got %d", callCount)
	}
}

func TestRetrier_Retry_ContextCancellation(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:     5,
		InitialDelay:    50 * time.Millisecond, // Reduced delay to ensure timeout happens during retry
		BackoffFactor:   2.0,
		RetryableErrors: []types.ErrorType{types.ErrTypeNetwork},
	}
	retrier := NewRetrier(policy)

	callCount := 0
	fn := func(ctx context.Context, attempt int) error {
		callCount++
		return NewNetworkError("network failure", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond) // Allow time for first call + partial delay
	defer cancel()

	result := retrier.Retry(ctx, fn)

	if result.Success {
		t.Error("Expected retry to fail due to context cancellation")
	}
	if callCount == 0 {
		t.Error("Expected at least one function call")
	}

	// The last error could be either the timeout error (if context cancelled during delay)
	// or the original network error (if context cancelled before retry logic kicks in)
	var nlErr *types.NLShellError
	if AsNLShellError(result.LastError, &nlErr) {
		if nlErr.Type != types.ErrTypeTimeout && nlErr.Type != types.ErrTypeNetwork {
			t.Errorf("Expected last error to be timeout or network, got %v", nlErr.Type)
		}
	} else {
		t.Error("Expected last error to be NLShellError")
	}
}

func TestRetrier_shouldRetry(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:        3,
		RetryableErrors:    []types.ErrorType{types.ErrTypeNetwork, types.ErrTypeTimeout},
		NonRetryableErrors: []types.ErrorType{types.ErrTypeAuth, types.ErrTypeValidation},
	}
	retrier := NewRetrier(policy)

	tests := []struct {
		name     string
		err      error
		attempt  int
		expected bool
	}{
		{
			name:     "retryable network error",
			err:      NewNetworkError("network failure", nil),
			attempt:  1,
			expected: true,
		},
		{
			name:     "retryable timeout error",
			err:      NewTimeoutError("timeout", nil),
			attempt:  1,
			expected: true,
		},
		{
			name:     "non-retryable auth error",
			err:      NewAuthError("auth failure", nil),
			attempt:  1,
			expected: false,
		},
		{
			name:     "non-retryable validation error",
			err:      NewValidationError("validation failure", nil),
			attempt:  1,
			expected: false,
		},
		{
			name:     "max attempts reached",
			err:      NewNetworkError("network failure", nil),
			attempt:  3,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			attempt:  1,
			expected: false,
		},
		{
			name:     "unknown error type",
			err:      NewInternalError("internal error", nil),
			attempt:  1,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := retrier.shouldRetry(tt.err, tt.attempt)
			if result != tt.expected {
				t.Errorf("shouldRetry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRetrier_calculateDelay(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false, // Disable jitter for predictable testing
	}
	retrier := NewRetrier(policy)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second}, // Capped at MaxDelay
		{6, 1 * time.Second}, // Still capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.attempt)), func(t *testing.T) {
			delay := retrier.calculateDelay(tt.attempt)
			if delay != tt.expected {
				t.Errorf("calculateDelay(%d) = %v, want %v", tt.attempt, delay, tt.expected)
			}
		})
	}
}

func TestRetrier_calculateDelay_WithJitter(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
	retrier := NewRetrier(policy)

	// With jitter, the delay should be within a reasonable range
	baseDelay := 100 * time.Millisecond
	delay := retrier.calculateDelay(1)

	// Should be between base delay and base delay + 25% jitter
	minDelay := baseDelay
	maxDelay := time.Duration(float64(baseDelay) * 1.25)

	if delay < minDelay || delay > maxDelay {
		t.Errorf("calculateDelay(1) with jitter = %v, expected between %v and %v", delay, minDelay, maxDelay)
	}
}

func TestRecoveryManager_RegisterStrategy(t *testing.T) {
	rm := NewRecoveryManager()
	strategy := &mockRecoveryStrategy{}

	rm.RegisterStrategy(types.ErrTypeNetwork, strategy)

	strategies, exists := rm.strategies[types.ErrTypeNetwork]
	if !exists {
		t.Error("Expected strategy to be registered")
	}
	if len(strategies) != 1 {
		t.Errorf("Expected 1 strategy, got %d", len(strategies))
	}
	if strategies[0] != strategy {
		t.Error("Expected registered strategy to match")
	}
}

func TestRecoveryManager_TryRecover_Success(t *testing.T) {
	rm := NewRecoveryManager()
	strategy := &mockRecoveryStrategy{canRecover: true, recoverError: nil}

	rm.RegisterStrategy(types.ErrTypeNetwork, strategy)

	err := NewNetworkError("network failure", nil)
	result := rm.TryRecover(context.Background(), err)

	if result != nil {
		t.Errorf("Expected recovery to succeed, got error: %v", result)
	}
	if !strategy.recoverCalled {
		t.Error("Expected Recover to be called")
	}
}

func TestRecoveryManager_TryRecover_Failure(t *testing.T) {
	rm := NewRecoveryManager()
	strategy := &mockRecoveryStrategy{canRecover: true, recoverError: errors.New("recovery failed")}

	rm.RegisterStrategy(types.ErrTypeNetwork, strategy)

	err := NewNetworkError("network failure", nil)
	result := rm.TryRecover(context.Background(), err)

	if result == nil {
		t.Error("Expected recovery to fail")
	}
	if !strategy.recoverCalled {
		t.Error("Expected Recover to be called")
	}
}

func TestRecoveryManager_TryRecover_NoStrategy(t *testing.T) {
	rm := NewRecoveryManager()

	err := NewNetworkError("network failure", nil)
	result := rm.TryRecover(context.Background(), err)

	if result != err {
		t.Error("Expected original error to be returned when no strategy available")
	}
}

func TestRecoveryManager_TryRecover_CannotRecover(t *testing.T) {
	rm := NewRecoveryManager()
	strategy := &mockRecoveryStrategy{canRecover: false}

	rm.RegisterStrategy(types.ErrTypeNetwork, strategy)

	err := NewNetworkError("network failure", nil)
	result := rm.TryRecover(context.Background(), err)

	if result != err {
		t.Error("Expected original error to be returned when strategy cannot recover")
	}
	if strategy.recoverCalled {
		t.Error("Expected Recover not to be called when CanRecover returns false")
	}
}

func TestConfigReloadStrategy(t *testing.T) {
	reloadCalled := false
	reloader := func(ctx context.Context) error {
		reloadCalled = true
		return nil
	}

	strategy := NewConfigReloadStrategy(reloader)

	// Test CanRecover
	configErr := NewConfigurationError("config error", nil)
	if !strategy.CanRecover(configErr) {
		t.Error("Expected strategy to be able to recover from config error")
	}

	networkErr := NewNetworkError("network error", nil)
	if strategy.CanRecover(networkErr) {
		t.Error("Expected strategy not to be able to recover from network error")
	}

	// Test Recover
	err := strategy.Recover(context.Background(), configErr)
	if err != nil {
		t.Errorf("Expected recovery to succeed, got error: %v", err)
	}
	if !reloadCalled {
		t.Error("Expected reloader to be called")
	}
}

func TestProviderFallbackStrategy(t *testing.T) {
	switchCalled := false
	var switchedProvider string
	switchFn := func(ctx context.Context, provider string) error {
		switchCalled = true
		switchedProvider = provider
		return nil
	}

	fallbackProviders := []string{"provider1", "provider2"}
	strategy := NewProviderFallbackStrategy(fallbackProviders, switchFn)

	// Test CanRecover
	providerErr := NewProviderError("provider error", nil)
	if !strategy.CanRecover(providerErr) {
		t.Error("Expected strategy to be able to recover from provider error")
	}

	networkErr := NewNetworkError("network error", nil)
	if strategy.CanRecover(networkErr) {
		t.Error("Expected strategy not to be able to recover from network error")
	}

	// Test Recover
	err := strategy.Recover(context.Background(), providerErr)
	if err != nil {
		t.Errorf("Expected recovery to succeed, got error: %v", err)
	}
	if !switchCalled {
		t.Error("Expected switch function to be called")
	}
	if switchedProvider != "provider1" {
		t.Errorf("Expected to switch to provider1, got %s", switchedProvider)
	}
}

func TestGracefulDegradationStrategy(t *testing.T) {
	handlerCalled := false
	handler := func(ctx context.Context, err *types.NLShellError) error {
		handlerCalled = true
		return nil
	}

	strategy := NewGracefulDegradationStrategy(handler)

	// Test CanRecover - should work for non-critical errors
	networkErr := NewNetworkError("network error", nil)
	if !strategy.CanRecover(networkErr) {
		t.Error("Expected strategy to be able to recover from network error")
	}

	// Test CanRecover - should not work for critical errors
	safetyErr := NewSafetyError("safety error", nil)
	if strategy.CanRecover(safetyErr) {
		t.Error("Expected strategy not to be able to recover from safety error")
	}

	internalErr := NewInternalError("internal error", nil)
	if strategy.CanRecover(internalErr) {
		t.Error("Expected strategy not to be able to recover from internal error")
	}

	permissionErr := NewPermissionError("permission error", nil)
	if strategy.CanRecover(permissionErr) {
		t.Error("Expected strategy not to be able to recover from permission error")
	}

	// Test Recover
	err := strategy.Recover(context.Background(), networkErr)
	if err != nil {
		t.Errorf("Expected recovery to succeed, got error: %v", err)
	}
	if !handlerCalled {
		t.Error("Expected handler to be called")
	}
}

func TestAsNLShellError(t *testing.T) {
	// Test with NLShellError
	nlErr := NewNetworkError("network error", nil)
	var target *types.NLShellError
	if !AsNLShellError(nlErr, &target) {
		t.Error("Expected AsNLShellError to return true for NLShellError")
	}
	if target != nlErr {
		t.Error("Expected target to be set to the original error")
	}

	// Test with regular error
	regularErr := errors.New("regular error")
	target = nil
	if AsNLShellError(regularErr, &target) {
		t.Error("Expected AsNLShellError to return false for regular error")
	}
	if target != nil {
		t.Error("Expected target to remain nil for regular error")
	}

	// Test with nil error
	target = nil
	if AsNLShellError(nil, &target) {
		t.Error("Expected AsNLShellError to return false for nil error")
	}
	if target != nil {
		t.Error("Expected target to remain nil for nil error")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	ctx := context.Background()

	// Test successful operation
	successFn := func(ctx context.Context, attempt int) error {
		return nil
	}

	result := RetryWithDefaultPolicy(ctx, successFn)
	if !result.Success {
		t.Error("Expected successful operation to succeed")
	}
	if result.Attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", result.Attempts)
	}

	// Test network retry
	networkResult := RetryNetworkOperation(ctx, successFn)
	if !networkResult.Success {
		t.Error("Expected network operation to succeed")
	}

	// Test provider retry
	providerResult := RetryProviderOperation(ctx, successFn)
	if !providerResult.Success {
		t.Error("Expected provider operation to succeed")
	}
}

// Mock recovery strategy for testing
type mockRecoveryStrategy struct {
	canRecover    bool
	recoverError  error
	recoverCalled bool
}

func (m *mockRecoveryStrategy) CanRecover(err *types.NLShellError) bool {
	return m.canRecover
}

func (m *mockRecoveryStrategy) Recover(ctx context.Context, err *types.NLShellError) error {
	m.recoverCalled = true
	return m.recoverError
}
