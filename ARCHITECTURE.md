# Architecture -- Concurrency

## Purpose

Generic, reusable Go module for concurrency primitives: worker pools with task batching, generic priority queues, token bucket and sliding window rate limiters, circuit breakers with automatic recovery, weighted semaphores, and system resource monitoring.

## Structure

```
pkg/
  pool/        Worker pool with task submission, batching, parallel execution, and Map[T]
  queue/       Generic thread-safe priority task queue with 4 priority levels
  limiter/     Token bucket and sliding window rate limiters
  breaker/     Circuit breaker (closed/open/half-open states)
  semaphore/   Weighted semaphore for resource access control with context support
  monitor/     System resource monitoring (CPU, memory, disk) via gopsutil
```

## Key Components

- **`pool.WorkerPool`** -- Bounded concurrency with configurable workers, queue size, task timeout, and shutdown grace period. Supports Submit, SubmitWait, SubmitBatch, and parallel Map[T]
- **`queue.PriorityQueue[T]`** -- Generic heap-based priority queue with Critical/High/Normal/Low levels. Thread-safe with Push/Pop/Peek
- **`limiter.TokenBucket`** -- Smooth rate limiting with configurable rate and burst capacity
- **`limiter.SlidingWindow`** -- Time-window-based request counting for accurate rate limiting
- **`breaker.CircuitBreaker`** -- Fault tolerance with closed/open/half-open state machine, configurable failure threshold and recovery timeout
- **`semaphore.Semaphore`** -- Weighted resource access control with context-aware Acquire/Release
- **`monitor.ResourceMonitor`** -- System resource snapshots (CPU, memory, disk usage)

## Data Flow

```
Task submission: Submit(task) -> internal queue -> worker goroutine -> Execute(ctx) -> result channel

Rate limiting: Allow(ctx) -> token bucket or sliding window check -> allowed/denied

Circuit breaker: Execute(fn) -> check state
    Closed -> run fn -> success/failure counter
    Open -> reject immediately (until timeout)
    HalfOpen -> allow limited requests -> transition to Closed or Open
```

## Dependencies

- `github.com/shirou/gopsutil/v3` -- System resource monitoring
- `github.com/stretchr/testify` -- Test assertions

## Testing Strategy

Table-driven tests with `testify` and race detection. Tests cover concurrent task execution, priority ordering, rate limiter accuracy, circuit breaker state transitions, semaphore weight limits, and resource monitor sampling. Benchmarks available for performance-critical paths.
