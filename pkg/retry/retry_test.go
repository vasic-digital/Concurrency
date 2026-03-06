package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 3, cfg.MaxAttempts)
	assert.Equal(t, time.Second, cfg.InitialDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.Equal(t, 2.0, cfg.BackoffFactor)
	assert.True(t, cfg.Jitter)
}

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: time.Millisecond}
	calls := 0
	err := Do(context.Background(), cfg, func() error {
		calls++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	cfg := Config{MaxAttempts: 5, InitialDelay: time.Millisecond, BackoffFactor: 1.0}
	calls := 0
	err := Do(context.Background(), cfg, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestDo_AllAttemptsFail(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: time.Millisecond, BackoffFactor: 1.0}
	calls := 0
	err := Do(context.Background(), cfg, func() error {
		calls++
		return errors.New("persistent")
	})
	assert.Error(t, err)
	assert.Equal(t, "persistent", err.Error())
	assert.Equal(t, 3, calls)
}

func TestDo_NonRetryableError(t *testing.T) {
	cfg := Config{MaxAttempts: 5, InitialDelay: time.Millisecond}
	calls := 0
	err := Do(context.Background(), cfg, func() error {
		calls++
		return NewRetryableError(errors.New("fatal"), false)
	})
	assert.Error(t, err)
	assert.Equal(t, 1, calls)
}

func TestDo_ContextCancelled(t *testing.T) {
	cfg := Config{MaxAttempts: 10, InitialDelay: 100 * time.Millisecond, BackoffFactor: 1.0}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := Do(ctx, cfg, func() error {
		return errors.New("keep trying")
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDoWithCallbacks_OnRetry(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: time.Millisecond, BackoffFactor: 1.0}
	retries := 0
	calls := 0

	err := DoWithCallbacks(context.Background(), cfg,
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
		nil,
	)
	assert.NoError(t, err)
	assert.Equal(t, 2, retries)
}

func TestDoWithCallbacks_OnFinalError(t *testing.T) {
	cfg := Config{MaxAttempts: 2, InitialDelay: time.Millisecond, BackoffFactor: 1.0}
	finalCalled := false

	err := DoWithCallbacks(context.Background(), cfg,
		func(attempt int) error {
			return errors.New("always fail")
		},
		nil,
		func(attempt int, err error) {
			finalCalled = true
			assert.Equal(t, 2, attempt)
		},
	)
	assert.Error(t, err)
	assert.True(t, finalCalled)
}

func TestRetryableError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	re := NewRetryableError(inner, true)
	assert.True(t, errors.Is(re, inner))
	assert.Equal(t, "inner", re.Error())
	assert.True(t, re.IsRetryable())
}

func TestExponentialBackoff_Execute(t *testing.T) {
	eb := NewExponentialBackoff(Config{
		MaxAttempts:   3,
		InitialDelay:  time.Millisecond,
		BackoffFactor: 1.0,
	})

	calls := 0
	err := eb.Execute(context.Background(), func() error {
		calls++
		if calls < 2 {
			return errors.New("retry")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestDoWithCallbacks_DefaultConfig(t *testing.T) {
	cfg := Config{} // all zeros
	calls := 0
	err := DoWithCallbacks(context.Background(), cfg,
		func(attempt int) error {
			calls++
			return nil
		}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}
