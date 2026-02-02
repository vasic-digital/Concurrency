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
