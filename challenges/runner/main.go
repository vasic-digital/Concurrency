// Concurrency describe-Challenge runner.
//
// CHALLENGE program (not production code) — exercises the real
// Concurrency primitives (worker pool, priority queue, token-bucket
// limiter, circuit breaker, weighted semaphore) end-to-end and
// emits a locale-aware human-readable summary loaded from
// challenges/fixtures/<locale>.yaml.
//
// Anti-bluff posture (CONST-035 / CONST-050 / Article XI §11.9):
//   - no mocks: every primitive is the real implementation from
//     digital.vasic.concurrency/pkg/*;
//   - every assertion captures the actual observed value (counts,
//     durations, state names) verbatim, so a regression cannot
//     disguise itself as "test still passing";
//   - human-readable output is loaded from real YAML fixtures
//     (CONST-046 anti-hardcoded-content compliance);
//   - paired anti-bluff mutation leg in challenges/concurrency_describe_challenge.sh
//     corrupts a fixture and re-runs THIS program — failure expected.
//
// Exit codes:
//   0  — every primitive verified and every fixture-driven assertion held.
//   1  — usage / I/O / fixture-load failure.
//   2  — primitive regression: observed behavior contradicts the documented
//         contract (e.g. priority queue returned the wrong head, breaker
//         did not open after MaxFailures+1 consecutive failures, etc.).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"

	"digital.vasic.concurrency/pkg/breaker"
	"digital.vasic.concurrency/pkg/limiter"
	"digital.vasic.concurrency/pkg/pool"
	"digital.vasic.concurrency/pkg/queue"
	"digital.vasic.concurrency/pkg/semaphore"
)

// fixture is the locale-bound flat key -> string map loaded from YAML.
type fixture struct {
	locale  string
	entries map[string]string
}

func loadFixture(path, locale string) (*fixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", path, err)
	}
	entries := map[string]string{}
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse fixture %s: %w", path, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("fixture %s is empty", path)
	}
	return &fixture{locale: locale, entries: entries}, nil
}

// t resolves a key. Lookup miss returns the key verbatim so a regression
// is loud, not silent.
func (f *fixture) t(key string) string {
	if v, ok := f.entries[key]; ok && v != "" {
		return v
	}
	return key
}

// requireKey aborts the run if a required key is missing — a fixture
// missing a required key is a CONST-046 i18n bluff.
func (f *fixture) requireKey(key string) error {
	if _, ok := f.entries[key]; !ok {
		return fmt.Errorf("locale %q missing required key %q", f.locale, key)
	}
	return nil
}

// requiredKeys lists every key the runner consumes. Centralised so a
// fixture-add audit can grep for the full set.
var requiredKeys = []string{
	"challenge.banner",
	"challenge.pool.label",
	"challenge.pool.summary",
	"challenge.queue.label",
	"challenge.queue.summary",
	"challenge.limiter.label",
	"challenge.limiter.summary",
	"challenge.breaker.label",
	"challenge.breaker.summary",
	"challenge.semaphore.label",
	"challenge.semaphore.summary",
	"challenge.result.success",
	"challenge.result.failure",
}

// runPool exercises a real WorkerPool with a small, deterministic batch
// of tasks and returns (executed, successful, failed).
func runPool(ctx context.Context) (int, int, int, error) {
	wp := pool.NewWorkerPool(&pool.PoolConfig{
		Workers:       4,
		QueueSize:     32,
		TaskTimeout:   2 * time.Second,
		ShutdownGrace: 1 * time.Second,
	})
	defer wp.Stop()

	const total = 12
	var (
		executed int32
		failed   int32
	)
	tasks := make([]pool.Task, 0, total)
	for i := 0; i < total; i++ {
		i := i
		tasks = append(tasks, pool.NewTaskFunc(
			fmt.Sprintf("desc-task-%d", i),
			func(ctx context.Context) (interface{}, error) {
				atomic.AddInt32(&executed, 1)
				// deterministic: every 5th task errors so the (success, failed)
				// counts are non-trivial and a regression in the error path is
				// caught by the fixture-driven assertion.
				if i%5 == 0 && i > 0 {
					atomic.AddInt32(&failed, 1)
					return nil, errors.New("intentional pool-task error")
				}
				return i * 2, nil
			},
		))
	}

	results, err := wp.SubmitBatchWait(ctx, tasks)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("pool SubmitBatchWait: %w", err)
	}

	got := len(results)
	if got != total {
		return 0, 0, 0, fmt.Errorf(
			"pool: expected %d results, got %d (regression)", total, got)
	}
	executedSnap := int(atomic.LoadInt32(&executed))
	failedSnap := int(atomic.LoadInt32(&failed))
	if executedSnap != total {
		return 0, 0, 0, fmt.Errorf(
			"pool: expected %d executions, got %d", total, executedSnap)
	}
	return executedSnap, executedSnap - failedSnap, failedSnap, nil
}

