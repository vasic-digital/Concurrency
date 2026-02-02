package limiter

import (
	"context"
	"sync"
	"time"
)

// RateLimiter is the common interface for rate limiting strategies.
type RateLimiter interface {
	// Allow reports whether a single event may happen now.
	// It does not block.
	Allow(ctx context.Context) bool

	// Wait blocks until a single event is allowed or the context
	// is cancelled.
	Wait(ctx context.Context) error
}

// ---------- Token Bucket ----------

// TokenBucketConfig configures a token bucket rate limiter.
type TokenBucketConfig struct {
	Rate     float64 // Tokens added per second
	Capacity int     // Maximum burst size (bucket capacity)
}

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	rate     float64
	capacity int
	tokens   float64
	lastTime time.Time
	mu       sync.Mutex
}

// NewTokenBucket creates a new token bucket rate limiter.
// The bucket starts full.
func NewTokenBucket(cfg *TokenBucketConfig) *TokenBucket {
	if cfg == nil {
		cfg = &TokenBucketConfig{Rate: 10, Capacity: 10}
	}
	return &TokenBucket{
		rate:     cfg.Rate,
		capacity: cfg.Capacity,
		tokens:   float64(cfg.Capacity),
		lastTime: time.Now(),
	}
}

// refill adds tokens based on elapsed time. Must be called with
// the mutex held.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.capacity) {
		tb.tokens = float64(tb.capacity)
	}
	tb.lastTime = now
}

// Allow reports whether one token can be consumed right now.
func (tb *TokenBucket) Allow(ctx context.Context) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

// Wait blocks until one token is available or the context is
// cancelled.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	for {
		if tb.Allow(ctx) {
			return nil
		}

		// Calculate wait time for next token
		tb.mu.Lock()
		var waitDur time.Duration
		if tb.rate > 0 {
			deficit := 1.0 - tb.tokens
			if deficit < 0 {
				deficit = 0
			}
			waitDur = time.Duration(
				deficit / tb.rate * float64(time.Second),
			)
			if waitDur < time.Millisecond {
				waitDur = time.Millisecond
			}
		} else {
			waitDur = 100 * time.Millisecond
		}
		tb.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
			// retry
		}
	}
}

// ---------- Sliding Window ----------

// SlidingWindowConfig configures a sliding window rate limiter.
type SlidingWindowConfig struct {
	WindowSize  time.Duration // Duration of the sliding window
	MaxRequests int           // Maximum requests per window
}

// SlidingWindow implements a sliding window rate limiter.
type SlidingWindow struct {
	windowSize  time.Duration
	maxRequests int
	timestamps  []time.Time
	mu          sync.Mutex
}

// NewSlidingWindow creates a new sliding window rate limiter.
func NewSlidingWindow(cfg *SlidingWindowConfig) *SlidingWindow {
	if cfg == nil {
		cfg = &SlidingWindowConfig{
			WindowSize:  time.Second,
			MaxRequests: 100,
		}
	}
	return &SlidingWindow{
		windowSize:  cfg.WindowSize,
		maxRequests: cfg.MaxRequests,
		timestamps:  make([]time.Time, 0, cfg.MaxRequests),
	}
}

// cleanup removes timestamps outside the window. Must be called
// with the mutex held.
func (sw *SlidingWindow) cleanup() {
	cutoff := time.Now().Add(-sw.windowSize)
	i := 0
	for i < len(sw.timestamps) && sw.timestamps[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		sw.timestamps = sw.timestamps[i:]
	}
}

// Allow reports whether a single request is allowed right now.
func (sw *SlidingWindow) Allow(ctx context.Context) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.cleanup()
	if len(sw.timestamps) < sw.maxRequests {
		sw.timestamps = append(sw.timestamps, time.Now())
		return true
	}
	return false
}

// Wait blocks until a request is allowed or the context is
// cancelled.
func (sw *SlidingWindow) Wait(ctx context.Context) error {
	for {
		if sw.Allow(ctx) {
			return nil
		}

		// Wait a fraction of the window before retrying
		waitDur := sw.windowSize / 10
		if waitDur < time.Millisecond {
			waitDur = time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
			// retry
		}
	}
}
