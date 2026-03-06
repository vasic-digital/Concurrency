# FAQ

## Are all packages thread-safe?

Yes. Every package uses appropriate synchronization: `sync.Mutex` or `sync.RWMutex` for state, `sync/atomic` for counters, and channels for task flow. All exported methods are safe for concurrent use from multiple goroutines.

## Can I use packages independently?

Yes. The packages have zero dependencies on each other. Import only what you need -- for example, you can use `pkg/breaker` without pulling in `pkg/pool` or `pkg/monitor`.

## What happens when the worker pool queue is full?

When the task buffer is full, `Submit` returns an error immediately rather than blocking. This gives the caller control over backpressure -- you can retry, drop the task, or queue it elsewhere.

## How does the circuit breaker recover?

The circuit breaker uses three states. When consecutive failures reach `MaxFailures`, the breaker transitions from Closed to Open. After `ResetTimeout` elapses, it moves to HalfOpen and allows `HalfOpenRequests` probe calls through. If those succeed, the breaker returns to Closed. If they fail, it reopens. Transitions are lazy (checked on each `Execute` call) rather than timer-driven.

## What is the difference between the token bucket and sliding window rate limiters?

The **token bucket** allows short bursts up to the bucket capacity and refills at a steady rate. It is best when you want to smooth traffic while tolerating occasional spikes. The **sliding window** enforces a strict request count within a rolling time window. It is best when you need hard limits with no burst allowance. Both implement the `RateLimiter` interface and can be swapped without changing consumer code.
