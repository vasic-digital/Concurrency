package pool

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkerPool_DefaultConfig(t *testing.T) {
	wp := NewWorkerPool(nil)
	require.NotNil(t, wp)
	assert.True(t, wp.config.Workers > 0)
	assert.Equal(t, 1000, wp.config.QueueSize)
	wp.Stop()
}

func TestNewWorkerPool_CustomConfig(t *testing.T) {
	cfg := &PoolConfig{
		Workers:   4,
		QueueSize: 50,
	}
	wp := NewWorkerPool(cfg)
	require.NotNil(t, wp)
	assert.Equal(t, 4, wp.config.Workers)
	assert.Equal(t, 50, wp.config.QueueSize)
	wp.Stop()
}

func TestWorkerPool_Submit_Basic(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	wp.Start()

	err := wp.Submit(NewTaskFunc("t1", func(ctx context.Context) (interface{}, error) {
		return "hello", nil
	}))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = wp.WaitForDrain(ctx)
	require.NoError(t, err)

	wp.Stop()
}

func TestWorkerPool_Submit_AutoStart(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	// Do not call Start() - Submit should auto-start

	err := wp.Submit(NewTaskFunc("t1", func(ctx context.Context) (interface{}, error) {
		return 42, nil
	}))
	require.NoError(t, err)
	assert.True(t, wp.IsRunning())

	wp.Stop()
}

func TestWorkerPool_Submit_Closed(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	wp.Stop()

	err := wp.Submit(NewTaskFunc("t1", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestWorkerPool_SubmitWait(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	wp.Start()
	defer wp.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	task := NewTaskFunc("wait-task", func(ctx context.Context) (interface{}, error) {
		return "result-value", nil
	})

	result, err := wp.SubmitWait(ctx, task)
	require.NoError(t, err)
	assert.Equal(t, "wait-task", result.TaskID)
	assert.Equal(t, "result-value", result.Value)
}

func TestWorkerPool_SubmitBatchWait(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:   4,
		QueueSize: 20,
	})
	wp.Start()
	defer wp.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tasks := make([]Task, 5)
	for i := 0; i < 5; i++ {
		idx := i
		tasks[i] = NewTaskFunc(
			fmt.Sprintf("batch-%d", i),
			func(ctx context.Context) (interface{}, error) {
				return idx * 10, nil
			},
		)
	}

	results, err := wp.SubmitBatchWait(ctx, tasks)
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestWorkerPool_Metrics(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	wp.Start()

	for i := 0; i < 3; i++ {
		_ = wp.Submit(NewTaskFunc(
			fmt.Sprintf("m-%d", i),
			func(ctx context.Context) (interface{}, error) {
				return nil, nil
			},
		))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = wp.WaitForDrain(ctx)

	// Give a small window for metrics to update
	time.Sleep(50 * time.Millisecond)

	m := wp.Metrics()
	assert.Equal(t, int64(3), m.CompletedTasks)
	assert.Equal(t, int64(0), m.FailedTasks)

	wp.Stop()
}

func TestWorkerPool_Metrics_WithFailures(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	wp.Start()

	_ = wp.Submit(NewTaskFunc("fail-1", func(ctx context.Context) (interface{}, error) {
		return nil, fmt.Errorf("intentional error")
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = wp.WaitForDrain(ctx)
	time.Sleep(50 * time.Millisecond)

	m := wp.Metrics()
	assert.Equal(t, int64(1), m.FailedTasks)

	wp.Stop()
}

func TestWorkerPool_Callbacks(t *testing.T) {
	var errorCount int64
	var completeCount int64

	wp := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
		OnError: func(taskID string, err error) {
			atomic.AddInt64(&errorCount, 1)
		},
		OnComplete: func(result Result) {
			atomic.AddInt64(&completeCount, 1)
		},
	})
	wp.Start()

	_ = wp.Submit(NewTaskFunc("ok", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))
	_ = wp.Submit(NewTaskFunc("fail", func(ctx context.Context) (interface{}, error) {
		return nil, fmt.Errorf("error")
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = wp.WaitForDrain(ctx)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int64(1), atomic.LoadInt64(&errorCount))
	assert.Equal(t, int64(2), atomic.LoadInt64(&completeCount))

	wp.Stop()
}

func TestWorkerPool_IsRunning(t *testing.T) {
	tests := []struct {
		name     string
		action   func(*WorkerPool)
		expected bool
	}{
		{
			name:     "before start",
			action:   func(wp *WorkerPool) {},
			expected: false,
		},
		{
			name:     "after start",
			action:   func(wp *WorkerPool) { wp.Start() },
			expected: true,
		},
		{
			name:     "after stop",
			action:   func(wp *WorkerPool) { wp.Start(); wp.Stop() },
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp := NewWorkerPool(&PoolConfig{
				Workers:   1,
				QueueSize: 5,
			})
			tt.action(wp)
			assert.Equal(t, tt.expected, wp.IsRunning())
			if wp.IsRunning() {
				wp.Stop()
			}
		})
	}
}

func TestWorkerPool_Shutdown_Graceful(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:       2,
		QueueSize:     10,
		ShutdownGrace: 5 * time.Second,
	})
	wp.Start()

	_ = wp.Submit(NewTaskFunc("slow", func(ctx context.Context) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "done", nil
	}))

	err := wp.Shutdown(5 * time.Second)
	require.NoError(t, err)
	assert.False(t, wp.IsRunning())
}

func TestWorkerPool_TaskTimeout(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 100 * time.Millisecond,
	})
	wp.Start()
	defer wp.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	task := NewTaskFunc("timeout-task", func(ctx context.Context) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return "should not reach", nil
		}
	})

	result, _ := wp.SubmitWait(ctx, task)
	assert.Error(t, result.Error)
}

