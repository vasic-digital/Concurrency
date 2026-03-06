# Lesson 1: Worker Pool with Bounded Parallelism

## Learning Objectives

- Build a worker pool that executes tasks concurrently with a fixed number of goroutines
- Implement channel-based task queuing with backpressure control
- Use Go generics for type-safe parallel Map operations

## Key Concepts

- **Auto-Start on First Submit**: The pool starts lazily. Calling `Submit` automatically starts workers if the pool has not been started yet, avoiding a separate `Start()` call.
- **Buffered Task Channel**: Tasks are submitted to a buffered channel whose size is configurable via `QueueSize`. When the buffer is full, `Submit` returns an error rather than blocking, giving the caller control over backpressure.
- **Atomic Metrics**: `PoolMetrics` uses `sync/atomic` for lock-free counter updates, avoiding contention between workers updating metrics simultaneously.
- **Graceful Shutdown**: `Shutdown` closes the task channel (signaling workers to drain), then waits with a timeout. `Stop` cancels the context immediately for hard shutdown.

## Code Walkthrough

### Source: `pkg/pool/pool.go`

The `Task` interface defines what workers execute:

```go
type Task interface {
    ID() string
    Execute(ctx context.Context) (interface{}, error)
}
```

The `TaskFunc` adapter wraps plain functions as `Task` implementations via `NewTaskFunc`, following the standard Go adapter pattern (like `http.HandlerFunc`).

Workers are goroutines that range over the task channel. Each picks up a task, calls `Execute`, and pushes the result to a separate results channel. The pool tracks submitted, completed, and failed counts atomically.

The `Map[T, R]` function provides type-safe parallel map using Go generics -- it creates an ephemeral pool, submits all items, collects results, and shuts down. This is the highest-level convenience function.

### Source: `pkg/pool/pool_test.go`

Tests verify:
- Basic submit-and-collect workflow
- Pool auto-start behavior
- Queue-full backpressure (Submit returns error)
- Graceful shutdown drains pending tasks
- Concurrent submit from multiple goroutines
- `Map` produces correct results in correct order

## Practice Exercise

1. Read `pkg/pool/pool.go` and identify the goroutine lifecycle: where workers start, how they read tasks, and how shutdown signals propagate.
2. Create a pool with 3 workers and queue size 10. Submit 20 tasks that each sleep for 100ms and return their task ID. Collect all results and verify no tasks were lost.
3. Implement a custom `Task` that downloads a URL and returns the HTTP status code. Use `Map` to process a list of 10 URLs in parallel with 4 workers.
