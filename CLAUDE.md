# CLAUDE.md - Concurrency Module

## INHERITED FROM constitution/CLAUDE.md

All rules in `constitution/CLAUDE.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. See parent root `CLAUDE.md` §6.AD for the Lava-specific incorporation context (29th §6.L cycle, 2026-05-14) and §6.AD-debt for the implementation-gap inventory. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution's `CLAUDE.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

## Definition of Done

### Acceptance demo for this module

```bash
# Worker pool, rate limiter, circuit breaker under load
cd Concurrency && GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v ./tests/integration/...
```
Expect: all integration tests PASS; exercises `pool.NewWorkerPool`, `queue.New[T]`, `limiter.NewTokenBucket`, `breaker.New`, `semaphore.New(10)` per `Concurrency/README.md` Quick Start.

## Overview

`digital.vasic.concurrency` is a generic, reusable Go module for concurrency primitives including worker pools, priority queues, rate limiters, circuit breakers, semaphores, and resource monitoring.

**Module**: `digital.vasic.concurrency` (Go 1.24+)

## Build & Test

```bash
go build ./...
go test ./... -count=1 -race
go test ./... -short              # Unit tests only
go test -tags=integration ./...   # Integration tests
go test -bench=. ./tests/benchmark/
```

## Code Style

- Standard Go conventions, `gofmt` formatting
- Imports grouped: stdlib, third-party, internal (blank line separated)
- Line length <= 100 chars
- Naming: `camelCase` private, `PascalCase` exported, acronyms all-caps
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`
- Tests: table-driven, `testify`, naming `Test<Struct>_<Method>_<Scenario>`

## Package Structure

| Package | Purpose |
|---------|---------|
| `pkg/pool` | Worker pool with task submission, batching, and parallel execution |
| `pkg/queue` | Generic thread-safe priority task queue |
| `pkg/limiter` | Rate limiting (token bucket, sliding window) |
| `pkg/breaker` | Circuit breaker (closed/open/half-open states) |
| `pkg/semaphore` | Weighted semaphore for resource access control |
| `pkg/monitor` | System resource monitoring (CPU, memory, disk) |

## Key Interfaces

- `pool.Task` — Unit of work with ID() and Execute(ctx)
- `pool.WorkerPool` — Bounded concurrency with configurable workers
- `queue.PriorityQueue[T]` — Generic priority queue with Push/Pop/Peek
- `limiter.RateLimiter` — Rate limiting with Allow(ctx) and Wait(ctx)
- `breaker.CircuitBreaker` — Fault tolerance with Execute(fn)
- `semaphore.Semaphore` — Weighted resource access with Acquire/Release
- `monitor.ResourceMonitor` — System resource snapshots

## Design Patterns

- **Worker Pool**: Bounded concurrency with task queuing and metrics
- **Priority Queue**: Heap-based ordering with generic type parameters
- **Token Bucket / Sliding Window**: Two rate limiting strategies
- **Circuit Breaker**: Fail-fast with automatic recovery
- **Semaphore**: Weighted resource access control

## Commit Style

Conventional Commits: `feat(pool): add batch submission support`

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | BackgroundTasks, HelixLLM |

*Siblings* means other project-owned modules at the parent project's repo root. The root project app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
