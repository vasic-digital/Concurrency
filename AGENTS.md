# AGENTS.md - Multi-Agent Coordination Guide

## Overview

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working with the `digital.vasic.concurrency` module. It describes conventions, coordination patterns, and boundaries that agents must respect.

## Module Identity

- **Module path**: `digital.vasic.concurrency`
- **Language**: Go 1.24+
- **Dependencies**: `github.com/shirou/gopsutil/v3` (monitor), `github.com/stretchr/testify` (tests)
- **Scope**: Generic, reusable concurrency primitives. No application-specific logic.

## Package Responsibilities

| Package | Owner Concern | Agent Must Not |
|---------|--------------|----------------|
| `pkg/pool` | Worker pool lifecycle, task execution, metrics | Add provider-specific logic |
| `pkg/queue` | Priority ordering, generic type safety | Break heap invariants |
| `pkg/limiter` | Rate limiting algorithms (token bucket, sliding window) | Introduce external state stores |
| `pkg/breaker` | Circuit breaker state machine | Add persistent state |
| `pkg/semaphore` | Weighted resource access control | Remove context cancellation support |
| `pkg/monitor` | System resource snapshots via gopsutil | Add non-system metrics |

## Coordination Rules

### 1. Single-Package Changes

When modifying a single package, the agent owns that package for the duration of the task. No coordination with other agents is needed unless the change affects an exported interface.

### 2. Cross-Package Changes

If a change affects an exported type or interface (e.g., `pool.Task`, `limiter.RateLimiter`), the agent must:

1. Verify all consumers of the interface within the module.
2. Update all affected packages in a single commit.
3. Run `go test ./... -race` to confirm no regressions.

### 3. Interface Contracts

These interfaces are stability boundaries. Breaking changes require explicit human approval:

- `pool.Task` -- `ID() string` and `Execute(ctx) (interface{}, error)`
- `limiter.RateLimiter` -- `Allow(ctx) bool` and `Wait(ctx) error`

### 4. Thread Safety Invariants

Every exported method in every package is safe for concurrent use. Agents must:

- Never remove mutex protection from shared state.
- Never introduce a public method that requires external synchronization.
- Always run `go test -race` after changes.

### 5. Test Requirements

- All tests use `testify/assert` and `testify/require`.
- Test naming convention: `Test<Struct>_<Method>_<Scenario>`.
- Table-driven tests are preferred.
- Race detector must pass: `go test ./... -race`.

## Agent Workflow

### Before Making Changes

```bash
# Verify the module builds and tests pass
go build ./...
go test ./... -count=1 -race
```

### After Making Changes

```bash
# Format, vet, and test
gofmt -w .
go vet ./...
go test ./... -count=1 -race
```

### Commit Convention

```
<type>(<package>): <description>

# Examples:
feat(pool): add task priority support
fix(breaker): correct half-open transition timing
test(limiter): add sliding window edge case coverage
refactor(semaphore): simplify waiter notification
docs(monitor): update SystemResources field descriptions
```

## Boundaries

### What Agents May Do

- Fix bugs in any package.
- Add tests for uncovered code paths.
- Refactor internals without changing exported APIs.
- Add new exported methods that extend existing types.
- Update documentation to match code.

### What Agents Must Not Do

- Break existing exported interfaces or method signatures.
- Remove thread safety guarantees.
- Add application-specific logic (this is a generic library).
- Introduce new external dependencies without human approval.
- Modify `go.mod` without explicit instruction.
- Create mocks or stubs in production code.

## File Layout Convention

```
pkg/<package>/
    <package>.go        # All production code
    <package>_test.go   # All tests
```

Each package is a single file pair. Agents should maintain this convention and not split packages into multiple source files without human approval.

## Conflict Resolution

If two agents need to modify the same package concurrently:

1. The agent with the narrower scope (e.g., bug fix) takes priority.
2. The agent with the broader scope (e.g., refactor) should wait or rebase.
3. When in doubt, ask the human operator.

## Integration with HelixAgent

This module is consumed by the parent HelixAgent project as a Go module dependency. Agents working on HelixAgent should import packages via:

```go
import (
    "digital.vasic.concurrency/pkg/pool"
    "digital.vasic.concurrency/pkg/breaker"
    // etc.
)
```

Changes to this module's exported API will require corresponding updates in HelixAgent consumers.


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


