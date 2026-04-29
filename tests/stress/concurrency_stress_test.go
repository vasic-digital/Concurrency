package stress

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.concurrency/pkg/breaker"
	"digital.vasic.concurrency/pkg/pool"
	"digital.vasic.concurrency/pkg/queue"
	"digital.vasic.concurrency/pkg/semaphore"
)

// Resource limit: GOMAXPROCS=2 recommended for stress tests

func TestStress_WorkerPoolHighThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:       8,
		QueueSize:     1000,
		TaskTimeout:   5 * time.Second,
		ShutdownGrace: 5 * time.Second,
	})
	defer p.Stop()

	const taskCount = 100
	var completed int64

	tasks := make([]pool.Task, taskCount)
	for i := 0; i < taskCount; i++ {
		tasks[i] = pool.NewTaskFunc(
			fmt.Sprintf("stress-%d", i),
			func(ctx context.Context) (interface{}, error) {
				atomic.AddInt64(&completed, 1)
				return nil, nil
			},
		)
	}

	results, err := p.SubmitBatchWait(context.Background(), tasks)
	require.NoError(t, err)
	assert.Len(t, results, taskCount)
	assert.Equal(t, int64(taskCount), atomic.LoadInt64(&completed))
}

func TestStress_ConcurrentPriorityQueue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	pq := queue.New[int](0)

	const goroutines = 100
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			priority := queue.Priority(id % 4)
			pq.Push(id, priority)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, goroutines, pq.Len())

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = pq.Pop()
		}()
	}
	wg.Wait()

	assert.True(t, pq.IsEmpty())
}

func TestStress_ConcurrentSemaphore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	sem := semaphore.New(5)

	const goroutines = 100
	var wg sync.WaitGroup
	var maxConcurrent int64

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ctx := context.Background()
			require.NoError(t, sem.Acquire(ctx, 1))

			curr := sem.Current()
			for {
				old := atomic.LoadInt64(&maxConcurrent)
				if curr <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, curr) {
					break
				}
			}

			time.Sleep(time.Millisecond)
			sem.Release(1)
		}()
	}

	wg.Wait()
	assert.True(t, maxConcurrent <= 5,
		"max concurrent should not exceed semaphore limit, got %d", maxConcurrent)
	assert.Equal(t, int64(0), sem.Current())
}

func TestStress_ConcurrentCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	cb := breaker.New(&breaker.Config{
		MaxFailures:      100,
		Timeout:          10 * time.Second,
		HalfOpenRequests: 1,
	})

	const goroutines = 80
	var wg sync.WaitGroup
	var successes int64

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			err := cb.Execute(func() error {
				if id%3 == 0 {
					return fmt.Errorf("failure")
				}
				return nil
			})
			if err == nil {
				atomic.AddInt64(&successes, 1)
			}
		}(i)
	}

	wg.Wait()
	assert.True(t, successes > 0,
		"some requests should succeed")
}

func TestStress_WorkerPoolConcurrentSubmit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   4,
		QueueSize: 500,
	})
	p.Start()
	defer p.Stop()

	const goroutines = 50
	var wg sync.WaitGroup
	var submitted int64

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			err := p.Submit(pool.NewTaskFunc(
				fmt.Sprintf("concurrent-%d", id),
				func(ctx context.Context) (interface{}, error) {
					return id, nil
				},
			))
			if err == nil {
				atomic.AddInt64(&submitted, 1)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int64(goroutines), atomic.LoadInt64(&submitted))
}

func TestStress_PriorityQueueMixedOperations(t *testing.T) {
	// bluff-scan: no-assert-ok (stress test — high-volume calls must not panic; go test -race verifies)
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	pq := queue.New[string](0)

	const goroutines = 50
	var wg sync.WaitGroup

	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			pq.Push(fmt.Sprintf("item-%d", id), queue.Normal)
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = pq.Len()
			_ = pq.IsEmpty()
			_, _ = pq.Peek()
			_, _ = pq.Pop()
		}()
	}

	wg.Wait()
}
