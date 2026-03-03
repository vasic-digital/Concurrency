package integration

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.concurrency/pkg/breaker"
	"digital.vasic.concurrency/pkg/limiter"
	"digital.vasic.concurrency/pkg/pool"
	"digital.vasic.concurrency/pkg/queue"
	"digital.vasic.concurrency/pkg/semaphore"
)

func TestWorkerPoolSubmitAndCollect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:       4,
		QueueSize:     100,
		TaskTimeout:   5 * time.Second,
		ShutdownGrace: 2 * time.Second,
	})
	defer p.Stop()

	var completed int64
	tasks := make([]pool.Task, 10)
	for i := 0; i < 10; i++ {
		idx := i
		tasks[i] = pool.NewTaskFunc(
			fmt.Sprintf("task-%d", idx),
			func(ctx context.Context) (interface{}, error) {
				atomic.AddInt64(&completed, 1)
				return idx * 2, nil
			},
		)
	}

	results, err := p.SubmitBatchWait(context.Background(), tasks)
	require.NoError(t, err)
	assert.Len(t, results, 10)
	assert.Equal(t, int64(10), atomic.LoadInt64(&completed))

	for _, r := range results {
		assert.NoError(t, r.Error)
		assert.True(t, r.Duration > 0)
	}
}

func TestWorkerPoolMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   2,
		QueueSize: 50,
	})
	p.Start()

	for i := 0; i < 5; i++ {
		_ = p.Submit(pool.NewTaskFunc(
			fmt.Sprintf("ok-%d", i),
			func(ctx context.Context) (interface{}, error) {
				return "ok", nil
			},
		))
	}

	for i := 0; i < 3; i++ {
		_ = p.Submit(pool.NewTaskFunc(
			fmt.Sprintf("fail-%d", i),
			func(ctx context.Context) (interface{}, error) {
				return nil, fmt.Errorf("task failed")
			},
		))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = p.WaitForDrain(ctx)
	time.Sleep(200 * time.Millisecond)

	metrics := p.Metrics()
	assert.Equal(t, int64(5), metrics.CompletedTasks)
	assert.Equal(t, int64(3), metrics.FailedTasks)
	assert.True(t, metrics.AverageLatency() > 0)

	_ = p.Shutdown(2 * time.Second)
}

func TestCircuitBreakerStateTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cb := breaker.New(&breaker.Config{
		MaxFailures:      3,
		Timeout:          200 * time.Millisecond,
		HalfOpenRequests: 1,
	})

	assert.Equal(t, breaker.Closed, cb.State())

	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return fmt.Errorf("failure %d", i)
		})
		assert.Error(t, err)
	}

	assert.Equal(t, breaker.Open, cb.State())

	err := cb.Execute(func() error { return nil })
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")

	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, breaker.HalfOpen, cb.State())

	err = cb.Execute(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, breaker.Closed, cb.State())
}

func TestPriorityQueueOrdering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pq := queue.New[string](10)

	pq.Push("low-task", queue.Low)
	pq.Push("critical-task", queue.Critical)
	pq.Push("normal-task", queue.Normal)
	pq.Push("high-task", queue.High)

	assert.Equal(t, 4, pq.Len())

	val, ok := pq.Pop()
	require.True(t, ok)
	assert.Equal(t, "critical-task", val)

	val, ok = pq.Pop()
	require.True(t, ok)
	assert.Equal(t, "high-task", val)

	val, ok = pq.Pop()
	require.True(t, ok)
	assert.Equal(t, "normal-task", val)

	val, ok = pq.Pop()
	require.True(t, ok)
	assert.Equal(t, "low-task", val)

	assert.True(t, pq.IsEmpty())
}

func TestSemaphoreAcquireRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sem := semaphore.New(3)

	ctx := context.Background()
	require.NoError(t, sem.Acquire(ctx, 2))
	assert.Equal(t, int64(2), sem.Current())
	assert.Equal(t, int64(1), sem.Available())

	assert.True(t, sem.TryAcquire(1))
	assert.Equal(t, int64(3), sem.Current())

	assert.False(t, sem.TryAcquire(1))

	sem.Release(2)
	assert.Equal(t, int64(1), sem.Current())

	require.NoError(t, sem.Acquire(ctx, 2))
	assert.Equal(t, int64(3), sem.Current())

	sem.Release(3)
	assert.Equal(t, int64(0), sem.Current())
}

func TestTokenBucketRateLimiter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	rl := limiter.NewTokenBucket(&limiter.TokenBucketConfig{
		Rate:     10,
		Capacity: 5,
	})

	ctx := context.Background()
	allowed := 0
	for i := 0; i < 10; i++ {
		if rl.Allow(ctx) {
			allowed++
		}
	}
	assert.Equal(t, 5, allowed,
		"should allow exactly capacity tokens initially")

	time.Sleep(200 * time.Millisecond)
	assert.True(t, rl.Allow(ctx),
		"should allow after refill")
}

func TestSlidingWindowRateLimiter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	rl := limiter.NewSlidingWindow(&limiter.SlidingWindowConfig{
		WindowSize:  200 * time.Millisecond,
		MaxRequests: 3,
	})

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		assert.True(t, rl.Allow(ctx))
	}
	assert.False(t, rl.Allow(ctx))

	time.Sleep(250 * time.Millisecond)
	assert.True(t, rl.Allow(ctx),
		"should allow after window expires")
}

func TestWorkerPoolResize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   2,
		QueueSize: 100,
	})
	p.Start()
	defer p.Stop()

	assert.Equal(t, 2, p.WorkerCount())

	require.NoError(t, p.Resize(4))
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 4, p.WorkerCount())

	require.NoError(t, p.Resize(1))
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 1, p.WorkerCount())
}
