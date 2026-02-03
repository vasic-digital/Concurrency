# API Reference

Complete reference for all exported types, functions, and methods in `digital.vasic.concurrency`.

---

## Package `pool`

**Import**: `digital.vasic.concurrency/pkg/pool`

### Interfaces

#### `Task`

Represents a unit of work to be executed by the worker pool.

```go
type Task interface {
    ID() string
    Execute(ctx context.Context) (interface{}, error)
}
```

| Method | Description |
|--------|-------------|
| `ID() string` | Returns a unique identifier for the task. |
| `Execute(ctx context.Context) (interface{}, error)` | Runs the task. The context carries the pool's cancellation and the per-task timeout. |

---

### Types

#### `TaskFunc`

Wraps a plain function as a `Task` implementation.

```go
type TaskFunc struct {
    // unexported fields
}
```

| Method | Signature | Description |
|--------|-----------|-------------|
| `ID` | `() string` | Returns the task ID provided at construction. |
| `Execute` | `(ctx context.Context) (interface{}, error)` | Calls the wrapped function. |

#### `NewTaskFunc`

```go
func NewTaskFunc(id string, fn func(ctx context.Context) (interface{}, error)) *TaskFunc
```

Creates a new `TaskFunc` with the given ID and function.

---

#### `Result`

Represents the outcome of a task execution.

```go
type Result struct {
    TaskID    string
    Value     interface{}
    Error     error
    StartTime time.Time
    Duration  time.Duration
}
```

| Field | Type | Description |
|-------|------|-------------|
| `TaskID` | `string` | The ID of the task that produced this result. |
| `Value` | `interface{}` | The return value from `Task.Execute`. |
| `Error` | `error` | The error from `Task.Execute`, or `nil` on success. |
| `StartTime` | `time.Time` | When execution began. |
| `Duration` | `time.Duration` | How long execution took. |

---

#### `PoolConfig`

Configuration for the worker pool.

```go
type PoolConfig struct {
    Workers       int
    QueueSize     int
    TaskTimeout   time.Duration
    ShutdownGrace time.Duration
    OnError       func(taskID string, err error)
    OnComplete    func(result Result)
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Workers` | `int` | `runtime.NumCPU()` | Number of concurrent workers. |
| `QueueSize` | `int` | `1000` | Size of the buffered task channel. |
| `TaskTimeout` | `time.Duration` | `30s` | Maximum time for a single task. Zero means no timeout. |
| `ShutdownGrace` | `time.Duration` | `5s` | Grace period during shutdown before force-cancelling. |
| `OnError` | `func(string, error)` | `nil` | Called when a task returns an error. |
| `OnComplete` | `func(Result)` | `nil` | Called when a task completes (success or failure). |

#### `DefaultPoolConfig`

```go
func DefaultPoolConfig() *PoolConfig
```

Returns a `PoolConfig` with default values.

---

#### `PoolMetrics`

Tracks worker pool statistics. All fields are updated atomically.

```go
type PoolMetrics struct {
    ActiveWorkers  int64
    QueuedTasks    int64
    CompletedTasks int64
    FailedTasks    int64
    TotalLatencyUs int64
    TaskCount      int64
}
```

| Field | Type | Description |
|-------|------|-------------|
| `ActiveWorkers` | `int64` | Number of workers currently executing tasks. |
| `QueuedTasks` | `int64` | Number of tasks submitted but not yet started. |
| `CompletedTasks` | `int64` | Number of tasks that completed successfully. |
| `FailedTasks` | `int64` | Number of tasks that returned an error. |
| `TotalLatencyUs` | `int64` | Sum of all task durations in microseconds. |
| `TaskCount` | `int64` | Total number of tasks executed (for average calculation). |

| Method | Signature | Description |
|--------|-----------|-------------|
| `AverageLatency` | `() time.Duration` | Returns `TotalLatencyUs / TaskCount` as a `time.Duration`. Returns 0 if no tasks have been executed. |

---

#### `WorkerPool`

Provides bounded concurrency with configurable workers.

```go
type WorkerPool struct {
    // unexported fields
}
```

#### `NewWorkerPool`

```go
func NewWorkerPool(config *PoolConfig) *WorkerPool
```

