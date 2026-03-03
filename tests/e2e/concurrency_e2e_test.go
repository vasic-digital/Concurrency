package e2e

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

func TestE2E_WorkerPoolLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	var processedCount int64
	onComplete := func(r pool.Result) {
		if r.Error == nil {
			atomic.AddInt64(&processedCount, 1)
		}
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:       3,
		QueueSize:     50,
		TaskTimeout:   2 * time.Second,
		ShutdownGrace: 3 * time.Second,
		OnComplete:    onComplete,
	})
	p.Start()
	assert.True(t, p.IsRunning())

	for i := 0; i < 20; i++ {
		err := p.Submit(pool.NewTaskFunc(
			fmt.Sprintf("e2e-task-%d", i),
			func(ctx context.Context) (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return "done", nil
			},
		))
		require.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, p.WaitForDrain(ctx))
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int64(20), atomic.LoadInt64(&processedCount))

	err := p.Shutdown(5 * time.Second)
	require.NoError(t, err)
	assert.False(t, p.IsRunning())
}

func TestE2E_ParallelExecute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	fns := make([]func(ctx context.Context) (interface{}, error), 5)
	for i := 0; i < 5; i++ {
		idx := i
		fns[i] = func(ctx context.Context) (interface{}, error) {
			return fmt.Sprintf("result-%d", idx), nil
		}
	}

	results, err := pool.ParallelExecute(context.Background(), fns)
	require.NoError(t, err)
	assert.Len(t, results, 5)

	for _, r := range results {
		assert.NoError(t, r.Error)
		assert.NotNil(t, r.Value)
	}
}

func TestE2E_CircuitBreakerRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	cb := breaker.New(&breaker.Config{
		MaxFailures:      2,
		Timeout:          300 * time.Millisecond,
		HalfOpenRequests: 1,
	})

	var callCount int64

	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			atomic.AddInt64(&callCount, 1)
			return fmt.Errorf("service unavailable")
		})
	}

	assert.Equal(t, breaker.Open, cb.State())
	assert.Equal(t, int64(2), callCount)

	err := cb.Execute(func() error {
		atomic.AddInt64(&callCount, 1)
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, int64(2), callCount)

	time.Sleep(400 * time.Millisecond)
	assert.Equal(t, breaker.HalfOpen, cb.State())

	err = cb.Execute(func() error {
		atomic.AddInt64(&callCount, 1)
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, breaker.Closed, cb.State())
	assert.Equal(t, int64(3), callCount)
}

func TestE2E_SemaphoreResourceControl(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	sem := semaphore.New(2)
	var active int64
	var maxActive int64

	ctx := context.Background()
	done := make(chan struct{}, 10)

	for i := 0; i < 10; i++ {
		go func() {
			require.NoError(t, sem.Acquire(ctx, 1))
			curr := atomic.AddInt64(&active, 1)
			for {
				old := atomic.LoadInt64(&maxActive)
				if curr <= old || atomic.CompareAndSwapInt64(&maxActive, old, curr) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt64(&active, -1)
			sem.Release(1)
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.True(t, maxActive <= 2,
		"max active should never exceed semaphore weight, got %d", maxActive)
}

func TestE2E_PriorityQueueTaskScheduling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	pq := queue.New[string](0)

	pq.Push("background-cleanup", queue.Low)
	pq.Push("user-request-1", queue.Normal)
	pq.Push("security-alert", queue.Critical)
	pq.Push("user-request-2", queue.Normal)
	pq.Push("system-update", queue.High)

	expected := []string{
		"security-alert",
		"system-update",
		"user-request-1",
		"user-request-2",
		"background-cleanup",
	}

	for _, exp := range expected {
		val, ok := pq.Pop()
		require.True(t, ok)
		assert.Equal(t, exp, val)
	}
}

func TestE2E_RateLimiterWait(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	rl := limiter.NewTokenBucket(&limiter.TokenBucketConfig{
		Rate:     100,
		Capacity: 2,
	})

	ctx := context.Background()

	assert.True(t, rl.Allow(ctx))
	assert.True(t, rl.Allow(ctx))
	assert.False(t, rl.Allow(ctx))

	waitCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	start := time.Now()
	err := rl.Wait(waitCtx)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.True(t, duration > 5*time.Millisecond,
		"Wait should have blocked for a bit")
}

func TestE2E_WorkerPoolPanicRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	defer p.Stop()

	result, err := p.SubmitWait(context.Background(),
		pool.NewTaskFunc("panic-task", func(ctx context.Context) (interface{}, error) {
			panic("intentional panic")
		}),
	)

	assert.Error(t, err)
	assert.Contains(t, result.Error.Error(), "panicked")

	metrics := p.Metrics()
	assert.Equal(t, int64(1), metrics.PanickedTasks)
}
