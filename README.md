# digital.vasic.concurrency

A generic, reusable Go module for concurrency primitives: worker pools, priority queues, rate limiters, circuit breakers, semaphores, and resource monitoring.

## Installation

```bash
go get digital.vasic.concurrency
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    "digital.vasic.concurrency/pkg/breaker"
    "digital.vasic.concurrency/pkg/limiter"
    "digital.vasic.concurrency/pkg/pool"
    "digital.vasic.concurrency/pkg/queue"
    "digital.vasic.concurrency/pkg/semaphore"
)

func main() {
    ctx := context.Background()

    // Worker pool
    wp := pool.NewWorkerPool(&pool.PoolConfig{
        Workers:   4,
        QueueSize: 100,
    })
    defer wp.Stop()

    wp.Submit(pool.NewTaskFunc("task-1", func(ctx context.Context) (interface{}, error) {
        return "hello", nil
    }))

    // Parallel map
    results, _ := pool.Map(ctx, []int{1, 2, 3}, 3,
        func(ctx context.Context, n int) (int, error) {
            return n * 2, nil
        },
    )
    fmt.Println(results) // [2 4 6]

    // Priority queue
    pq := queue.New[string](0)
    pq.Push("low-priority", queue.Low)
    pq.Push("critical-task", queue.Critical)
    item, _ := pq.Pop() // "critical-task"
    fmt.Println(item)

    // Rate limiter
    rl := limiter.NewTokenBucket(&limiter.TokenBucketConfig{
        Rate:     100,
        Capacity: 10,
    })
    if rl.Allow(ctx) {
        fmt.Println("Request allowed")
    }

    // Circuit breaker
    cb := breaker.New(&breaker.Config{
        MaxFailures:      5,
        Timeout:          10 * time.Second,
        HalfOpenRequests: 2,
    })
    err := cb.Execute(func() error {
        return nil // protected call
    })
    fmt.Println(err)

    // Semaphore
    sem := semaphore.New(10)
    _ = sem.Acquire(ctx, 3)
    defer sem.Release(3)
}
```

## Features

- **Worker Pool**: bounded concurrency, task batching, parallel map, metrics tracking
- **Priority Queue**: generic type parameter, 4 priority levels, thread-safe
- **Token Bucket Rate Limiter**: smooth rate limiting with burst capacity
- **Sliding Window Rate Limiter**: time-window-based request counting
- **Circuit Breaker**: closed/open/half-open states, automatic recovery
- **Weighted Semaphore**: resource access control with context support
- **Resource Monitor**: CPU, memory, disk usage via gopsutil
- **Thread-safe**: all components safe for concurrent use

## Packages

| Package | Description |
|---------|-------------|
| `pkg/pool` | Worker pool with task submission and parallel execution |
| `pkg/queue` | Generic priority task queue |
| `pkg/limiter` | Token bucket and sliding window rate limiters |
| `pkg/breaker` | Circuit breaker for fault tolerance |
| `pkg/semaphore` | Weighted semaphore |
| `pkg/monitor` | System resource monitoring |

## Worker Pool

```go
// Create and configure
wp := pool.NewWorkerPool(&pool.PoolConfig{
    Workers:       8,
    QueueSize:     1000,
    TaskTimeout:   30 * time.Second,
    ShutdownGrace: 5 * time.Second,
    OnError: func(taskID string, err error) {
        log.Printf("Task %s failed: %v", taskID, err)
    },
})
defer wp.Shutdown(5 * time.Second)

// Submit tasks
wp.Submit(pool.NewTaskFunc("job-1", myFunc))

// Submit and wait
result, err := wp.SubmitWait(ctx, myTask)

// Batch submission
results := wp.SubmitBatch(tasks)
for r := range results {
    fmt.Println(r.TaskID, r.Value)
}

// Parallel execute convenience function
results, err := pool.ParallelExecute(ctx, funcs)

// Generic parallel map
doubled, err := pool.Map(ctx, numbers, 4, func(ctx context.Context, n int) (int, error) {
    return n * 2, nil
})
```

## Rate Limiting

```go
// Token bucket
rl := limiter.NewTokenBucket(&limiter.TokenBucketConfig{
    Rate:     100.0, // 100 tokens per second
    Capacity: 10,    // burst of 10
})

// Sliding window
rl := limiter.NewSlidingWindow(&limiter.SlidingWindowConfig{
    WindowSize: time.Second,
    MaxRequests: 100,
})

// Use the limiter
if rl.Allow(ctx) {
    // proceed
}
_ = rl.Wait(ctx) // blocks until allowed
```

## Circuit Breaker

```go
cb := breaker.New(&breaker.Config{
    MaxFailures:      5,
    Timeout:          30 * time.Second,
    HalfOpenRequests: 2,
})

err := cb.Execute(func() error {
    return externalServiceCall()
})

fmt.Println(cb.State()) // Closed, Open, or HalfOpen
```
