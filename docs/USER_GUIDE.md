# User Guide

## Installation

```bash
go get digital.vasic.concurrency
```

Requires Go 1.24 or later.

## Worker Pool (`pkg/pool`)

The worker pool provides bounded concurrency with configurable workers, task timeouts, metrics tracking, and batch execution.

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"

    "digital.vasic.concurrency/pkg/pool"
)

func main() {
    wp := pool.NewWorkerPool(&pool.PoolConfig{
        Workers:       4,
        QueueSize:     100,
        TaskTimeout:   30 * time.Second,
        ShutdownGrace: 5 * time.Second,
    })
    defer wp.Shutdown(5 * time.Second)

    // Submit a task
    err := wp.Submit(pool.NewTaskFunc("greet", func(ctx context.Context) (interface{}, error) {
        return "hello, world", nil
    }))
    if err != nil {
        fmt.Printf("submit error: %v\n", err)
    }

    // Read results
    result := <-wp.Results()
    fmt.Printf("Task %s returned: %v\n", result.TaskID, result.Value)
}
```

### Submit and Wait

`SubmitWait` submits a task and blocks until the specific task completes:

```go
ctx := context.Background()

result, err := wp.SubmitWait(ctx, pool.NewTaskFunc("compute", func(ctx context.Context) (interface{}, error) {
    return 42, nil
}))
if err != nil {
    fmt.Printf("error: %v\n", err)
}
fmt.Println(result.Value)    // 42
fmt.Println(result.Duration) // execution time
```

### Batch Submission

Submit multiple tasks and collect results from a channel:

```go
tasks := []pool.Task{
    pool.NewTaskFunc("t1", func(ctx context.Context) (interface{}, error) { return 1, nil }),
    pool.NewTaskFunc("t2", func(ctx context.Context) (interface{}, error) { return 2, nil }),
    pool.NewTaskFunc("t3", func(ctx context.Context) (interface{}, error) { return 3, nil }),
}

// Streaming results
for result := range wp.SubmitBatch(tasks) {
    fmt.Printf("%s: %v\n", result.TaskID, result.Value)
}

// Or collect all at once
results, err := wp.SubmitBatchWait(ctx, tasks)
```

### Parallel Execute (Convenience)

For one-off parallel execution without managing a pool:

```go
fns := []func(ctx context.Context) (interface{}, error){
    func(ctx context.Context) (interface{}, error) { return fetchURL("https://a.com") },
    func(ctx context.Context) (interface{}, error) { return fetchURL("https://b.com") },
}

results, err := pool.ParallelExecute(ctx, fns)
for _, r := range results {
    fmt.Println(r.Value)
}
```

### Generic Parallel Map

Apply a function to a slice in parallel with bounded concurrency:

```go
numbers := []int{1, 2, 3, 4, 5}

doubled, err := pool.Map(ctx, numbers, 3, func(ctx context.Context, n int) (int, error) {
    return n * 2, nil
})
fmt.Println(doubled) // [2 4 6 8 10]
```

### Error Callbacks and Metrics

```go
wp := pool.NewWorkerPool(&pool.PoolConfig{
    Workers:   4,
    QueueSize: 100,
    OnError: func(taskID string, err error) {
        log.Printf("FAIL %s: %v", taskID, err)
    },
    OnComplete: func(result pool.Result) {
        log.Printf("DONE %s in %v", result.TaskID, result.Duration)
    },
})

// Check metrics
metrics := wp.Metrics()
fmt.Printf("Active: %d, Completed: %d, Failed: %d, Avg Latency: %v\n",
    metrics.ActiveWorkers,
    metrics.CompletedTasks,
    metrics.FailedTasks,
    metrics.AverageLatency(),
)
```

### Draining the Queue

Wait until all submitted tasks have been processed:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

err := wp.WaitForDrain(ctx)
if err != nil {
    fmt.Println("timed out waiting for drain")
}
```

### Default Configuration

Passing `nil` to `NewWorkerPool` uses defaults:

- Workers: `runtime.NumCPU()`
- QueueSize: 1000
- TaskTimeout: 30 seconds
- ShutdownGrace: 5 seconds

---

## Priority Queue (`pkg/queue`)

A generic, thread-safe priority queue backed by a heap. Items with higher priority are dequeued first. Items with equal priority are dequeued in FIFO order.

### Priority Levels

| Constant | Value | Description |
|----------|-------|-------------|
| `queue.Low` | 0 | Lowest priority |
| `queue.Normal` | 1 | Default priority |
| `queue.High` | 2 | Elevated priority |
| `queue.Critical` | 3 | Highest priority |

### Usage

