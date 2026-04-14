# CLAUDE.md - Concurrency Module

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


## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**