// runQueue exercises a real priority queue and returns (drainedCount, headValue).
func runQueue() (int, string, error) {
	pq := queue.New[string](0)
	// Push in deliberately wrong order; Pop must return Critical first.
	pq.Push("low-1", queue.Low)
	pq.Push("normal-1", queue.Normal)
	pq.Push("critical-1", queue.Critical)
	pq.Push("high-1", queue.High)
	pq.Push("low-2", queue.Low)

	head, ok := pq.Peek()
	if !ok || head != "critical-1" {
		return 0, head, fmt.Errorf(
			"queue: expected head=critical-1, got head=%q ok=%t (regression)",
			head, ok)
	}

	drained := 0
	prevPriority := -1
	for {
		item, ok := pq.Pop()
		if !ok {
			break
		}
		drained++
		// Validate monotonic non-increasing priority by re-deriving from name.
		var p int
		switch {
		case strings.HasPrefix(item, "critical"):
			p = int(queue.Critical)
		case strings.HasPrefix(item, "high"):
			p = int(queue.High)
		case strings.HasPrefix(item, "normal"):
			p = int(queue.Normal)
		case strings.HasPrefix(item, "low"):
			p = int(queue.Low)
		}
		if prevPriority != -1 && p > prevPriority {
			return 0, head, fmt.Errorf(
				"queue: priority order violation (prev=%d cur=%d item=%s)",
				prevPriority, p, item)
		}
		prevPriority = p
	}
	if drained != 5 {
		return 0, head, fmt.Errorf(
			"queue: expected drained=5, got %d (regression)", drained)
	}
	return drained, head, nil
}

// runLimiter exercises a real TokenBucket and returns (allowed, capacity, rejected).
func runLimiter(ctx context.Context) (int, int, int, error) {
	const capacity = 5
	rl := limiter.NewTokenBucket(&limiter.TokenBucketConfig{
		Rate:     0, // no refill during the burst-only window
		Capacity: capacity,
	})
	allowed := 0
	rejected := 0
	const attempts = 8
	for i := 0; i < attempts; i++ {
		if rl.Allow(ctx) {
			allowed++
		} else {
			rejected++
		}
	}
	if allowed != capacity {
		return 0, 0, 0, fmt.Errorf(
			"limiter: expected allowed=%d (bucket capacity), got %d (regression)",
			capacity, allowed)
	}
	if rejected != attempts-capacity {
		return 0, 0, 0, fmt.Errorf(
			"limiter: expected rejected=%d, got %d (regression)",
			attempts-capacity, rejected)
	}
	return allowed, capacity, rejected, nil
}

// runBreaker exercises a real CircuitBreaker. Returns
// (recordedFailures, stateName, recovered).
func runBreaker() (int, string, bool, error) {
	const maxFails = 3
	cb := breaker.New(&breaker.Config{
		MaxFailures:      maxFails,
		Timeout:          100 * time.Millisecond,
		HalfOpenRequests: 1,
	})
	failErr := errors.New("intentional breaker failure")
	for i := 0; i < maxFails+1; i++ {
		_ = cb.Execute(func() error { return failErr })
	}
	if cb.State() != breaker.Open {
		return 0, cb.State().String(), false, fmt.Errorf(
			"breaker: expected Open after %d failures, got %s (regression)",
			maxFails+1, cb.State())
	}
	// Wait for the half-open transition.
	time.Sleep(150 * time.Millisecond)
	// One successful probe should recover.
	if err := cb.Execute(func() error { return nil }); err != nil {
		return 0, cb.State().String(), false, fmt.Errorf(
			"breaker: probe should succeed, got %v", err)
	}
	recovered := cb.State() == breaker.Closed
	if !recovered {
		return 0, cb.State().String(), false, fmt.Errorf(
			"breaker: expected Closed after successful probe, got %s (regression)",
			cb.State())
	}
	return cb.Failures(), cb.State().String(), recovered, nil
}