```go
import "digital.vasic.concurrency/pkg/queue"

// Create a queue of strings with pre-allocated capacity of 64
pq := queue.New[string](64)

// Push items with different priorities
pq.Push("background-job", queue.Low)
pq.Push("user-request", queue.Normal)
pq.Push("system-alert", queue.Critical)
pq.Push("admin-task", queue.High)

// Pop returns highest priority first
val, ok := pq.Pop() // "system-alert", true
val, ok = pq.Pop()  // "admin-task", true
val, ok = pq.Pop()  // "user-request", true
val, ok = pq.Pop()  // "background-job", true
val, ok = pq.Pop()  // "", false (empty)
```

### Peeking

```go
// Peek without removing
val, ok := pq.Peek()
if ok {
    fmt.Println("Next item:", val)
}

// Check size
fmt.Println("Queue length:", pq.Len())
fmt.Println("Is empty:", pq.IsEmpty())
```

### Generic Type Support

The queue works with any type:

```go
type Job struct {
    Name    string
    Payload []byte
}

jobQueue := queue.New[Job](0)
jobQueue.Push(Job{Name: "process", Payload: data}, queue.High)

job, ok := jobQueue.Pop()
```

---

## Rate Limiting (`pkg/limiter`)

Two rate limiting strategies share the `RateLimiter` interface, making them interchangeable.

### Token Bucket

Smooth rate limiting with burst capacity. Tokens are added at a fixed rate up to a maximum capacity. Each request consumes one token.

```go
import "digital.vasic.concurrency/pkg/limiter"

rl := limiter.NewTokenBucket(&limiter.TokenBucketConfig{
    Rate:     100.0, // 100 tokens per second
    Capacity: 10,    // burst of 10 requests
})

ctx := context.Background()

// Non-blocking check
if rl.Allow(ctx) {
    handleRequest()
}

// Blocking wait
err := rl.Wait(ctx)
if err != nil {
    // context was cancelled
}
```

The bucket starts full, so the first `Capacity` requests are immediately allowed.

### Sliding Window

Time-window-based request counting. Tracks timestamps of allowed requests and rejects new ones when the window limit is reached.

```go
rl := limiter.NewSlidingWindow(&limiter.SlidingWindowConfig{
    WindowSize:  time.Second,  // 1-second window
    MaxRequests: 100,          // max 100 requests per second
})

if rl.Allow(ctx) {
    handleRequest()
}

// Or block until a slot opens
err := rl.Wait(ctx)
```

### Using the Interface

Since both implement `RateLimiter`, you can write code that accepts either:

```go
func protectedHandler(rl limiter.RateLimiter, ctx context.Context) error {
    if !rl.Allow(ctx) {
        return fmt.Errorf("rate limited")
    }
    return processRequest()
}
```

### Default Configurations

- **TokenBucket** (nil config): Rate=10, Capacity=10
- **SlidingWindow** (nil config): WindowSize=1s, MaxRequests=100

---

## Circuit Breaker (`pkg/breaker`)

Implements the circuit breaker pattern with three states: Closed, Open, and HalfOpen.

### State Machine

1. **Closed** (normal): Requests pass through. Consecutive failures are counted. When failures reach `MaxFailures`, transitions to Open.
2. **Open** (failing): All requests are immediately rejected with an error. After `Timeout` elapses, transitions to HalfOpen.
3. **HalfOpen** (probing): A limited number of requests (`HalfOpenRequests`) are allowed through. If they succeed, transitions back to Closed. If any fail, transitions back to Open.

### Usage

```go
import (
    "time"
    "digital.vasic.concurrency/pkg/breaker"
)

cb := breaker.New(&breaker.Config{
    MaxFailures:      5,              // open after 5 consecutive failures
    Timeout:          30 * time.Second, // stay open for 30 seconds
    HalfOpenRequests: 2,              // allow 2 probe requests
})

// Wrap calls with circuit breaker protection
err := cb.Execute(func() error {
    return callExternalService()
})

if err != nil {
    // Either the function failed, or the circuit is open
    fmt.Println("error:", err)
}

// Check state
fmt.Println("State:", cb.State()) // "closed", "open", or "half-open"

// Check failure count
fmt.Println("Failures:", cb.Failures())

// Force reset to closed
cb.Reset()
```

### Default Configuration

Passing `nil` to `New` uses defaults:

- MaxFailures: 5
- Timeout: 30 seconds
- HalfOpenRequests: 1

---

## Semaphore (`pkg/semaphore`)

A weighted semaphore for controlling concurrent access to shared resources. Supports context cancellation and non-blocking acquisition.

### Basic Usage

```go
import "digital.vasic.concurrency/pkg/semaphore"

// Allow up to 10 units of concurrent access
sem := semaphore.New(10)

ctx := context.Background()

// Acquire 3 units (blocks if not available)
err := sem.Acquire(ctx, 3)
if err != nil {
    fmt.Println("acquire failed:", err)
}
defer sem.Release(3)

// Do work with the acquired resource
```

