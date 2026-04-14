package pool_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Zero-Size Pool ---

func TestWorkerPool_ZeroSizePool(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   0,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)
	require.NotNil(t, wp)

	// With 0 workers, tasks submitted should not be processed.
	// The pool should still be creatable without panicking.
	wp.Stop()
}

func TestWorkerPool_ZeroQueueSize(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   2,
		QueueSize: 0,
	}
	wp := pool.NewWorkerPool(cfg)
	require.NotNil(t, wp)
	wp.Start()

	// Queue size 0 means unbuffered channel -- Submit should block/fail
	// immediately if no worker is ready to receive.
	err := wp.Submit(pool.NewTaskFunc("t1", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))
	// May succeed (worker picks up) or fail (queue full), either is acceptable
	_ = err

	wp.Stop()
}

// --- Pool Overflow ---

func TestWorkerPool_QueueOverflow(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:     1,
		QueueSize:   2,
		TaskTimeout: time.Second,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	defer wp.Stop()

	// Block the single worker with a long task
	blockCh := make(chan struct{})
	_ = wp.Submit(pool.NewTaskFunc("blocker", func(ctx context.Context) (interface{}, error) {
		<-blockCh
		return nil, nil
	}))

	// Fill the queue
	_ = wp.Submit(pool.NewTaskFunc("q1", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))
	_ = wp.Submit(pool.NewTaskFunc("q2", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))

	// This should fail with "task queue is full"
	err := wp.Submit(pool.NewTaskFunc("overflow", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue is full")

	close(blockCh)
}

// --- Panic in Worker ---

func TestWorkerPool_PanicInWorker(t *testing.T) {
	t.Parallel()

	var panicErrors []string
	var mu sync.Mutex

	cfg := &pool.PoolConfig{
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 5 * time.Second,
		OnError: func(taskID string, err error) {
			mu.Lock()
			panicErrors = append(panicErrors, err.Error())
			mu.Unlock()
		},
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()

	// Submit a panicking task
	err := wp.Submit(pool.NewTaskFunc("panic-task", func(ctx context.Context) (interface{}, error) {
		panic("test panic in worker")
	}))
	require.NoError(t, err)

	// Submit a normal task after the panic to verify pool is still alive
	err = wp.Submit(pool.NewTaskFunc("normal-task", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = wp.WaitForDrain(ctx)

	wp.Stop()

	metrics := wp.Metrics()
	assert.Equal(t, int64(1), metrics.PanickedTasks, "expected exactly one panicked task")
	assert.True(t, metrics.CompletedTasks >= 1, "normal task should have completed")
}

// --- Context Cancel Mid-Task ---

func TestWorkerPool_ContextCancelMidTask(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 10 * time.Second,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	defer wp.Stop()

	taskStarted := make(chan struct{})
	err := wp.Submit(pool.NewTaskFunc("cancel-task", func(ctx context.Context) (interface{}, error) {
		close(taskStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	require.NoError(t, err)

	<-taskStarted
	// Stop the pool which cancels the context
	wp.Stop()

	metrics := wp.Metrics()
	assert.True(t, metrics.TaskCount >= 1)
}

// --- Nil Function ---

func TestWorkerPool_NilFunction(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:     1,
		QueueSize:   10,
		TaskTimeout: 2 * time.Second,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()

	// A nil function inside TaskFunc will panic, which should be recovered
	err := wp.Submit(pool.NewTaskFunc("nil-fn", func(ctx context.Context) (interface{}, error) {
		var fn func()
		fn() // nil function call -> panic
		return nil, nil
	}))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = wp.WaitForDrain(ctx)

	wp.Stop()

	metrics := wp.Metrics()
	assert.Equal(t, int64(1), metrics.PanickedTasks)
}

// --- Double Start / Stop ---

func TestWorkerPool_DoubleStart(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)

	// Double start should not panic or create extra workers
	wp.Start()
	wp.Start()

	assert.True(t, wp.IsRunning())
	assert.Equal(t, 2, wp.WorkerCount())

	wp.Stop()
}

func TestWorkerPool_DoubleStop(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()

	// Double stop should not panic
	wp.Stop()
	wp.Stop()

	assert.False(t, wp.IsRunning())
}

func TestWorkerPool_StopWithoutStart(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)

	// Stop without Start should not panic
	wp.Stop()
	assert.False(t, wp.IsRunning())
}

// --- Submit After Shutdown ---

func TestWorkerPool_SubmitAfterShutdown(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	wp.Stop()

	err := wp.Submit(pool.NewTaskFunc("post-close", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// --- Resize Edge Cases ---

func TestWorkerPool_Resize_ZeroWorkers(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	defer wp.Stop()

	err := wp.Resize(0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestWorkerPool_Resize_NegativeWorkers(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	defer wp.Stop()

	err := wp.Resize(-5)
	assert.Error(t, err)
}

func TestWorkerPool_Resize_NotRunning(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)

	err := wp.Resize(4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

// --- SubmitWait With Cancelled Context ---

func TestWorkerPool_SubmitWait_CancelledContext(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   1,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	defer wp.Stop()

	// Block the worker
	blockCh := make(chan struct{})
	_ = wp.Submit(pool.NewTaskFunc("blocker", func(ctx context.Context) (interface{}, error) {
		<-blockCh
		return "blocked", nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := wp.SubmitWait(ctx, pool.NewTaskFunc("wait-task", func(ctx context.Context) (interface{}, error) {
		return "result", nil
	}))
	assert.Error(t, err)

	close(blockCh)
}

// --- Metrics on Empty Pool ---

func TestWorkerPool_Metrics_EmptyPool(t *testing.T) {
	t.Parallel()

	wp := pool.NewWorkerPool(nil)
	metrics := wp.Metrics()

	assert.Equal(t, int64(0), metrics.ActiveWorkers)
	assert.Equal(t, int64(0), metrics.CompletedTasks)
	assert.Equal(t, int64(0), metrics.FailedTasks)
	assert.Equal(t, int64(0), metrics.PanickedTasks)
	assert.Equal(t, time.Duration(0), metrics.AverageLatency())

	wp.Stop()
}

// --- Concurrent Submit Stress ---

func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:     4,
		QueueSize:   100,
		TaskTimeout: 5 * time.Second,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	defer wp.Stop()

	var completed int64
	var wg sync.WaitGroup
	n := 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := wp.Submit(pool.NewTaskFunc(
				fmt.Sprintf("concurrent-%d", idx),
				func(ctx context.Context) (interface{}, error) {
					atomic.AddInt64(&completed, 1)
					return idx, nil
				},
			))
			if err != nil {
				// Queue might be full, acceptable in stress test
				return
			}
		}(i)
	}
	wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = wp.WaitForDrain(ctx)

	assert.True(t, atomic.LoadInt64(&completed) > 0, "some tasks should have completed")
}

// --- Map With Empty Slice ---

func TestMap_EmptySlice(t *testing.T) {
	t.Parallel()

	result, err := pool.Map(
		context.Background(),
		[]int{},
		2,
		func(ctx context.Context, item int) (string, error) {
			return fmt.Sprintf("%d", item), nil
		},
	)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// --- ParallelExecute With Empty Functions ---

func TestParallelExecute_EmptyFunctions(t *testing.T) {
	t.Parallel()

	results, err := pool.ParallelExecute(
		context.Background(),
		[]func(ctx context.Context) (interface{}, error){},
	)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- WaitForDrain With Already-Cancelled Context ---

func TestWorkerPool_WaitForDrain_CancelledContext(t *testing.T) {
	t.Parallel()

	cfg := &pool.PoolConfig{
		Workers:   1,
		QueueSize: 10,
	}
	wp := pool.NewWorkerPool(cfg)
	wp.Start()
	defer wp.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := wp.WaitForDrain(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
