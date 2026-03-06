# Lesson 3: Circuit Breaker and Semaphore

## Learning Objectives

- Implement the circuit breaker pattern with a three-state machine for cascading failure prevention
- Build a weighted semaphore with a waiter queue and context cancellation
- Understand lazy state transitions and probe-based recovery

## Key Concepts

- **Circuit Breaker States**: Closed (normal), Open (fast-fail), HalfOpen (probing). Transitions are driven by consecutive failure counts and timeout expiration.
- **Consecutive Failure Tracking**: The failure counter resets to zero on any success in the Closed state. Intermittent failures do not accumulate.
- **Lazy State Transition**: The transition from Open to HalfOpen is checked at the start of each `Execute` call, not by a background timer. This eliminates unnecessary goroutines.
- **Weighted Semaphore**: Different operations consume different amounts of capacity. Waiters are queued with channels and notified when capacity becomes available.

## Code Walkthrough

### Source: `pkg/breaker/breaker.go`

The state machine:

```
[Closed] ---(MaxFailures reached)---> [Open]
[Open]   ---(Timeout elapsed)-------> [HalfOpen]
[HalfOpen] ---(probe succeeds)------> [Closed]
[HalfOpen] ---(probe fails)---------> [Open]
```

The `Execute` method checks state, optionally transitions, runs the function, then updates counters. The breaker mutex is held for state checks but released during `fn()` execution to avoid blocking other callers.

`HalfOpen` allows only `HalfOpenRequests` calls through. Additional calls are rejected, preventing a flood of requests to a recovering service.

### Source: `pkg/semaphore/semaphore.go`

The weighted semaphore uses a waiter queue:

```go
type Semaphore struct {
    mu      sync.Mutex
    current int64
    max     int64
    waiters []*waiter
}
```

Each blocked `Acquire` call creates a `waiter` struct with a `chan struct{}` that is closed when the waiter can proceed. `notifyWaiters` scans front-to-back and wakes any waiter whose requested weight fits. `TryAcquire` provides non-blocking acquisition for fallback scenarios.

### Source: `pkg/breaker/breaker_test.go` and `pkg/semaphore/semaphore_test.go`

Tests cover all state transitions, concurrent access patterns, context cancellation during semaphore wait, and manual breaker reset.

## Practice Exercise

1. Create a circuit breaker with `MaxFailures=3` and `ResetTimeout=1s`. Execute a function that fails 3 times, verify the circuit opens, wait 1 second, then verify HalfOpen allows a probe through.
2. Build a weighted semaphore with `max=10`. Acquire weight 7, then attempt to acquire weight 5 in a goroutine (should block). Release the first acquisition and verify the second proceeds.
3. Combine breaker and semaphore: protect an HTTP client with a circuit breaker and limit concurrent requests to 5 using the semaphore. Test with a flaky server mock.
