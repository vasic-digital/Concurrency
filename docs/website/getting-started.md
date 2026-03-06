# Getting Started

## Installation

```bash
go get digital.vasic.concurrency
```

## Worker Pool

Submit tasks to a bounded worker pool with configurable parallelism:

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.concurrency/pkg/pool"
)

func main() {
    cfg := &pool.Config{
        Workers:   4,
        QueueSize: 100,
    }
    wp := pool.NewWorkerPool(cfg)

    // Submit a task using the function adapter
    task := pool.NewTaskFunc("greet", func(ctx context.Context) (interface{}, error) {
        return "Hello, World!", nil
    })

    if err := wp.Submit(task); err != nil {
        panic(err)
    }

    // Read results
    result := <-wp.Results()
    fmt.Println(result.Value) // "Hello, World!"

    // Graceful shutdown
    wp.Shutdown(context.Background())
}
```

## Rate Limiting

Use either token bucket or sliding window rate limiting:

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.concurrency/pkg/limiter"
)

func main() {
    // Token bucket: 10 tokens capacity, 5 tokens/sec refill
    tb := limiter.NewTokenBucket(10, 5.0)

    if tb.Allow(context.Background()) {
        fmt.Println("Request allowed")
    }

    // Sliding window: 100 requests per 60 seconds
    sw := limiter.NewSlidingWindow(100, 60)

    if sw.Allow(context.Background()) {
        fmt.Println("Request allowed")
    }

    // Both implement the RateLimiter interface:
    // Wait blocks until a token is available
    sw.Wait(context.Background())
}
```

## Circuit Breaker

Protect against cascading failures:

```go
package main

import (
    "fmt"
    "net/http"
    "time"

    "digital.vasic.concurrency/pkg/breaker"
)

func main() {
    cb := breaker.New(breaker.Config{
        MaxFailures:     5,
        ResetTimeout:    30 * time.Second,
        HalfOpenRequests: 2,
    })

    err := cb.Execute(func() error {
        resp, err := http.Get("https://api.example.com/data")
        if err != nil {
            return err
        }
        resp.Body.Close()
        return nil
    })

    if err != nil {
        fmt.Println("Call failed or circuit is open:", err)
    }

    fmt.Println("State:", cb.State()) // Closed, Open, or HalfOpen
}
```

## Priority Queue

Process items by priority with FIFO ordering within the same level:

```go
package main

import (
    "fmt"

    "digital.vasic.concurrency/pkg/queue"
)

func main() {
    pq := queue.NewPriorityQueue[string]()

    pq.Push("low priority task", queue.Low)
    pq.Push("critical task", queue.Critical)
    pq.Push("normal task", queue.Normal)

    // Items come out in priority order
    item, ok := pq.Pop()
    if ok {
        fmt.Println(item) // "critical task"
    }

    fmt.Println("Queue size:", pq.Len())
}
```
