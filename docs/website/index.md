# Concurrency Module

`digital.vasic.concurrency` is a focused library of concurrency primitives for Go. Every package is independently usable, thread-safe by default, and context-aware for cancellation and timeout propagation.

## Key Features

- **Worker pool** -- Bounded concurrency with task queuing, batching, and metrics. Includes a generic `Map[T, R]` function for type-safe parallel processing.
- **Priority queue** -- Generic `PriorityQueue[T]` using a min-heap with FIFO ordering within the same priority level
- **Rate limiting** -- Token bucket and sliding window implementations behind a common `RateLimiter` interface
- **Circuit breaker** -- Three-state machine (Closed, Open, HalfOpen) with configurable failure thresholds and automatic recovery
- **Weighted semaphore** -- Limit concurrent access to resources with per-operation weight
- **Resource monitor** -- System resource snapshots (CPU, memory, disk, Go runtime stats) via `gopsutil`

## Package Overview

| Package | Purpose |
|---------|---------|
| `pkg/pool` | Worker pool with task submission, batching, and parallel map |
| `pkg/queue` | Generic thread-safe priority queue |
| `pkg/limiter` | Rate limiting (token bucket, sliding window) |
| `pkg/breaker` | Circuit breaker with Closed/Open/HalfOpen states |
| `pkg/semaphore` | Weighted semaphore for resource access control |
| `pkg/monitor` | System resource monitoring (CPU, memory, disk) |
| `pkg/retry` | Retry logic with configurable backoff |
| `pkg/bulkhead` | Bulkhead isolation pattern |

## Installation

```bash
go get digital.vasic.concurrency
```

Requires Go 1.24 or later.

## Dependencies

Minimal external dependencies: only `gopsutil` (for the `monitor` package) and `testify` (for tests). All other packages depend solely on the Go standard library.
