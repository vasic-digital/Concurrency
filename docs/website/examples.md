# Examples

## Parallel Map with Generics

Use `Map[T, R]` for type-safe parallel processing of a slice:

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "digital.vasic.concurrency/pkg/pool"
)

func main() {
    urls := []string{
        "https://example.com/a",
        "https://example.com/b",
        "https://example.com/c",
    }

    // Process all URLs in parallel with 3 workers
    results, err := pool.Map(context.Background(), urls, 3, func(ctx context.Context, url string) (string, error) {
        // Simulate processing
        return strings.ToUpper(url), nil
    })
    if err != nil {
        panic(err)
    }

    for _, r := range results {
        fmt.Println(r)
    }
}
```

## Weighted Semaphore for Resource Control

Use the semaphore to limit concurrent access where different operations consume different capacity:

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    "digital.vasic.concurrency/pkg/semaphore"
)

func main() {
    // Maximum weight of 10 (e.g., 10 database connections)
    sem := semaphore.New(10)

    var wg sync.WaitGroup
    ctx := context.Background()

    // Light operation: weight 1
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            sem.Acquire(ctx, 1)
            defer sem.Release(1)
            fmt.Printf("Light task %d running\n", id)
            time.Sleep(100 * time.Millisecond)
        }(i)
    }

    // Heavy operation: weight 5
    wg.Add(1)
    go func() {
        defer wg.Done()
        sem.Acquire(ctx, 5)
        defer sem.Release(5)
        fmt.Println("Heavy task running")
        time.Sleep(200 * time.Millisecond)
    }()

    wg.Wait()

    // Non-blocking attempt
    if sem.TryAcquire(3) {
        fmt.Println("Got 3 units")
        sem.Release(3)
    }
}
```

## System Resource Monitoring

Collect CPU, memory, and disk metrics for adaptive behavior:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "digital.vasic.concurrency/pkg/monitor"
)

func main() {
    mon := monitor.New()

    // One-shot collection
    resources, err := mon.GetSystemResources()
    if err != nil {
        panic(err)
    }

    fmt.Printf("CPU Usage: %.1f%%\n", resources.CPUPercent)
    fmt.Printf("Memory: %.1f%%\n", resources.MemoryPercent)
    fmt.Printf("Goroutines: %d\n", resources.NumGoroutines)
    fmt.Printf("Heap: %d MB\n", resources.HeapAlloc/1024/1024)

    // Periodic background collection
    if err := mon.Start(context.Background(), 5*time.Second); err != nil {
        panic(err)
    }

    // Read the latest snapshot at any time
    time.Sleep(6 * time.Second)
    latest := mon.Latest()
    fmt.Printf("Latest CPU: %.1f%%\n", latest.CPUPercent)
}
```