func TestWorkerPool_QueueLength(t *testing.T) {
	wp := NewWorkerPool(&PoolConfig{
		Workers:   1,
		QueueSize: 100,
	})
	// Don't start - tasks will queue up
	assert.Equal(t, 0, wp.QueueLength())
	wp.Stop()
}

func TestPoolMetrics_AverageLatency(t *testing.T) {
	tests := []struct {
		name     string
		metrics  PoolMetrics
		expected time.Duration
	}{
		{
			name:     "no tasks",
			metrics:  PoolMetrics{TaskCount: 0, TotalLatencyUs: 0},
			expected: 0,
		},
		{
			name:     "single task 1000us",
			metrics:  PoolMetrics{TaskCount: 1, TotalLatencyUs: 1000},
			expected: 1000 * time.Microsecond,
		},
		{
			name:     "two tasks averaging 500us",
			metrics:  PoolMetrics{TaskCount: 2, TotalLatencyUs: 1000},
			expected: 500 * time.Microsecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.metrics.AverageLatency())
		})
	}
}

func TestTaskFunc_IDAndExecute(t *testing.T) {
	tf := NewTaskFunc("my-id", func(ctx context.Context) (interface{}, error) {
		return 42, nil
	})
	assert.Equal(t, "my-id", tf.ID())

	val, err := tf.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 42, val)
}

func TestParallelExecute(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fns := []func(ctx context.Context) (interface{}, error){
		func(ctx context.Context) (interface{}, error) { return 1, nil },
		func(ctx context.Context) (interface{}, error) { return 2, nil },
		func(ctx context.Context) (interface{}, error) { return 3, nil },
	}

	results, err := ParallelExecute(ctx, fns)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestMap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	items := []int{1, 2, 3, 4, 5}
	results, err := Map(ctx, items, 3, func(ctx context.Context, n int) (int, error) {
		return n * 2, nil
	})
	require.NoError(t, err)

	// Results may be in any order due to concurrency, so collect and sort
	assert.Len(t, results, 5)
	sum := 0
	for _, v := range results {
		sum += v
	}
	assert.Equal(t, 30, sum) // 2+4+6+8+10 = 30
}

func TestMap_WithError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	items := []int{1, 2, 3}
	_, err := Map(ctx, items, 2, func(ctx context.Context, n int) (int, error) {
		if n == 2 {
			return 0, fmt.Errorf("error on item 2")
		}
		return n, nil
	})
	require.Error(t, err)
}
