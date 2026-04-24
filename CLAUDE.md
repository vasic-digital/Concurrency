# CLAUDE.md - Concurrency Module


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

<!-- TODO: replace this block with the exact command(s) that exercise this
     module end-to-end against real dependencies, and the expected output.
     The commands must run the real artifact (built binary, deployed
     container, real service) — no in-process fakes, no mocks, no
     `httptest.NewServer`, no Robolectric, no JSDOM as proof of done. -->

```bash
# TODO
```

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
- **NEVER** use `su` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Container-Based Solutions
When a build or runtime environment requires system-level dependencies, use containers instead of elevation:

- **Use the `Containers` submodule** (`https://github.com/vasic-digital/Containers`) for containerized build and runtime environments
- **Add the `Containers` submodule as a Git dependency** and configure it for local use within the project
- **Build and run inside containers** to avoid any need for privilege escalation
- **Rootless Podman/Docker** is the preferred container runtime

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo` or `su`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Use the `Containers` submodule for containerized builds
5. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | BackgroundTasks, HelixLLM |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