Creates a new worker pool. Passing `nil` uses `DefaultPoolConfig()`. The pool does not start workers until `Start()` is called or the first task is submitted.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Start` | `()` | Starts worker goroutines. Called automatically on first `Submit`. Safe to call multiple times. |
| `Submit` | `(task Task) error` | Adds a task to the queue. Returns an error if the pool is closed or the queue is full. Auto-starts the pool if needed. |
| `SubmitWait` | `(ctx context.Context, task Task) (Result, error)` | Submits a task and blocks until it completes or the context is cancelled. |
| `SubmitBatch` | `(tasks []Task) <-chan Result` | Submits multiple tasks and returns a channel that receives results as they complete. The channel is closed when all submitted tasks have results. |
| `SubmitBatchWait` | `(ctx context.Context, tasks []Task) ([]Result, error)` | Submits multiple tasks and blocks until all complete or the context is cancelled. |
| `Results` | `() <-chan Result` | Returns the read-only results channel. |
| `Metrics` | `() *PoolMetrics` | Returns a snapshot of current pool metrics. |
| `QueueLength` | `() int` | Returns the number of tasks currently in the queue. |
| `ActiveWorkers` | `() int` | Returns the number of workers currently executing tasks. |
| `IsRunning` | `() bool` | Returns `true` if the pool has been started and not closed. |
| `WaitForDrain` | `(ctx context.Context) error` | Blocks until the task queue is empty and no workers are active, or the context is cancelled. Polls every 100ms. |
| `Shutdown` | `(timeout time.Duration) error` | Gracefully shuts down: closes the task queue, waits for workers to finish up to `timeout`, then cancels remaining work. |
| `Stop` | `()` | Immediately cancels all work and shuts down. |

---

### Functions

#### `ParallelExecute`

```go
func ParallelExecute(
    ctx context.Context,
    fns []func(ctx context.Context) (interface{}, error),
) ([]Result, error)
```

Creates an ephemeral pool with `len(fns)` workers, submits all functions, waits for all results, and shuts down. Convenience function for one-off parallel execution.

#### `Map`

```go
func Map[T any, R any](
    ctx context.Context,
    items []T,
    workers int,
    fn func(ctx context.Context, item T) (R, error),
) ([]R, error)
```

Applies `fn` to each item in `items` using `workers` concurrent goroutines. Returns results in order. Returns an error if any invocation fails.

---

## Package `queue`

**Import**: `digital.vasic.concurrency/pkg/queue`

### Constants

```go
const (
    Low      Priority = 0
    Normal   Priority = 1
    High     Priority = 2
    Critical Priority = 3
)
```

### Types

#### `Priority`

```go
type Priority int
```

Represents the priority level of a queued item. Higher numeric values are dequeued first.

| Method | Signature | Description |
|--------|-----------|-------------|
| `String` | `() string` | Returns `"low"`, `"normal"`, `"high"`, `"critical"`, or `"unknown(N)"`. |

---

#### `PriorityQueue[T any]`

A generic, thread-safe priority queue. Items with higher priority are dequeued first. Items with equal priority are dequeued in FIFO order.

```go
type PriorityQueue[T any] struct {
    // unexported fields
}
```

#### `New`

```go
func New[T any](initialCap int) *PriorityQueue[T]
```

Creates a new `PriorityQueue`. If `initialCap > 0`, the underlying slice is pre-allocated with that capacity. Negative values are treated as 0.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Push` | `(value T, priority Priority)` | Adds a value with the given priority. |
| `Pop` | `() (T, bool)` | Removes and returns the highest-priority item. Returns `(zero, false)` if empty. |
| `Peek` | `() (T, bool)` | Returns the highest-priority item without removing it. Returns `(zero, false)` if empty. |
| `Len` | `() int` | Returns the number of items in the queue. |
| `IsEmpty` | `() bool` | Returns `true` if the queue has no items. |

---

## Package `limiter`

**Import**: `digital.vasic.concurrency/pkg/limiter`

### Interfaces

#### `RateLimiter`

Common interface for rate limiting strategies.

```go
type RateLimiter interface {
    Allow(ctx context.Context) bool
    Wait(ctx context.Context) error
}
```

| Method | Description |
|--------|-------------|
| `Allow(ctx context.Context) bool` | Reports whether a single event may happen now. Does not block. |
| `Wait(ctx context.Context) error` | Blocks until a single event is allowed or the context is cancelled. Returns `ctx.Err()` on cancellation. |

