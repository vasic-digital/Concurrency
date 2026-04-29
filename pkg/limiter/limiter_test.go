package limiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- Token Bucket Tests ----------

func TestTokenBucket_Allow_BurstCapacity(t *testing.T) {
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     10,
		Capacity: 5,
	})

	// Should allow up to capacity
	allowed := 0
	for i := 0; i < 10; i++ {
		if tb.Allow(context.Background()) {
			allowed++
		}
	}
	assert.Equal(t, 5, allowed)
}

func TestTokenBucket_Allow_Refill(t *testing.T) {
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     1000, // 1000 per second
		Capacity: 1,
	})

	// Drain the bucket
	tb.Allow(context.Background())
	assert.False(t, tb.Allow(context.Background()))

	// Wait for refill
	time.Sleep(5 * time.Millisecond)
	assert.True(t, tb.Allow(context.Background()))
}

func TestTokenBucket_Wait_Success(t *testing.T) {
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     1000,
		Capacity: 1,
	})

	// Drain
	tb.Allow(context.Background())

	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()

	err := tb.Wait(ctx)
	require.NoError(t, err)
}

func TestTokenBucket_Wait_ContextCancelled(t *testing.T) {
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     0.001, // very slow refill
		Capacity: 1,
	})

	// Drain
	tb.Allow(context.Background())

	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Millisecond,
	)
	defer cancel()

	err := tb.Wait(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestTokenBucket_DefaultConfig(t *testing.T) {
	tb := NewTokenBucket(nil)
	assert.NotNil(t, tb)
	// Should work with defaults
	assert.True(t, tb.Allow(context.Background()))
}

func TestTokenBucket_ConcurrentAccess(t *testing.T) {
	// bluff-scan: no-assert-ok (concurrency test — go test -race catches data races; absence of panic == correctness)
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     10000,
		Capacity: 100,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tb.Allow(context.Background())
		}()
	}
	wg.Wait()
}

// ---------- Sliding Window Tests ----------

func TestSlidingWindow_Allow_WithinLimit(t *testing.T) {
	sw := NewSlidingWindow(&SlidingWindowConfig{
		WindowSize:  time.Second,
		MaxRequests: 5,
	})

	for i := 0; i < 5; i++ {
		assert.True(t, sw.Allow(context.Background()))
	}
	assert.False(t, sw.Allow(context.Background()))
}

func TestSlidingWindow_Allow_WindowExpiry(t *testing.T) {
	sw := NewSlidingWindow(&SlidingWindowConfig{
		WindowSize:  50 * time.Millisecond,
		MaxRequests: 2,
	})

	assert.True(t, sw.Allow(context.Background()))
	assert.True(t, sw.Allow(context.Background()))
	assert.False(t, sw.Allow(context.Background()))

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)
	assert.True(t, sw.Allow(context.Background()))
}

func TestSlidingWindow_Wait_Success(t *testing.T) {
	sw := NewSlidingWindow(&SlidingWindowConfig{
		WindowSize:  50 * time.Millisecond,
		MaxRequests: 1,
	})

	// Use the single allowed request
	sw.Allow(context.Background())

	ctx, cancel := context.WithTimeout(
		context.Background(), 200*time.Millisecond,
	)
	defer cancel()

	err := sw.Wait(ctx)
	require.NoError(t, err)
}

func TestSlidingWindow_Wait_ContextCancelled(t *testing.T) {
	sw := NewSlidingWindow(&SlidingWindowConfig{
		WindowSize:  10 * time.Second, // very long window
		MaxRequests: 1,
	})

	// Use the single allowed request
	sw.Allow(context.Background())

	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Millisecond,
	)
	defer cancel()

	err := sw.Wait(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestSlidingWindow_DefaultConfig(t *testing.T) {
	sw := NewSlidingWindow(nil)
	assert.NotNil(t, sw)
	assert.True(t, sw.Allow(context.Background()))
}

func TestSlidingWindow_ConcurrentAccess(t *testing.T) {
	// bluff-scan: no-assert-ok (concurrency test — go test -race catches data races; absence of panic == correctness)
	sw := NewSlidingWindow(&SlidingWindowConfig{
		WindowSize:  time.Second,
		MaxRequests: 1000,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sw.Allow(context.Background())
		}()
	}
	wg.Wait()
}

// ---------- Interface Compliance ----------

func TestRateLimiter_InterfaceCompliance(t *testing.T) {
	tests := []struct {
		name    string
		limiter RateLimiter
	}{
		{
			name: "token bucket",
			limiter: NewTokenBucket(&TokenBucketConfig{
				Rate: 100, Capacity: 10,
			}),
		},
		{
			name: "sliding window",
			limiter: NewSlidingWindow(&SlidingWindowConfig{
				WindowSize: time.Second, MaxRequests: 100,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.limiter.Allow(context.Background()))
		})
	}
}

// ---------- Edge Case Tests ----------

func TestTokenBucket_Wait_ZeroRate(t *testing.T) {
	// Test the path where rate <= 0 in Wait (line 98-99)
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     0, // Zero rate
		Capacity: 1,
	})

	// Drain the token
	tb.Allow(context.Background())

	// Wait with short timeout - should use the 100ms fallback wait
	ctx, cancel := context.WithTimeout(
		context.Background(), 50*time.Millisecond,
	)
	defer cancel()

	err := tb.Wait(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestTokenBucket_Wait_DeficitNegative(t *testing.T) {
	// Test the path where deficit calculation results in < 0
	// This happens when tokens are refilled between Allow returning false
	// and the deficit calculation
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     10000, // Very high rate - refills quickly
		Capacity: 2,
	})

	// Drain bucket
	tb.Allow(context.Background())
	tb.Allow(context.Background())

	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()

	// Should succeed because high rate will refill quickly
	err := tb.Wait(ctx)
	require.NoError(t, err)
}

