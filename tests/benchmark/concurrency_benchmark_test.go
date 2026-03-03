package benchmark

import (
	"context"
	"fmt"
	"testing"

	"digital.vasic.concurrency/pkg/breaker"
	"digital.vasic.concurrency/pkg/pool"
	"digital.vasic.concurrency/pkg/queue"
	"digital.vasic.concurrency/pkg/semaphore"
)

func BenchmarkWorkerPoolSubmit(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   4,
		QueueSize: b.N + 100,
	})
	p.Start()
	defer p.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Submit(pool.NewTaskFunc(
			fmt.Sprintf("bench-%d", i),
			func(ctx context.Context) (interface{}, error) {
				return nil, nil
			},
		))
	}
}

func BenchmarkPriorityQueuePushPop(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	pq := queue.New[int](b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Push(i, queue.Priority(i%4))
	}
	for i := 0; i < b.N; i++ {
		pq.Pop()
	}
}

func BenchmarkCircuitBreakerExecute(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	cb := breaker.New(breaker.DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Execute(func() error { return nil })
	}
}

func BenchmarkSemaphoreAcquireRelease(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	sem := semaphore.New(int64(b.N + 1))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sem.Acquire(ctx, 1)
		sem.Release(1)
	}
}

func BenchmarkSemaphoreTryAcquire(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	sem := semaphore.New(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if sem.TryAcquire(1) {
			sem.Release(1)
		}
	}
}

func BenchmarkPriorityQueuePush(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	pq := queue.New[int](b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Push(i, queue.Normal)
	}
}

func BenchmarkCircuitBreakerState(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	cb := breaker.New(breaker.DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.State()
	}
}

func BenchmarkWorkerPoolMetrics(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	p := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:   2,
		QueueSize: 10,
	})
	p.Start()
	defer p.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Metrics()
	}
}