---

### Types

#### `TokenBucketConfig`

```go
type TokenBucketConfig struct {
    Rate     float64 // Tokens added per second
    Capacity int     // Maximum burst size (bucket capacity)
}
```

#### `TokenBucket`

Implements a token bucket rate limiter. Satisfies `RateLimiter`.

```go
type TokenBucket struct {
    // unexported fields
}
```

#### `NewTokenBucket`

```go
func NewTokenBucket(cfg *TokenBucketConfig) *TokenBucket
```

Creates a new token bucket. The bucket starts full (at `Capacity` tokens). Passing `nil` uses defaults: Rate=10, Capacity=10.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Allow` | `(ctx context.Context) bool` | Consumes one token if available. Returns `true` if consumed, `false` otherwise. |
| `Wait` | `(ctx context.Context) error` | Blocks until one token is available or the context is cancelled. |

---

#### `SlidingWindowConfig`

```go
type SlidingWindowConfig struct {
    WindowSize  time.Duration // Duration of the sliding window
    MaxRequests int           // Maximum requests per window
}
```

#### `SlidingWindow`

Implements a sliding window rate limiter. Satisfies `RateLimiter`.

```go
type SlidingWindow struct {
    // unexported fields
}
```

#### `NewSlidingWindow`

```go
func NewSlidingWindow(cfg *SlidingWindowConfig) *SlidingWindow
```

Creates a new sliding window rate limiter. Passing `nil` uses defaults: WindowSize=1s, MaxRequests=100.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Allow` | `(ctx context.Context) bool` | Records the request if within the limit. Returns `true` if allowed, `false` otherwise. |
| `Wait` | `(ctx context.Context) error` | Blocks until a request slot is available or the context is cancelled. Polls at `WindowSize/10` intervals (minimum 1ms). |

---

## Package `breaker`

**Import**: `digital.vasic.concurrency/pkg/breaker`

### Constants

```go
const (
    Closed   State = iota // Normal operation
    Open                  // Failing -- requests rejected
    HalfOpen              // Probing -- limited requests allowed
)
```

### Types

#### `State`

```go
type State int
```

Represents the circuit breaker state.

| Method | Signature | Description |
|--------|-----------|-------------|
| `String` | `() string` | Returns `"closed"`, `"open"`, `"half-open"`, or `"unknown(N)"`. |

---

#### `Config`

```go
type Config struct {
    MaxFailures      int           // Failures before opening
    Timeout          time.Duration // How long to stay open
    HalfOpenRequests int           // Probe requests in half-open
}
```

#### `DefaultConfig`

```go
func DefaultConfig() *Config
```

Returns defaults: MaxFailures=5, Timeout=30s, HalfOpenRequests=1.

---

#### `CircuitBreaker`

Implements the circuit breaker pattern.

```go
type CircuitBreaker struct {
    // unexported fields
}
```

#### `New`

```go
func New(cfg *Config) *CircuitBreaker
```

Creates a new circuit breaker in the Closed state. Passing `nil` uses `DefaultConfig()`. `HalfOpenRequests` is forced to at least 1.

| Method | Signature | Description |
|--------|-----------|-------------|
| `State` | `() State` | Returns the current state, checking for automatic Open-to-HalfOpen transition. |
| `Execute` | `(fn func() error) error` | Calls `fn` if the circuit allows it. In Open state, returns `"circuit breaker is open"` without calling `fn`. In HalfOpen, allows up to `HalfOpenRequests` probe calls. In Closed, passes through. On failure in Closed, increments failure count; on reaching `MaxFailures`, transitions to Open. |
| `Reset` | `()` | Forces the breaker back to Closed state, resetting all counters. |
| `Failures` | `() int` | Returns the current consecutive failure count. |

---

## Package `semaphore`

**Import**: `digital.vasic.concurrency/pkg/semaphore`

### Types

#### `Semaphore`

A weighted semaphore for controlling concurrent access to a shared resource. Safe for concurrent use.

```go
type Semaphore struct {
    // unexported fields
}
```

#### `New`

```go
func New(maxWeight int64) *Semaphore
```

