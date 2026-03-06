# Course: Go Concurrency Primitives

## Module Overview

This course covers the `digital.vasic.concurrency` module, a focused library of concurrency primitives for Go. You will learn to build worker pools, priority queues, rate limiters (token bucket and sliding window), circuit breakers, weighted semaphores, a retry mechanism, a bulkhead pattern, and a system resource monitor. Every component is thread-safe, context-aware, and has zero external state.

## Prerequisites

- Solid understanding of Go goroutines, channels, and `sync` package
- Familiarity with `context.Context` for cancellation and timeouts
- Basic knowledge of concurrency patterns (producer-consumer, semaphore)
- Go 1.24+ installed

## Lessons

| # | Title | Duration |
|---|-------|----------|
| 1 | Worker Pool with Bounded Parallelism | 45 min |
| 2 | Priority Queue and Rate Limiters | 50 min |
| 3 | Circuit Breaker and Semaphore | 45 min |
| 4 | Retry, Bulkhead, and Resource Monitor | 40 min |

## Source Files

- `pkg/pool/` -- Worker pool with task queue, results, and generic Map
- `pkg/queue/` -- Generic priority queue using min-heap
- `pkg/limiter/` -- Token bucket and sliding window rate limiters
- `pkg/breaker/` -- Circuit breaker (Closed/Open/HalfOpen state machine)
- `pkg/semaphore/` -- Weighted semaphore with waiter queue
- `pkg/retry/` -- Configurable retry with exponential backoff
- `pkg/bulkhead/` -- Bulkhead pattern for failure isolation
- `pkg/monitor/` -- System resource monitor (CPU, memory, disk, load)
