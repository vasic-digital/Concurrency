package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRetryLogger implements the Logger interface for testing.
type mockRetryLogger struct {
	debugMsgs []string
	infoMsgs  []string
	warnMsgs  []string
	errorMsgs []string
}

func (m *mockRetryLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.debugMsgs = append(m.debugMsgs, msg)
}

func (m *mockRetryLogger) Info(msg string, keysAndValues ...interface{}) {
	m.infoMsgs = append(m.infoMsgs, msg)
}

func (m *mockRetryLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.warnMsgs = append(m.warnMsgs, msg)
}

func (m *mockRetryLogger) Error(msg string, keysAndValues ...interface{}) {
	m.errorMsgs = append(m.errorMsgs, msg)
}

// TestDoWithCallbacks_WithLogger_SuccessOnFirstAttempt tests that Debug is
// called on the first attempt with a logger configured.
func TestDoWithCallbacks_WithLogger_SuccessOnFirstAttempt(t *testing.T) {
	logger := &mockRetryLogger{}
	cfg := Config{
		MaxAttempts:   3,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
		Logger:        logger,
	}

	err := DoWithCallbacks(context.Background(), cfg,
		func(attempt int) error {
			return nil
		}, nil, nil)
	require.NoError(t, err)
	assert.Len(t, logger.debugMsgs, 1)
	assert.Equal(t, "retry attempt", logger.debugMsgs[0])
	// No Info messages since it succeeded on first attempt (attempt == 0)
	assert.Len(t, logger.infoMsgs, 0)
}

// TestDoWithCallbacks_WithLogger_SuccessAfterRetry tests that Info is called
// when the operation succeeds after retries.
func TestDoWithCallbacks_WithLogger_SuccessAfterRetry(t *testing.T) {
	logger := &mockRetryLogger{}
	cfg := Config{
		MaxAttempts:   5,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
		Logger:        logger,
	}

	calls := 0
	err := DoWithCallbacks(context.Background(), cfg,
		func(attempt int) error {
			calls++
			if calls < 3 {
				return errors.New("transient")
			}
			return nil
		}, nil, nil)
	require.NoError(t, err)
	// Debug is called for each attempt (3 times)
	assert.Len(t, logger.debugMsgs, 3)
	// Info is called once for success after retry
	assert.Len(t, logger.infoMsgs, 1)
	assert.Equal(t, "operation succeeded after retry", logger.infoMsgs[0])
	// Warn is called for each failed attempt before retry (2 times)
	assert.Len(t, logger.warnMsgs, 2)
}

// TestDoWithCallbacks_WithLogger_AllFails tests that Error is logged when
// all attempts fail.
func TestDoWithCallbacks_WithLogger_AllFails(t *testing.T) {
	logger := &mockRetryLogger{}
	cfg := Config{
		MaxAttempts:   2,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
		Logger:        logger,
	}

	err := DoWithCallbacks(context.Background(), cfg,
		func(attempt int) error {
			return errors.New("persistent")
		}, nil, nil)
	assert.Error(t, err)
	assert.Len(t, logger.errorMsgs, 1)
	assert.Equal(t, "all retry attempts failed", logger.errorMsgs[0])
}

// TestDoWithCallbacks_WithLogger_NonRetryable tests that Debug "not retryable"
// is logged for non-retryable errors.
func TestDoWithCallbacks_WithLogger_NonRetryable(t *testing.T) {
	logger := &mockRetryLogger{}
	cfg := Config{
		MaxAttempts:   5,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
		Logger:        logger,
	}

	err := DoWithCallbacks(context.Background(), cfg,
		func(attempt int) error {
			return NewRetryableError(errors.New("fatal"), false)
		}, nil, nil)
	assert.Error(t, err)
	// Debug messages: 1 for "retry attempt" + 1 for "error is not retryable"
	assert.GreaterOrEqual(t, len(logger.debugMsgs), 2)
	assert.Equal(t, "error is not retryable", logger.debugMsgs[1])
}