Creates a new semaphore with the given maximum weight. Values less than or equal to 0 are treated as 1.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Acquire` | `(ctx context.Context, weight int64) error` | Blocks until `weight` units are available or the context is cancelled. Returns an error if the requested weight exceeds `maxWeight` or the context is cancelled. Weight <= 0 returns `nil` immediately. |
| `TryAcquire` | `(weight int64) bool` | Attempts to acquire without blocking. Returns `true` if successful. |
| `Release` | `(weight int64)` | Returns `weight` units, potentially unblocking waiting goroutines. Weight <= 0 is a no-op. |
| `Current` | `() int64` | Returns the currently acquired weight. |
| `Available` | `() int64` | Returns `maxWeight - current`. |

---

## Package `monitor`

**Import**: `digital.vasic.concurrency/pkg/monitor`

### Types

#### `SystemResources`

Snapshot of system resource usage.

```go
type SystemResources struct {
    // CPU
    CPUPercent  float64
    CPUPerCore  []float64
    NumCPU      int

    // Memory
    MemoryTotal     uint64
    MemoryUsed      uint64
    MemoryAvailable uint64
    MemoryPercent   float64

    // Disk
    DiskTotal   uint64
    DiskUsed    uint64
    DiskFree    uint64
    DiskPercent float64

    // Load averages (Unix only)
    Load1  float64
    Load5  float64
    Load15 float64

    // Go runtime
    NumGoroutines int
    HeapAlloc     uint64
    HeapSys       uint64

    // Timestamp
    CollectedAt time.Time
}
```

| Field | Type | Description |
|-------|------|-------------|
| `CPUPercent` | `float64` | Overall CPU usage percentage (0-100). |
| `CPUPerCore` | `[]float64` | Per-core CPU usage percentages. |
| `NumCPU` | `int` | Number of logical CPUs (`runtime.NumCPU()`). |
| `MemoryTotal` | `uint64` | Total physical memory in bytes. |
| `MemoryUsed` | `uint64` | Used memory in bytes. |
| `MemoryAvailable` | `uint64` | Available memory in bytes. |
| `MemoryPercent` | `float64` | Memory usage percentage (0-100). |
| `DiskTotal` | `uint64` | Total disk space in bytes at the configured path. |
| `DiskUsed` | `uint64` | Used disk space in bytes. |
| `DiskFree` | `uint64` | Free disk space in bytes. |
| `DiskPercent` | `float64` | Disk usage percentage (0-100). |
| `Load1` | `float64` | 1-minute load average (Unix only, 0 on other platforms). |
| `Load5` | `float64` | 5-minute load average. |
| `Load15` | `float64` | 15-minute load average. |
| `NumGoroutines` | `int` | Number of active goroutines. |
| `HeapAlloc` | `uint64` | Heap memory currently allocated in bytes. |
| `HeapSys` | `uint64` | Heap memory obtained from the OS in bytes. |
| `CollectedAt` | `time.Time` | When this snapshot was taken. |

---

#### `Config`

```go
type Config struct {
    DiskPath        string
    CPUSampleTime   time.Duration
    CollectInterval time.Duration
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DiskPath` | `string` | `"/"` | Filesystem path to check disk usage. |
| `CPUSampleTime` | `time.Duration` | `200ms` | Duration to sample CPU usage. Longer values give more accurate readings. |
| `CollectInterval` | `time.Duration` | `5s` | Interval for background collection when using `Start`. |

#### `DefaultConfig`

```go
func DefaultConfig() *Config
```

Returns defaults: DiskPath="/", CPUSampleTime=200ms, CollectInterval=5s.

---

#### `ResourceMonitor`

Collects system resource snapshots.

```go
type ResourceMonitor struct {
    // unexported fields
}
```

#### `New`

```go
func New(cfg *Config) *ResourceMonitor
```

Creates a new resource monitor. Passing `nil` uses `DefaultConfig()`.

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetSystemResources` | `(ctx context.Context) (*SystemResources, error)` | Collects and returns a snapshot. Performs real I/O; takes up to `CPUSampleTime` to complete. Individual metric failures are tolerated (partial results returned). |
| `Start` | `(ctx context.Context) error` | Begins background collection at `CollectInterval`. Performs one immediate collection. Returns an error if the initial collection fails. |
| `Stop` | `()` | Halts background collection. |
| `Latest` | `() *SystemResources` | Returns the most recently collected snapshot, or `nil` if no collection has occurred. |
