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

## Configuration & Environment

Concurrency primitives are configured via Go structs at construction time — there are no environment-variable knobs the library reads on its own, which is deliberate (`pkg/*` stays project-not-aware per CONST-051(B); the consuming service decides how to source its configuration).

| Knob                                     | Where it lives                          | Default                | Notes                                                                  |
|------------------------------------------|-----------------------------------------|------------------------|------------------------------------------------------------------------|
| `PoolConfig.Workers`                     | `pkg/pool.PoolConfig{}`                 | `runtime.NumCPU()`     | Set higher for I/O-bound workloads.                                    |
| `PoolConfig.QueueSize`                   | `pkg/pool.PoolConfig{}`                 | `1000`                 | Buffered task channel depth; saturating it makes `Submit` block.       |
| `PoolConfig.TaskTimeout`                 | `pkg/pool.PoolConfig{}`                 | `0` (no per-task)      | Wraps each `Task.Execute(ctx)` with a `context.WithTimeout`.           |
| `PoolConfig.ShutdownGrace`               | `pkg/pool.PoolConfig{}`                 | `5 * time.Second`      | Drain window before forced cancel during `Shutdown`.                   |
| `TokenBucketConfig.Rate`                 | `pkg/limiter.TokenBucketConfig{}`       | none                   | Tokens/sec refill rate; `0` disables refill (pure burst).              |
| `TokenBucketConfig.Capacity`             | `pkg/limiter.TokenBucketConfig{}`       | none                   | Burst capacity.                                                        |
| `Config.MaxFailures`                     | `pkg/breaker.Config{}`                  | `5`                    | Consecutive failures before `Open` transition.                         |
| `Config.Timeout`                         | `pkg/breaker.Config{}`                  | `30 * time.Second`     | How long to stay `Open` before promoting to `HalfOpen`.                |
| `Config.HalfOpenRequests`                | `pkg/breaker.Config{}`                  | `1`                    | Probe requests allowed in `HalfOpen` before fully reclosing.           |
| `Semaphore.New(weight)`                  | `pkg/semaphore.New(int64)`              | none                   | Total weight available; pass-through for `Acquire(ctx, n)`.            |

## Edge Cases & Operational Notes

- **WorkerPool stopping while submitting**: `Submit` returns an error after `Stop` is called — callers must check and propagate.
- **TokenBucket with `Rate=0`**: pure burst mode (no refill). Useful for test fixtures and "one-shot" credit windows; the round-243 Challenge runner uses this to assert deterministic accept/reject counts.
- **CircuitBreaker `HalfOpen` race**: a single successful probe closes the breaker; subsequent in-flight probes that finish *after* the close still count toward the success metrics (they don't reopen).
- **Priority queue ties**: items with the same priority pop in FIFO order within their priority class (heap stability provided by the secondary index in `pkg/queue.item`).
- **Weighted semaphore over-release**: `Release(n)` with `n > current` panics — callers must mirror Acquire/Release counts symmetrically (typically with `defer`).
- **No host power-management**: per CONST-033, no code path in this submodule may suspend/hibernate/poweroff the host. Verified by `challenges/scripts/no_suspend_calls_challenge.sh`.

## Anti-Bluff Posture (CONST-035, CONST-050, Article XI §11.9)

Every claim of correctness in this submodule MUST carry positive runtime evidence captured during execution. Metadata-only / configuration-only / absence-of-error / grep-based PASS without runtime evidence are defects.

| Layer                    | Mechanism                                                                                                  |
|--------------------------|------------------------------------------------------------------------------------------------------------|
| Pre-build                | `go vet ./...` + `go build ./...` — exits non-zero on any vet warning.                                     |
| Post-build               | `go test ./... -count=1 -race` — every package, race detector on.                                          |
| Runtime end-to-end       | `challenges/concurrency_describe_challenge.sh` exercises every primitive against bilingual fixtures.       |
| Paired mutation (anti-bluff guard) | `bash challenges/concurrency_describe_challenge.sh --anti-bluff-mutate` corrupts the SR fixture; expects exit 99 (= the runner correctly rejected the corrupted fixture, proving its assertions are real). |

### Reproduce the full pipeline

```bash
cd dependencies/vasic-digital/Concurrency
bash challenges/concurrency_describe_challenge.sh
# Evidence lands in challenges/.last-run/{01-vet,02-build,03-test,04-fixtures,05-runtime,06-mutation}.log
```

### Reproduce only the anti-bluff guard

```bash
bash challenges/concurrency_describe_challenge.sh --anti-bluff-mutate
echo "exit=$?"   # 99 means anti-bluff guard verified
```

## Test-Type Coverage

The full CONST-050(B) test-type coverage matrix for this submodule lives at [`docs/test-coverage.md`](docs/test-coverage.md). Summary: unit / integration / E2E / security / DDoS / scaling / chaos / stress / UI / UX / Challenges are `covered`; performance / benchmarking / HelixQA-enrolment are `planned`.

## Governance

This submodule inherits every clause of the HelixConstitution submodule (`constitution/Constitution.md`, `constitution/CLAUDE.md`, `constitution/AGENTS.md`). The full list of cascaded clauses (CONST-033 through CONST-061) and submodule-specific addenda lives in this repo's `CLAUDE.md`, `AGENTS.md`, and `CONSTITUTION.md`.

The non-negotiable forensic anchor (verbatim 2026-04-29 operator mandate, reasserted multiple times across 2026-05):

> *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"*

The bar for shipping is not "tests pass" but "users can use the feature."
