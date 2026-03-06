# Lesson 4: Retry, Bulkhead, and Resource Monitor

## Learning Objectives

- Implement configurable retry with exponential backoff and context cancellation
- Apply the bulkhead pattern for failure isolation between subsystems
- Monitor system resources (CPU, memory, disk) using `gopsutil` with graceful degradation

## Key Concepts

- **Retry with Backoff**: Configurable maximum attempts, base delay, and maximum delay. The backoff doubles on each attempt up to the cap. Context cancellation is checked between retries.
- **Bulkhead Pattern**: Isolates failures by partitioning resources into independent compartments. Each compartment has its own concurrency limit, preventing one failing subsystem from consuming all available resources.
- **Resource Monitor**: Wraps `gopsutil/v3` to collect CPU, memory, disk, and load averages. Supports both one-shot and periodic background collection. `RWMutex` protects the latest snapshot for concurrent readers.
- **Graceful Degradation**: If any individual metric collection fails, the others still populate. Only a total failure returns an error. Go runtime stats (goroutines, heap) are collected alongside OS metrics.

## Code Walkthrough

### Source: `pkg/retry/retry.go`

The retry function signature accepts a context and a function to retry:

```go
func Do(ctx context.Context, config Config, fn func() error) error
```

Each attempt calls `fn()`. On failure, the backoff is computed as `BaseDelay * 2^attempt`, capped at `MaxDelay`. Between attempts, the function checks `ctx.Done()` to respect cancellation and deadlines.

### Source: `pkg/bulkhead/bulkhead.go`

The bulkhead uses a semaphore (buffered channel) to limit concurrent access to a partition:

```go
type Bulkhead struct {
    sem chan struct{}
    // ...
}
```

Each `Execute` call attempts to acquire a slot. If the bulkhead is full, the call can either block (with context timeout) or return immediately with an error, depending on configuration.

### Source: `pkg/monitor/monitor.go`

The monitor collects system resources:

```go
type SystemResources struct {
    CPUPercent    float64
    MemoryPercent float64
    DiskPercent   float64
    LoadAvg1     float64
    LoadAvg5     float64
    LoadAvg15    float64
    NumGoroutines int
    HeapAlloc     uint64
    HeapSys       uint64
}
```

Background collection writes to `latest` under a write lock. `Latest()` reads under a read lock, allowing multiple concurrent readers without blocking.

### Source: `pkg/monitor/monitor_test.go`

Tests verify metric collection returns reasonable values, background collection updates the snapshot, and partial failures still return valid results for other metrics.

## Practice Exercise

1. Write a retry wrapper for an HTTP request that returns 503. Configure 5 attempts with 100ms base delay. Verify the total elapsed time matches the exponential backoff schedule.
2. Create two bulkheads: "database" (max 5 concurrent) and "external-api" (max 3 concurrent). Simulate one subsystem timing out and verify the other continues operating normally.
3. Use the resource monitor to collect system stats every second for 10 seconds. Log CPU, memory, and goroutine count. Verify that the monitor goroutine stops cleanly when Close is called.
