# Contributing

Thank you for your interest in contributing to `digital.vasic.concurrency`. This document describes the development workflow, coding standards, and submission process.

## Prerequisites

- Go 1.24 or later
- `gofmt` and `go vet` (included with Go)
- `golangci-lint` (recommended, for extended linting)

## Getting Started

Clone the repository and verify that all tests pass:

```bash
git clone <repository-url>
cd Concurrency
go build ./...
go test ./... -count=1 -race
```

## Development Workflow

### 1. Create a Branch

Use the following naming convention:

```
feat/<description>    # New feature
fix/<description>     # Bug fix
test/<description>    # Test additions or improvements
refactor/<description> # Code restructuring
docs/<description>    # Documentation changes
chore/<description>   # Maintenance tasks
```

Example:

```bash
git checkout -b feat/pool-priority-support
```

### 2. Make Changes

Follow the coding standards described below. Each package is a single file pair:

```
pkg/<package>/<package>.go       # Production code
pkg/<package>/<package>_test.go  # Tests
```

Do not split packages into multiple source files without prior discussion.

### 3. Format, Vet, and Test

Before committing, always run:

```bash
gofmt -w .
go vet ./...
go test ./... -count=1 -race
```

All three must pass cleanly.

### 4. Commit

Use Conventional Commits format:

```
<type>(<scope>): <short description>

[optional body]
```

**Types**: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`, `perf`

**Scope**: The package name (`pool`, `queue`, `limiter`, `breaker`, `semaphore`, `monitor`) or omit for cross-cutting changes.

Examples:

```
feat(pool): add task cancellation support
fix(breaker): correct half-open probe counting
test(limiter): add concurrent token bucket stress test
refactor(semaphore): simplify waiter notification loop
docs(monitor): add SystemResources field documentation
perf(queue): reduce allocation in Push path
```

### 5. Submit a Pull Request

- Keep PRs focused. One logical change per PR.
- Include a clear description of what changed and why.
- Ensure all CI checks pass.

## Coding Standards

### Go Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go) and standard `gofmt` formatting.
- Group imports: stdlib, third-party, internal (blank line separated).
- Keep lines at 100 characters or fewer where practical.

### Naming

| Scope | Convention | Example |
|-------|-----------|---------|
| Private | `camelCase` | `refillTokens` |
| Exported | `PascalCase` | `NewWorkerPool` |
| Constants | `PascalCase` | `Critical` |
| Acronyms | All caps | `CPU`, `ID`, `URL` |
| Receivers | 1-2 letters | `p` for pool, `cb` for circuit breaker |

### Error Handling

- Always check errors. Never use `_` to discard an error unless there is a documented reason.
- Wrap errors with context: `fmt.Errorf("failed to acquire semaphore: %w", err)`.
- Use `defer` for cleanup in functions that acquire resources.

### Concurrency

- All exported methods must be safe for concurrent use.
- Use `sync.Mutex` or `sync.RWMutex` for shared mutable state.
- Accept `context.Context` as the first parameter for blocking operations.
- Avoid `sync.WaitGroup` leaks: always pair `Add` with `Done` via `defer`.

### Thread Safety Requirement

Every exported function and method in this module must be safe for concurrent use from multiple goroutines. This is a hard requirement. If a new method cannot be made thread-safe without significant overhead, discuss the trade-off in the PR description.

### Testing

- Use `testify/assert` for non-fatal assertions and `testify/require` for fatal ones.
- Name tests `Test<Struct>_<Method>_<Scenario>`:
  - `TestWorkerPool_Submit_QueueFull`
  - `TestCircuitBreaker_Execute_OpensAfterMaxFailures`
- Prefer table-driven tests for methods with multiple input/output cases.
- Always run with `-race` to detect data races.
- Test both success and failure paths.
- Test concurrent access with multiple goroutines where applicable.

### Benchmarks

Place benchmarks in `tests/benchmark/` or alongside unit tests. Name them `Benchmark<Struct>_<Method>`:

```go
func BenchmarkPriorityQueue_Push(b *testing.B) {
    pq := queue.New[int](0)
    for i := 0; i < b.N; i++ {
        pq.Push(i, queue.Normal)
    }
}
```

## What to Contribute

### Good First Contributions

- Add tests for uncovered edge cases.
- Improve error messages.
- Fix typos or clarify documentation.
- Add benchmarks for existing operations.

### Feature Contributions

- Open an issue or discussion first to agree on the API surface.
- New packages should follow the same single-file-pair convention.
- New exported interfaces should be minimal (1-3 methods).
- All new code requires tests with race detection passing.

### Reporting Bugs

- Include the Go version (`go version`).
- Include a minimal reproducing test case.
- Describe the expected vs. actual behavior.

## Code Review Checklist

Reviewers will verify:

- [ ] All tests pass with `-race`.
- [ ] New code follows naming and formatting conventions.
- [ ] Exported methods are thread-safe.
- [ ] Error paths are tested.
- [ ] No new external dependencies added without discussion.
- [ ] Commit messages follow Conventional Commits format.
- [ ] Documentation is updated if exported API changed.

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.