### Non-Blocking Acquire

```go
if sem.TryAcquire(5) {
    defer sem.Release(5)
    // proceed
} else {
    // not enough capacity right now
}
```

### Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

err := sem.Acquire(ctx, 8)
if err != nil {
    // context timed out or was cancelled while waiting
    fmt.Println("could not acquire:", err)
}
```

### Checking Capacity

```go
fmt.Println("Currently acquired:", sem.Current())
fmt.Println("Available:", sem.Available())
```

### Limiting Database Connections

```go
// Allow at most 20 concurrent database queries
dbSem := semaphore.New(20)

func queryDB(ctx context.Context, query string) (Result, error) {
    if err := dbSem.Acquire(ctx, 1); err != nil {
        return Result{}, fmt.Errorf("too many concurrent queries: %w", err)
    }
    defer dbSem.Release(1)

    return db.ExecContext(ctx, query)
}
```

---

## Resource Monitor (`pkg/monitor`)

Collects system resource snapshots (CPU, memory, disk, load averages, Go runtime stats) using `gopsutil`.

### One-Shot Collection

```go
import "digital.vasic.concurrency/pkg/monitor"

mon := monitor.New(nil) // default config

ctx := context.Background()
resources, err := mon.GetSystemResources(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("CPU: %.1f%%\n", resources.CPUPercent)
fmt.Printf("Memory: %.1f%% (%d MB used / %d MB total)\n",
    resources.MemoryPercent,
    resources.MemoryUsed/1024/1024,
    resources.MemoryTotal/1024/1024,
)
fmt.Printf("Disk: %.1f%%\n", resources.DiskPercent)
fmt.Printf("Load: %.2f / %.2f / %.2f\n",
    resources.Load1, resources.Load5, resources.Load15,
)
fmt.Printf("Goroutines: %d\n", resources.NumGoroutines)
fmt.Printf("Heap: %d MB\n", resources.HeapAlloc/1024/1024)
```

### Background Collection

Start periodic collection and retrieve the latest snapshot at any time:

```go
mon := monitor.New(&monitor.Config{
    DiskPath:        "/",
    CPUSampleTime:   200 * time.Millisecond,
    CollectInterval: 5 * time.Second,
})

ctx := context.Background()
if err := mon.Start(ctx); err != nil {
    log.Fatal(err)
}
defer mon.Stop()

// Later, get the most recent snapshot (non-blocking)
latest := mon.Latest()
if latest != nil {
    fmt.Printf("CPU: %.1f%% (collected at %s)\n",
        latest.CPUPercent,
        latest.CollectedAt.Format(time.RFC3339),
    )
}
```

### Custom Configuration

```go
mon := monitor.New(&monitor.Config{
    DiskPath:        "/data",               // monitor /data partition
    CPUSampleTime:   500 * time.Millisecond, // longer CPU sample
    CollectInterval: 10 * time.Second,       // collect every 10s
})
```

### Default Configuration

Passing `nil` to `New` uses defaults:

- DiskPath: `/`
- CPUSampleTime: 200ms
- CollectInterval: 5 seconds

---

## Combining Primitives

The packages are designed to work together. Here is an example of a rate-limited worker pool with circuit breaker protection:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "digital.vasic.concurrency/pkg/breaker"
    "digital.vasic.concurrency/pkg/limiter"
    "digital.vasic.concurrency/pkg/pool"
)

func main() {
    ctx := context.Background()

    // Rate limit to 50 requests per second
    rl := limiter.NewTokenBucket(&limiter.TokenBucketConfig{
        Rate:     50,
        Capacity: 10,
    })

    // Circuit breaker for the downstream service
    cb := breaker.New(&breaker.Config{
        MaxFailures:      3,
        Timeout:          10 * time.Second,
        HalfOpenRequests: 1,
    })

    // Worker pool with 8 workers
    wp := pool.NewWorkerPool(&pool.PoolConfig{
        Workers:   8,
        QueueSize: 200,
    })
    defer wp.Shutdown(5 * time.Second)

    // Submit rate-limited, circuit-breaker-protected tasks
    for i := 0; i < 100; i++ {
        i := i
        wp.Submit(pool.NewTaskFunc(
            fmt.Sprintf("req-%d", i),
            func(ctx context.Context) (interface{}, error) {
                // Wait for rate limiter
                if err := rl.Wait(ctx); err != nil {
                    return nil, err
                }
                // Execute through circuit breaker
                var result interface{}
                err := cb.Execute(func() error {
                    var callErr error
                    result, callErr = callDownstreamService(i)
                    return callErr
                })
                return result, err
            },
        ))
    }
}
```
