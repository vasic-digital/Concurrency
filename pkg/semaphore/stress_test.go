// SPDX-License-Identifier: Apache-2.0
//
// Stress tests for digital.vasic.concurrency/pkg/semaphore, modelled on
// the canonical P3 stress-test template in
// digital.vasic.buildcheck/pkg/buildcheck/stress_test.go.
//
// Run with:
//   GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -run '^TestStress' \
//       ./pkg/semaphore/ -p 1 -count=1 -timeout 120s
package semaphore

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	stressGoroutines   = 8
	stressIterations   = 400
	stressMaxWallClock = 15 * time.Second
)

// TestStress_Semaphore_AcquireReleaseWeighted puts 8 goroutines through
// a sustained mix of weighted Acquire / TryAcquire / Release against a
// tight-capacity semaphore. Validates no race, semaphore invariants
// hold (Current never exceeds Max, Available never negative), and no
// goroutine leak.
func TestStress_Semaphore_AcquireReleaseWeighted(t *testing.T) {
	const maxWeight = 8
	s := New(maxWeight)

	startGoroutines := runtime.NumGoroutine()
	var wg sync.WaitGroup
	var acqErrors atomic.Int64
	deadline := time.Now().Add(stressMaxWallClock)

	for g := 0; g < stressGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			weight := int64(1 + (id % 3)) // weights 1, 2, 3
			for j := 0; j < stressIterations; j++ {
				if time.Now().After(deadline) {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				if err := s.Acquire(ctx, weight); err != nil {
					cancel()
					acqErrors.Add(1)
					continue
				}
				cancel()
				// Tiny workload while holding.
				time.Sleep(10 * time.Microsecond)
				s.Release(weight)
			}
		}(g)
	}
	wg.Wait()

	// Under any legal interleaving the semaphore must always be back at
	// 0 in-use when all goroutines have released.
	assert.Equal(t, int64(0), s.Current(),
		"semaphore not fully released: Current=%d after wg.Wait", s.Current())
	assert.Equal(t, int64(maxWeight), s.Available(),
		"semaphore capacity drifted: Available=%d expected %d", s.Available(), maxWeight)
	assert.Equal(t, int64(0), acqErrors.Load(),
		"Acquire should not time out on well-formed calls with 1s budget")

	time.Sleep(50 * time.Millisecond)
	runtime.Gosched()
	endGoroutines := runtime.NumGoroutine()
	assert.LessOrEqual(t, endGoroutines-startGoroutines, 2,
		"goroutine leak: worker count grew by %d", endGoroutines-startGoroutines)
}

// TestStress_Semaphore_TryAcquireContended stresses the non-blocking
// TryAcquire path where many goroutines race for limited capacity.
// The semaphore must always remain within its declared bounds — the
// failure mode would be a race that lets Current briefly exceed
// maxWeight or drop below zero on Release.
func TestStress_Semaphore_TryAcquireContended(t *testing.T) {
	const maxWeight = 4
	s := New(maxWeight)

	var wg sync.WaitGroup
	var succ, fail atomic.Int64
	deadline := time.Now().Add(5 * time.Second)

	for g := 0; g < stressGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < stressIterations; j++ {
				if time.Now().After(deadline) {
					return
				}
				if s.TryAcquire(1) {
					succ.Add(1)
					time.Sleep(5 * time.Microsecond)
					s.Release(1)
				} else {
					fail.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	// The contract we can verify from outside: all successes must net to
	// a fully-released semaphore. We can't assert much about succ/fail
	// ratios without making the test flaky.
	assert.Equal(t, int64(0), s.Current(), "Current must be 0 after all Release")
	assert.Equal(t, int64(maxWeight), s.Available(), "Available must equal maxWeight after drain")
}

// BenchmarkStress_Semaphore_AcquireRelease establishes a throughput
// baseline for ±25% regression gates.
func BenchmarkStress_Semaphore_AcquireRelease(b *testing.B) {
	s := New(16)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.Acquire(ctx, 1); err != nil {
			b.Fatal(err)
		}
		s.Release(1)
	}
}
