// Package retry provides configurable retry logic with exponential backoff.
//
// Design patterns: Strategy (backoff calculation), Template Method (retry
// callbacks for customization points).
package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Logger is a minimal logging interface for retry operations.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// Config contains configuration for retry logic.
type Config struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	Jitter        bool
	Logger        Logger
}

// DefaultConfig returns a default retry configuration.
func DefaultConfig() Config {
	return Config{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

// RetryableError represents an error that may or may not be retryable.
type RetryableError struct {
	Err       error
	Retryable bool
}

func (e RetryableError) Error() string {
	return e.Err.Error()
}

func (e RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is retryable.
func (e RetryableError) IsRetryable() bool {
	return e.Retryable
}

// NewRetryableError creates a new retryable error.
func NewRetryableError(err error, retryable bool) RetryableError {
	return RetryableError{Err: err, Retryable: retryable}
}

// Func is a function that can be retried.
type Func func() error

// CallbackFunc is a function that receives the attempt number.
type CallbackFunc func(attempt int) error

// Do executes fn with retry logic.
func Do(ctx context.Context, cfg Config, fn Func) error {
	return DoWithCallbacks(ctx, cfg, func(attempt int) error {
		return fn()
	}, nil, nil)
}

// DoWithCallbacks executes fn with retry logic and lifecycle callbacks.
func DoWithCallbacks(
	ctx context.Context,
	cfg Config,
	fn CallbackFunc,
	onRetry func(attempt int, err error, delay time.Duration),
	onFinalError func(attempt int, err error),
) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 1 * time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.BackoffFactor <= 0 {
		cfg.BackoffFactor = 2.0
	}

	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if cfg.Logger != nil {
			cfg.Logger.Debug("retry attempt",
				"attempt", attempt+1, "max_attempts", cfg.MaxAttempts)
		}

		err := fn(attempt)
		if err == nil {
			if cfg.Logger != nil && attempt > 0 {
				cfg.Logger.Info("operation succeeded after retry", "attempts", attempt+1)
			}
			return nil
		}

		lastErr = err

		if retryableErr, ok := err.(RetryableError); ok && !retryableErr.IsRetryable() {
			if cfg.Logger != nil {
				cfg.Logger.Debug("error is not retryable", "error", err)
			}
			break
		}

		if attempt == cfg.MaxAttempts-1 {
			break
		}

		delay := calculateDelay(cfg, attempt)

		if onRetry != nil {
			onRetry(attempt+1, err, delay)
		}

		if cfg.Logger != nil {
			cfg.Logger.Warn("operation failed, retrying",
				"error", err, "attempt", attempt+1, "delay", delay)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	if onFinalError != nil {
		onFinalError(cfg.MaxAttempts, lastErr)
	}

	if cfg.Logger != nil {
		cfg.Logger.Error("all retry attempts failed",
			"attempts", cfg.MaxAttempts, "error", lastErr)
	}

	return lastErr
}

func calculateDelay(cfg Config, attempt int) time.Duration {
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.BackoffFactor, float64(attempt))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	if cfg.Jitter {
		// #nosec G404 - math/rand is appropriate for retry jitter
		jitter := rand.Float64() * 0.1 * delay
		delay += jitter
	}
	return time.Duration(delay)
}

// ExponentialBackoff wraps retry logic in a reusable strategy.
type ExponentialBackoff struct {
	config Config
}

// NewExponentialBackoff creates a new exponential backoff strategy.
func NewExponentialBackoff(cfg Config) *ExponentialBackoff {
	return &ExponentialBackoff{config: cfg}
}

// Execute runs fn with exponential backoff.
func (eb *ExponentialBackoff) Execute(ctx context.Context, fn Func) error {
	return Do(ctx, eb.config, fn)
}

// ExecuteWithCallbacks runs fn with exponential backoff and callbacks.
func (eb *ExponentialBackoff) ExecuteWithCallbacks(
	ctx context.Context,
	fn CallbackFunc,
	onRetry func(attempt int, err error, delay time.Duration),
	onFinalError func(attempt int, err error),
) error {
	return DoWithCallbacks(ctx, eb.config, fn, onRetry, onFinalError)
}