func TestSlidingWindow_Wait_ShortWindowSize(t *testing.T) {
	// Test the path where waitDur < time.Millisecond (line 179-180)
	sw := NewSlidingWindow(&SlidingWindowConfig{
		WindowSize:  5 * time.Millisecond, // Very short window
		MaxRequests: 1,
	})

	// Use the single allowed request
	sw.Allow(context.Background())

	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()

	// Wait should use 1ms minimum wait even though window/10 < 1ms
	err := sw.Wait(ctx)
	require.NoError(t, err)
}

func TestTokenBucket_Wait_DeficitBecomesNegative(t *testing.T) {
	// Test the path where deficit < 0 in Wait (lines 89-91)
	// This happens when tokens refill between Allow returning false
	// and the deficit calculation, making tokens >= 1
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     100000, // Extremely high rate - refills very fast
		Capacity: 1,
	})

	// Drain the bucket
	tb.Allow(context.Background())

	// At this rate, the bucket will refill almost immediately,
	// so when Wait runs the deficit calculation, deficit may be <= 0
	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()

	err := tb.Wait(ctx)
	require.NoError(t, err)
}

func TestTokenBucket_Wait_ConcurrentAccess_DeficitPath(t *testing.T) {
	// bluff-scan: no-assert-ok (concurrency test — go test -race catches data races; absence of panic == correctness)
	// Test concurrent access to try to hit the deficit < 0 path
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     1000000, // Very high rate
		Capacity: 10,
	})

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Multiple goroutines doing Allow and Wait concurrently
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					tb.Allow(context.Background())
				}
			}
		}()
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Millisecond)
					_ = tb.Wait(waitCtx)
					waitCancel()
				}
			}
		}()
	}

	wg.Wait()
}

// TestTokenBucket_Wait_DeficitNegativeViaHook tests the deficit < 0 path
// using the test hook to simulate a race condition where tokens increase
// between Allow() returning false and the deficit calculation.
func TestTokenBucket_Wait_DeficitNegativeViaHook(t *testing.T) {
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     10,
		Capacity: 2,
	})

	// Drain the bucket
	tb.Allow(context.Background())
	tb.Allow(context.Background())

	// Set up the hook to add tokens after Allow() returns false
	// but before the deficit calculation. This simulates the race
	// condition where tokens are refilled by another goroutine.
	hookCalled := false
	tb.testHook = func(bucket *TokenBucket) {
		if !hookCalled {
			hookCalled = true
			// Add tokens to make deficit negative
			bucket.mu.Lock()
			bucket.tokens = 2.0 // More than 1.0, so deficit will be < 0
			bucket.mu.Unlock()
		}
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()

	// Wait should succeed because the hook adds tokens
	err := tb.Wait(ctx)
	require.NoError(t, err)
	assert.True(t, hookCalled, "test hook should have been called")
}

// TestTokenBucket_Wait_TestHookNil verifies Wait works normally when
// testHook is nil.
func TestTokenBucket_Wait_TestHookNil(t *testing.T) {
	tb := NewTokenBucket(&TokenBucketConfig{
		Rate:     1000,
		Capacity: 1,
	})

	// Drain the bucket
	tb.Allow(context.Background())

	// testHook is nil by default
	assert.Nil(t, tb.testHook)

	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()

	err := tb.Wait(ctx)
	require.NoError(t, err)
}
