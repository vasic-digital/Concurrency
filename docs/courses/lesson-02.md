# Lesson 2: Priority Queue and Rate Limiters

## Learning Objectives

- Implement a generic priority queue backed by a min-heap with stable FIFO ordering
- Build a token bucket rate limiter with lazy refill for smooth burst control
- Build a sliding window rate limiter for strict count-based limiting

## Key Concepts

- **Min-Heap with Sequence Numbers**: `PriorityQueue[T]` uses `container/heap` with composite ordering: primary sort by priority (descending), secondary sort by insertion sequence number (ascending). The atomic `uint64` counter ensures stable FIFO within a priority level.
- **Token Bucket (Lazy Refill)**: Tokens are calculated on demand based on elapsed wall-clock time, eliminating the need for a background refill goroutine. Float64 tokens allow smooth sub-second refill rates.
- **Sliding Window (Lazy Cleanup)**: Maintains a slice of timestamps. On each `Allow` call, timestamps outside the window are pruned. No background cleanup needed.
- **Common Interface**: Both limiters implement `RateLimiter` with `Allow(ctx) bool` and `Wait(ctx) error`, making them interchangeable.

## Code Walkthrough

### Source: `pkg/queue/queue.go`

The priority queue uses Go generics and four priority levels (`Low(0)`, `Normal(1)`, `High(2)`, `Critical(3)`):

```go
type PriorityQueue[T any] struct {
    heap *innerHeap[T]
    mu   sync.Mutex
    seq  uint64 // atomic sequence counter
}
```

All public methods lock a `sync.Mutex`. The sequence counter assigns monotonically increasing numbers so that items at the same priority level are dequeued in FIFO order.

### Source: `pkg/limiter/limiter.go`

The token bucket refills lazily:

```go
func (tb *TokenBucket) Allow(ctx context.Context) bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    tb.refill() // calculates tokens based on elapsed time
    if tb.tokens >= 1 {
        tb.tokens--
        return true
    }
    return false
}
```

The sliding window prunes expired timestamps on each call:

```go
func (sw *SlidingWindow) Allow(ctx context.Context) bool {
    sw.mu.Lock()
    defer sw.mu.Unlock()
    sw.cleanup() // removes timestamps outside the window
    if len(sw.timestamps) < sw.maxRequests {
        sw.timestamps = append(sw.timestamps, time.Now())
        return true
    }
    return false
}
```

### Source: `pkg/limiter/limiter_test.go`

Tests verify:
- Token bucket burst allowance and steady-state rate
- Sliding window strict count enforcement
- Wait blocks until a token is available or context cancels
- Concurrent access from multiple goroutines

## Practice Exercise

1. Create a `PriorityQueue[string]` and push 10 items at Mixed priorities (Critical, Normal, Low). Pop all items and verify they come out in priority order, with FIFO within each level.
2. Configure a token bucket with rate=5 tokens/sec and capacity=10. Issue 10 rapid requests (all should succeed as burst), then issue 6 more (only 5 should succeed over the next second).
3. Compare token bucket vs. sliding window: write a benchmark that issues 1000 `Allow` calls and measure the throughput difference.
