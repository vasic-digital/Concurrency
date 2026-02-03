# Architecture

## Design Philosophy

`digital.vasic.concurrency` is a focused library of concurrency primitives for Go. Every design decision follows these principles:

1. **Generic and reusable** -- No application-specific logic. Every package is independently useful.
2. **Thread-safe by default** -- All exported methods are safe for concurrent use. Internal synchronization uses `sync.Mutex` or `sync.RWMutex`.
3. **Context-aware** -- Long-running and blocking operations accept `context.Context` for cancellation and timeout propagation.
4. **Zero external state** -- No databases, no files, no network calls (except `monitor` which reads system stats). All state is in-memory.
5. **Minimal dependencies** -- Only `gopsutil` (for `monitor`) and `testify` (for tests).

## Package Dependency Graph

```
                    (no internal dependencies between packages)

    pkg/pool        pkg/queue       pkg/limiter
        |               |               |
        v               v               v
    [stdlib]        [stdlib]        [stdlib]

    pkg/breaker     pkg/semaphore   pkg/monitor
        |               |               |
        v               v               v
    [stdlib]        [stdlib]        [gopsutil]
```

Packages have zero dependencies on each other. This is intentional: consumers import only what they need, and changes to one package cannot break another.

## Design Patterns

### Worker Pool Pattern (`pkg/pool`)

**Problem**: Execute many tasks concurrently with bounded parallelism, without spawning unbounded goroutines.

**Solution**: A fixed number of worker goroutines consume tasks from a buffered channel. A semaphore channel (`chan struct{}`) enforces the worker count limit.

**Key decisions**:

- **Auto-start on first Submit**: The pool starts lazily. Calling `Submit` automatically starts workers if the pool has not been started yet. This avoids requiring a separate `Start()` call in simple use cases.
- **Channel-based task queue**: Tasks are submitted to a buffered channel (`chan Task`). The buffer size is configurable via `QueueSize`. When the buffer is full, `Submit` returns an error rather than blocking, giving the caller control over backpressure.
- **Result channel**: Completed tasks push results to a separate buffered channel. Consumers can read from `Results()` or use `SubmitWait`/`SubmitBatchWait` for synchronous semantics.
- **Atomic metrics**: `PoolMetrics` uses `sync/atomic` for lock-free counter updates, avoiding contention between workers updating metrics simultaneously.
- **Graceful shutdown**: `Shutdown` closes the task channel (signaling workers to drain), then waits with a timeout. `Stop` cancels the context immediately for hard shutdown.

**Task interface**:

```go
type Task interface {
    ID() string
    Execute(ctx context.Context) (interface{}, error)
}
```

The `TaskFunc` adapter wraps plain functions as `Task` implementations via `NewTaskFunc`.

**Generic Map**: The `Map[T, R]` function provides type-safe parallel map using Go generics. It creates an ephemeral pool, submits all items, collects results, and shuts down.

### Priority Queue Pattern (`pkg/queue`)

**Problem**: Process items by priority, with FIFO ordering within the same priority level.

**Solution**: A min-heap (via `container/heap`) with composite ordering: primary sort by priority (descending), secondary sort by insertion sequence number (ascending).

**Key decisions**:

- **Go generics**: `PriorityQueue[T any]` works with any type. No type assertions needed by the consumer.
- **Sequence numbers**: An atomic `uint64` counter assigns monotonically increasing sequence numbers to each pushed item, ensuring stable FIFO ordering within a priority level.
- **Four priority levels**: `Low(0)`, `Normal(1)`, `High(2)`, `Critical(3)`. These are `int` constants, so custom intermediate levels are possible but not encouraged.
- **Mutex protection**: All public methods lock a `sync.Mutex`. The queue is safe for concurrent producers and consumers.

### Rate Limiter Patterns (`pkg/limiter`)

**Common interface**:

```go
type RateLimiter interface {
    Allow(ctx context.Context) bool
    Wait(ctx context.Context) error
}
```

Both implementations satisfy this interface, allowing them to be swapped without changing consumer code.

#### Token Bucket

**Problem**: Smooth rate limiting that allows short bursts.

**Solution**: Tokens are added at a fixed rate (tokens per second) up to a maximum capacity. Each `Allow` call consumes one token. The bucket refills based on elapsed wall-clock time since the last refill.

**Key decisions**:

- **Lazy refill**: Tokens are calculated on demand (in `Allow`/`Wait`) rather than by a background goroutine. This eliminates the need for a timer and simplifies lifecycle management.
- **Float64 tokens**: Fractional tokens allow smooth sub-second refill rates without integer rounding artifacts.
- **Spin-wait with backoff**: `Wait` calculates the expected time until the next token is available and sleeps for that duration, then retries. Minimum sleep is 1ms to avoid busy-waiting.

