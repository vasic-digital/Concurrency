# Changelog

All notable changes to `digital.vasic.concurrency` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-01-01

### Added

- **pkg/pool**: Worker pool with bounded concurrency and configurable worker count.
  - `Task` interface and `TaskFunc` adapter for wrapping functions as tasks.
  - `Submit`, `SubmitWait`, `SubmitBatch`, `SubmitBatchWait` for task submission.
  - `PoolConfig` with workers, queue size, task timeout, shutdown grace, and callbacks (`OnError`, `OnComplete`).
  - `PoolMetrics` with atomic counters for active workers, queued/completed/failed tasks, and average latency.
  - `ParallelExecute` convenience function for one-off parallel execution.
  - `Map[T, R]` generic parallel map with bounded concurrency.
  - `WaitForDrain` to wait until all queued tasks are processed.
  - Graceful `Shutdown` with timeout and immediate `Stop`.

- **pkg/queue**: Generic, thread-safe priority queue.
  - `PriorityQueue[T]` backed by `container/heap` with stable FIFO ordering within priority levels.
  - Four priority levels: `Low`, `Normal`, `High`, `Critical`.
  - `Push`, `Pop`, `Peek`, `Len`, `IsEmpty` operations.
  - Pre-allocation support via `initialCap` parameter.

- **pkg/limiter**: Rate limiting with two strategies.
  - `RateLimiter` interface with `Allow(ctx)` and `Wait(ctx)`.
  - `TokenBucket`: smooth rate limiting with configurable rate and burst capacity. Lazy refill based on elapsed time.
  - `SlidingWindow`: time-window-based request counting with automatic cleanup of expired timestamps.

- **pkg/breaker**: Circuit breaker for fault tolerance.
  - Three-state machine: `Closed`, `Open`, `HalfOpen`.
  - `Execute(fn)` wraps function calls with circuit protection.
  - Configurable `MaxFailures`, `Timeout`, and `HalfOpenRequests`.
  - Automatic Open-to-HalfOpen transition after timeout.
  - Manual `Reset()` to force back to Closed state.

- **pkg/semaphore**: Weighted semaphore for resource access control.
  - `Acquire(ctx, weight)` with context cancellation support.
  - `TryAcquire(weight)` for non-blocking acquisition.
  - `Release(weight)` with automatic waiter notification.
  - `Current()` and `Available()` for capacity inspection.

- **pkg/monitor**: System resource monitoring via gopsutil.
  - `SystemResources` snapshot: CPU (overall and per-core), memory, disk, load averages, Go runtime stats (goroutines, heap).
  - `GetSystemResources(ctx)` for one-shot collection.
  - `Start(ctx)` / `Stop()` for background periodic collection.
  - `Latest()` for non-blocking access to most recent snapshot.
  - Configurable disk path, CPU sample time, and collection interval.

- Full test coverage for all packages with race detection.
- Thread safety for all exported methods across all packages.
