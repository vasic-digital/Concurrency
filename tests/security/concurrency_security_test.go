package security

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.concurrency/pkg/breaker"
	"digital.vasic.concurrency/pkg/pool"
	"digital.vasic.concurrency/pkg/queue"
	"digital.vasic.concurrency/pkg/semaphore"
)

func TestSecurity_WorkerPoolPanicIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   2,
		QueueSize: 20,
	})
	defer p.Stop()

	result, err := p.SubmitWait(context.Background(),
		pool.NewTaskFunc("panic", func(ctx context.Context) (interface{}, error) {
			panic("simulated crash")
		}),
	)
	assert.Error(t, err)
	assert.Contains(t, result.Error.Error(), "panicked")

	result2, err2 := p.SubmitWait(context.Background(),
		pool.NewTaskFunc("after-panic", func(ctx context.Context) (interface{}, error) {
			return "recovered", nil
		}),
	)
	require.NoError(t, err2)
	assert.Equal(t, "recovered", result2.Value)
}

func TestSecurity_WorkerPoolClosedSubmit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   1,
		QueueSize: 5,
	})
	p.Start()
	_ = p.Shutdown(time.Second)

	err := p.Submit(pool.NewTaskFunc("late", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestSecurity_SemaphoreExcessWeight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	sem := semaphore.New(5)

	err := sem.Acquire(context.Background(), 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max weight")

	assert.False(t, sem.TryAcquire(10))

	assert.Equal(t, int64(0), sem.Current())
}

func TestSecurity_SemaphoreContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	sem := semaphore.New(1)
	require.NoError(t, sem.Acquire(context.Background(), 1))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := sem.Acquire(ctx, 1)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestSecurity_CircuitBreakerReset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	cb := breaker.New(&breaker.Config{
		MaxFailures:      2,
		Timeout:          10 * time.Second,
		HalfOpenRequests: 1,
	})

	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error { return fmt.Errorf("fail") })
	}
	assert.Equal(t, breaker.Open, cb.State())

	cb.Reset()
	assert.Equal(t, breaker.Closed, cb.State())
	assert.Equal(t, 0, cb.Failures())

	err := cb.Execute(func() error { return nil })
	assert.NoError(t, err)
}

func TestSecurity_PriorityQueueEmptyPop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	pq := queue.New[int](0)

	val, ok := pq.Pop()
	assert.False(t, ok)
	assert.Equal(t, 0, val)

	val, ok = pq.Peek()
	assert.False(t, ok)
	assert.Equal(t, 0, val)

	assert.True(t, pq.IsEmpty())
}

func TestSecurity_WorkerPoolResizeInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	p.Start()
	defer p.Stop()

	err := p.Resize(0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")

	err = p.Resize(-1)
	assert.Error(t, err)
}

func TestSecurity_SemaphoreZeroWeight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	sem := semaphore.New(3)

	err := sem.Acquire(context.Background(), 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), sem.Current())

	assert.True(t, sem.TryAcquire(0))

	sem.Release(0)
	assert.Equal(t, int64(0), sem.Current())
}