#### Sliding Window

**Problem**: Strict request count limit within a rolling time window.

**Solution**: Maintains a slice of timestamps for each allowed request. On each `Allow` call, timestamps outside the window are pruned. If the remaining count is below `MaxRequests`, the request is allowed and its timestamp is recorded.

**Key decisions**:

- **Slice-based storage**: Timestamps are stored in a sorted slice. Cleanup scans from the front (oldest), which is O(k) where k is the number of expired entries. This is efficient for typical request rates.
- **No background cleanup**: Expired timestamps are cleaned up lazily on each `Allow` call, avoiding the need for a background goroutine.

### Circuit Breaker Pattern (`pkg/breaker`)

**Problem**: Prevent cascading failures by failing fast when a dependency is unhealthy, while allowing automatic recovery.

**Solution**: Three-state machine (Closed, Open, HalfOpen) with configurable failure thresholds and recovery timeouts.

**State transitions**:

```
    [Closed] ---(MaxFailures reached)---> [Open]
    [Open]   ---(Timeout elapsed)-------> [HalfOpen]
    [HalfOpen] ---(probe succeeds)------> [Closed]
    [HalfOpen] ---(probe fails)---------> [Open]
```

**Key decisions**:

- **Consecutive failures**: The failure counter resets to zero on any success in the Closed state. This means intermittent failures do not accumulate.
- **Lazy state transition**: The transition from Open to HalfOpen is checked at the start of each `Execute` call (in `checkStateTransition`), not by a background timer.
- **HalfOpen probe limit**: Only `HalfOpenRequests` calls are allowed through during HalfOpen. Additional calls are rejected. This prevents a flood of requests to a recovering service.
- **Manual reset**: `Reset()` forces the breaker back to Closed, useful for administrative recovery.

### Semaphore Pattern (`pkg/semaphore`)

**Problem**: Limit concurrent access to a resource where different operations may consume different amounts of capacity (weighted access).

**Solution**: A weighted semaphore that tracks current usage against a maximum weight. Waiters are queued and notified when capacity becomes available.

**Key decisions**:

- **Waiter queue**: Blocked `Acquire` calls create a `waiter` struct with a `chan struct{}` that is closed when the waiter can proceed. This allows efficient wake-up without polling.
- **FIFO-ish notification**: `notifyWaiters` scans the waiter list front-to-back and wakes any waiter whose requested weight fits in the available capacity. This is not strictly FIFO (a smaller later waiter may be woken before a larger earlier one), which maximizes throughput.
- **Context cancellation**: If an `Acquire` call's context is cancelled while waiting, the waiter removes itself from the queue and returns the context error.
- **TryAcquire**: Non-blocking acquisition for scenarios where callers prefer to fall back rather than wait.

### Resource Monitor (`pkg/monitor`)

**Problem**: Collect system resource usage (CPU, memory, disk, load averages) for health checks and adaptive behavior.

**Solution**: Wraps `gopsutil/v3` with a simple API that returns a `SystemResources` snapshot. Supports both one-shot and periodic background collection.

**Key decisions**:

- **RWMutex for latest**: Background collection writes to `latest` under a write lock. `Latest()` reads under a read lock. This allows multiple concurrent readers without blocking.
- **Graceful degradation**: If any individual metric collection fails (CPU, memory, disk, load), the others still populate. Only a total failure returns an error.
- **Go runtime stats included**: `NumGoroutines`, `HeapAlloc`, and `HeapSys` are collected from `runtime.ReadMemStats` alongside OS-level metrics.

## Thread Safety Summary

| Package | Synchronization Mechanism |
|---------|--------------------------|
| `pool` | `sync.Mutex` for state, `sync/atomic` for metrics, channels for task/result flow |
| `queue` | `sync.Mutex` on all public methods |
| `limiter` | `sync.Mutex` on all public methods |
| `breaker` | `sync.Mutex` on all public methods (released during `fn()` execution) |
| `semaphore` | `sync.Mutex` with channel-based waiter notification |
| `monitor` | `sync.RWMutex` for latest snapshot |

## Error Handling

- **pool**: `Submit` returns an error if the pool is closed or the queue is full. Task errors are captured in `Result.Error` and optionally forwarded to `OnError` callback.
- **queue**: `Pop` and `Peek` return a boolean indicating whether an item was available, following the Go comma-ok idiom.
- **limiter**: `Allow` returns a boolean. `Wait` returns a context error if cancelled.
- **breaker**: `Execute` returns either the wrapped function's error or a circuit-open error.
- **semaphore**: `Acquire` returns a context error or a weight-exceeded error. `TryAcquire` returns a boolean.
- **monitor**: `GetSystemResources` returns partial results on individual metric failures; `Start` returns an error only if the initial collection fails entirely.