// TestCalculateDelay_WithoutJitter tests calculateDelay without jitter.
func TestCalculateDelay_WithoutJitter(t *testing.T) {
	cfg := Config{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	// Attempt 0: 100ms * 2^0 = 100ms
	delay := calculateDelay(cfg, 0)
	assert.Equal(t, 100*time.Millisecond, delay)

	// Attempt 1: 100ms * 2^1 = 200ms
	delay = calculateDelay(cfg, 1)
	assert.Equal(t, 200*time.Millisecond, delay)

	// Attempt 2: 100ms * 2^2 = 400ms
	delay = calculateDelay(cfg, 2)
	assert.Equal(t, 400*time.Millisecond, delay)
}

// TestCalculateDelay_WithJitter tests calculateDelay with jitter enabled.
func TestCalculateDelay_WithJitter(t *testing.T) {
	cfg := Config{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}

	delay := calculateDelay(cfg, 0)
	// With jitter, delay should be >= 100ms and < 110ms (100ms + 10% of 100ms)
	assert.GreaterOrEqual(t, delay, 100*time.Millisecond)
	assert.Less(t, delay, 111*time.Millisecond)
}

// TestCalculateDelay_CappedByMaxDelay tests that calculateDelay caps at
// MaxDelay.
func TestCalculateDelay_CappedByMaxDelay(t *testing.T) {
	cfg := Config{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      500 * time.Millisecond,
		BackoffFactor: 10.0,
		Jitter:        false,
	}

	// Attempt 2: 100ms * 10^2 = 10000ms, should be capped at 500ms
	delay := calculateDelay(cfg, 2)
	assert.Equal(t, 500*time.Millisecond, delay)
}

// TestCalculateDelay_CappedByMaxDelayWithJitter tests that calculateDelay
// caps at MaxDelay even with jitter (jitter is applied on the capped value).
func TestCalculateDelay_CappedByMaxDelayWithJitter(t *testing.T) {
	cfg := Config{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      500 * time.Millisecond,
		BackoffFactor: 10.0,
		Jitter:        true,
	}

	delay := calculateDelay(cfg, 2)
	// Capped at 500ms + up to 10% jitter = max 550ms
	assert.GreaterOrEqual(t, delay, 500*time.Millisecond)
	assert.Less(t, delay, 551*time.Millisecond)
}

// TestExponentialBackoff_ExecuteWithCallbacks tests the
// ExecuteWithCallbacks method of ExponentialBackoff.
func TestExponentialBackoff_ExecuteWithCallbacks(t *testing.T) {
	eb := NewExponentialBackoff(Config{
		MaxAttempts:   3,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
	})

	retries := 0
	finalCalled := false
	calls := 0

	err := eb.ExecuteWithCallbacks(context.Background(),
		func(attempt int) error {
			calls++
			if calls < 3 {
				return errors.New("retry me")
			}
			return nil
		},
		func(attempt int, err error, delay time.Duration) {
			retries++
		},
		func(attempt int, err error) {
			finalCalled = true
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, 2, retries)
	assert.False(t, finalCalled) // Not called on success
	assert.Equal(t, 3, calls)
}

// TestExponentialBackoff_ExecuteWithCallbacks_AllFails tests
// ExecuteWithCallbacks when all attempts fail.
func TestExponentialBackoff_ExecuteWithCallbacks_AllFails(t *testing.T) {
	eb := NewExponentialBackoff(Config{
		MaxAttempts:   2,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
	})

	finalCalled := false
	var finalAttempt int

	err := eb.ExecuteWithCallbacks(context.Background(),
		func(attempt int) error {
			return errors.New("always fail")
		},
		nil,
		func(attempt int, err error) {
			finalCalled = true
			finalAttempt = attempt
		},
	)
	assert.Error(t, err)
	assert.True(t, finalCalled)
	assert.Equal(t, 2, finalAttempt)
}

// TestDoWithCallbacks_NonRetryableError_WithOnFinalError tests that
// onFinalError is called when a non-retryable error terminates early.
func TestDoWithCallbacks_NonRetryableError_WithOnFinalError(t *testing.T) {
	cfg := Config{
		MaxAttempts:   5,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
	}

	finalCalled := false
	err := DoWithCallbacks(context.Background(), cfg,
		func(attempt int) error {
			return NewRetryableError(errors.New("fatal"), false)
		},
		nil,
		func(attempt int, err error) {
			finalCalled = true
		},
	)
	assert.Error(t, err)
	assert.True(t, finalCalled)
}