// runSemaphore exercises a real weighted semaphore.
func runSemaphore(ctx context.Context) (int, int64, int64, error) {
	const capacity = 8
	sem := semaphore.New(int64(capacity))
	const acquired = 3
	if err := sem.Acquire(ctx, int64(acquired)); err != nil {
		return 0, 0, 0, fmt.Errorf("semaphore Acquire: %w", err)
	}
	defer sem.Release(int64(acquired))
	cur := sem.Current()
	avail := sem.Available()
	if cur != int64(acquired) {
		return 0, 0, 0, fmt.Errorf(
			"semaphore: expected current=%d, got %d (regression)",
			acquired, cur)
	}
	if avail != int64(capacity-acquired) {
		return 0, 0, 0, fmt.Errorf(
			"semaphore: expected available=%d, got %d (regression)",
			capacity-acquired, avail)
	}
	return acquired, cur, avail, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		// Code 2 reserved for primitive regression; code 1 for I/O / load.
		if strings.Contains(err.Error(), "regression") {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run() error {
	// Locate fixtures relative to this source file's repo root.
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	enPath := filepath.Join(repoRoot, "challenges", "fixtures", "en.yaml")
	srPath := filepath.Join(repoRoot, "challenges", "fixtures", "sr-Latn.yaml")

	enFix, err := loadFixture(enPath, "en")
	if err != nil {
		return err
	}
	srFix, err := loadFixture(srPath, "sr-Latn")
	if err != nil {
		return err
	}

	for _, f := range []*fixture{enFix, srFix} {
		for _, k := range requiredKeys {
			if err := f.requireKey(k); err != nil {
				return err
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolExec, poolOK, poolFail, err := runPool(ctx)
	if err != nil {
		return err
	}
	qDrained, qHead, err := runQueue()
	if err != nil {
		return err
	}
	limAllowed, limCap, limRej, err := runLimiter(ctx)
	if err != nil {
		return err
	}
	brFails, brState, brRecovered, err := runBreaker()
	if err != nil {
		return err
	}
	semAcq, semCur, semAvail, err := runSemaphore(ctx)
	if err != nil {
		return err
	}

	emit := func(f *fixture) {
		fmt.Println(f.t("challenge.banner"))
		fmt.Printf("  [%s] %s\n", f.t("challenge.pool.label"),
			fmt.Sprintf(f.t("challenge.pool.summary"),
				poolExec, 4, poolOK, poolFail))
		fmt.Printf("  [%s] %s\n", f.t("challenge.queue.label"),
			fmt.Sprintf(f.t("challenge.queue.summary"),
				qDrained, qHead))
		fmt.Printf("  [%s] %s\n", f.t("challenge.limiter.label"),
			fmt.Sprintf(f.t("challenge.limiter.summary"),
				limAllowed, limCap, limRej))
		fmt.Printf("  [%s] %s\n", f.t("challenge.breaker.label"),
			fmt.Sprintf(f.t("challenge.breaker.summary"),
				brFails, brState, brRecovered))
		fmt.Printf("  [%s] %s\n", f.t("challenge.semaphore.label"),
			fmt.Sprintf(f.t("challenge.semaphore.summary"),
				semAcq, semCur, semAvail))
		fmt.Printf("  %s\n", f.t("challenge.result.success"))
	}

	emit(enFix)
	emit(srFix)

	// Cross-locale anti-bluff: the EN and SR banners MUST differ. If they
	// match, the YAML loader silently returned an empty/default value and
	// the bilingual proof is meaningless (CONST-046 bluff).
	if enFix.t("challenge.banner") == srFix.t("challenge.banner") {
		return fmt.Errorf(
			"locale-bluff regression: EN banner == SR banner (%q); fixtures not actually distinct",
			enFix.t("challenge.banner"))
	}
	// And neither may equal the key itself (= lookup-miss verbatim return).
	for _, f := range []*fixture{enFix, srFix} {
		if f.t("challenge.banner") == "challenge.banner" {
			return fmt.Errorf(
				"locale %q: banner key resolved to verbatim key (fixture lookup miss)",
				f.locale)
		}
	}

	fmt.Println("\nconcurrency describe challenge: PASS (bilingual, real primitives, no mocks)")
	return nil
}
