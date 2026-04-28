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



<!-- BEGIN host-power-management addendum (CONST-033) -->

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

<!-- END host-power-management addendum (CONST-033) -->



## Sixth Law — Real User Verification (Anti-Pseudo-Test Rule)

> Inherits from the root project's Anti-Bluff Testing Pact and the cross-project
> universal mandate (CONST-035). Submodule rules below are additive, never
> relaxing.

A test that passes while the feature it covers is broken for end users is the
most expensive kind of test in this codebase — it converts unknown breakage into
believed safety. This has happened in consuming projects before: tests and
Integration Challenge Tests executed green while large parts of the product
were unusable on a real device. That outcome is a constitutional failure, not a
coverage failure, and it MUST NOT recur in any module that depends on or is
depended on by this one.

Every test added MUST satisfy ALL of the following. Violation of any of them is
a release blocker, irrespective of coverage metrics, CI status, reviewer
sign-off, or schedule pressure.

1. **Same surfaces the user touches.** The test must traverse the production
   code path the user's action triggers, end to end, with no shortcut that
   bypasses real wiring.

2. **Provably falsifiable on real defects.** Before merging, the author MUST
   run the test once with the underlying feature deliberately broken (throw
   inside the function, return the wrong row, return the wrong status) and
   confirm the test fails with a clear assertion message. The PR description
   MUST state which deliberate break was used and what failure the test
   produced. A test that cannot be made to fail by breaking the thing it claims
   to verify is a bluff test by definition.

3. **Primary assertion on user-visible state.** The chief failure signal MUST
   be on something a real consumer could see or measure: rendered output,
   persisted database row, HTTP response body / status / header, file written
   to disk, packet on the wire. "Mock was invoked N times" is a permitted
   secondary assertion, never the primary one.

4. **Integration / Challenge tests are the load-bearing acceptance gate.** A
   green Challenge Test means a real consumer can complete the flow against
   real services — not "the wiring compiles". A feature for which a Challenge
   Test cannot be written is, by definition, not shippable.

5. **CI green is necessary, not sufficient.** Before any release tag is cut, a
   human (or a scripted black-box runner) MUST have exercised the feature
   end-to-end and observed the user-visible outcome.

6. **Inheritance.** This rule applies recursively to every consumer of this
   submodule. Consumer constitutions MAY add stricter rules but MUST NOT relax
   this one.


<!-- BEGIN anti-bluff-testing addendum (Article XI) -->

## Article XI — Anti-Bluff Testing (MANDATORY)

**Inherited from the umbrella project's Constitution Article XI.
Tests and Challenges that pass without exercising real end-user
behaviour are forbidden in this submodule too.**

Every test, every Challenge, every HelixQA bank entry MUST:

1. **Assert on a concrete end-user-visible outcome** — rendered DOM,
   DB rows that a real query would return, files on disk, media that
   actually plays, search results that actually contain expected
   items. Not "no error" or "200 OK".
2. **Run against the real system below the assertion.** Mocks/stubs
   are permitted ONLY in unit tests (`*_test.go` under `go test
   -short` or language equivalent). Integration / E2E / Challenge /
   HelixQA tests use real containers, real databases, real
   renderers. Unreachable real-system → skip with `SKIP-OK:
   #<ticket>`, never silently pass.
3. **Include a matching negative.** Every positive assertion is
   paired with an assertion that fails when the feature is broken.
4. **Emit copy-pasteable evidence** — body, screenshot, frame, DB
   row, log excerpt. Boolean pass/fail is insufficient.
5. **Verify "fails when feature is removed."** Author runs locally
   with the feature commented out; the test MUST FAIL. If it still
   passes, it's a bluff — delete and rewrite.
6. **No blind shells.** No `&& echo PASS`, `|| true`, `tee` exit
   laundering, `if [ -f file ]` without content assertion.

**Challenges in this submodule** must replay the user journey
end-to-end through the umbrella project's deliverables — never via
raw `curl` or third-party scripts. Sub-1-second Challenges almost
always indicate a bluff.

**HelixQA banks** declare executable actions
(`adb_shell:`, `playwright:`, `http:`, `assertVisible:`,
`assertNotVisible:`), never prose. Stagnation guard from Article I
§1.3 applies — frame N+1 identical to frame N for >10 s = FAIL.

**PR requirement:** every PR adding/modifying a test or Challenge in
this submodule MUST include a fenced `## Anti-Bluff Verification`
block with: (a) command run, (b) pasted output, (c) proof the test
fails when the feature is broken (second run with feature
commented-out showing FAIL).

**Cross-reference:** umbrella `CONSTITUTION.md` Article XI
(§§ 11.1 — 11.8).

<!-- END anti-bluff-testing addendum (Article XI) -->
