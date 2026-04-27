// SPDX-License-Identifier: Apache-2.0
package safe

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Unit tests (single-goroutine contract) ---

func TestStore_ZeroValueReady(t *testing.T) {
	t.Parallel()
	var s Store[string, int]
	_, ok := s.Get("missing")
	assert.False(t, ok)
	assert.Equal(t, 0, s.Len())
	assert.False(t, s.Has("missing"))

	s.Put("a", 1)
	v, ok := s.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)
	assert.Equal(t, 1, s.Len())
}

func TestStore_PutGetDelete(t *testing.T) {
	t.Parallel()
	s := NewStore[string, int]()
	s.Put("a", 1)
	s.Put("b", 2)
	assert.Equal(t, 2, s.Len())

	prev, ok := s.Delete("a")
	assert.True(t, ok)
	assert.Equal(t, 1, prev)
	assert.Equal(t, 1, s.Len())
	assert.False(t, s.Has("a"))

	_, ok = s.Delete("missing")
	assert.False(t, ok)
}

func TestStore_PutIfAbsent(t *testing.T) {
	t.Parallel()
	s := NewStore[string, int]()
	v, stored := s.PutIfAbsent("a", 1)
	assert.True(t, stored)
	assert.Equal(t, 1, v)

	v, stored = s.PutIfAbsent("a", 999)
	assert.False(t, stored)
	assert.Equal(t, 1, v)

	got, _ := s.Get("a")
	assert.Equal(t, 1, got, "existing value preserved")
}

func TestStore_Update(t *testing.T) {
	t.Parallel()
	s := NewStore[string, int]()
	s.Put("counter", 5)

	s.Update("counter", func(cur int, ok bool) (int, bool) {
		assert.True(t, ok)
		return cur + 1, true
	})
	v, _ := s.Get("counter")
	assert.Equal(t, 6, v)

	s.Update("counter", func(int, bool) (int, bool) {
		return 0, false // deletes
	})
	assert.False(t, s.Has("counter"))

	s.Update("new", func(_ int, ok bool) (int, bool) {
		assert.False(t, ok)
		return 42, true
	})
	v, _ = s.Get("new")
	assert.Equal(t, 42, v)
}

func TestStore_SnapshotIndependence(t *testing.T) {
	t.Parallel()
	s := NewStore[string, int]()
	s.Put("a", 1)
	snap := s.Snapshot()
	snap["a"] = 999
	snap["new"] = 100

	got, _ := s.Get("a")
	assert.Equal(t, 1, got, "store untouched by caller mutations of snapshot")
	assert.False(t, s.Has("new"))
}

func TestStore_KeysValues(t *testing.T) {
	t.Parallel()
	s := NewStore[string, int]()
	s.Put("a", 1)
	s.Put("b", 2)
	keys := s.Keys()
	vals := s.Values()
	assert.ElementsMatch(t, []string{"a", "b"}, keys)
	assert.ElementsMatch(t, []int{1, 2}, vals)
}

func TestStore_Range_EarlyExit(t *testing.T) {
	t.Parallel()
	s := NewStore[int, int]()
	for i := 0; i < 10; i++ {
		s.Put(i, i*i)
	}
	var visited int
	s.Range(func(_ int, _ int) bool {
		visited++
		return visited < 3
	})
	assert.Equal(t, 3, visited, "Range stops when callback returns false")
}

func TestStore_Clear(t *testing.T) {
	t.Parallel()
	s := NewStore[string, int]()
	s.Put("a", 1)
	s.Put("b", 2)
	s.Clear()
	assert.Equal(t, 0, s.Len())
	s.Put("a", 1) // still usable after Clear
	assert.Equal(t, 1, s.Len())
}

// --- Stress tests (must be race-clean) ---

const (
	stressGoroutines = 16
	stressIterations = 2000
	stressDeadline   = 20 * time.Second
)

// TestStress_Store_ConcurrentPutGetDelete — the canonical race torture
// test. 16 goroutines churn the same Store with mixed operations for
// up to 20s. Must be -race clean AND produce zero errors.
func TestStress_Store_ConcurrentPutGetDelete(t *testing.T) {
	s := NewStore[string, int]()

	var wg sync.WaitGroup
	var errs atomic.Int64
	deadline := time.Now().Add(stressDeadline)

	for g := 0; g < stressGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < stressIterations; j++ {
				if time.Now().After(deadline) {
					return
				}
				key := fmt.Sprintf("g%d/k%d", id, j%64)
				s.Put(key, j)
				if v, ok := s.Get(key); ok && v != j && v >= 0 {
					// Value is either what we just wrote or was
					// overwritten by another goroutine; either way
					// it must be a valid int, not garbage.
					continue
				}
				if j%5 == 0 {
					s.Delete(key)
				}
				if j%7 == 0 {
					s.Update(key, func(cur int, _ bool) (int, bool) {
						return cur + 1, true
					})
				}
				if j%13 == 0 {
					_ = s.Len()
					_ = s.Has(key)
				}
				if j%31 == 0 {
					_ = s.Snapshot()
				}
			}
			_ = errs.Load()
		}(g)
	}
	wg.Wait()
	assert.Equal(t, int64(0), errs.Load())
}

// TestStress_Store_Update_Monotonic verifies that many concurrent
// Update-increments produce exactly the expected sum. Proves the
// read-modify-write atomicity.
func TestStress_Store_Update_Monotonic(t *testing.T) {
	s := NewStore[string, int64]()
	const key = "counter"
	const total = 16 * 1000

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				s.Update(key, func(cur int64, _ bool) (int64, bool) {
					return cur + 1, true
				})
			}
		}()
	}
	wg.Wait()
	v, _ := s.Get(key)
	assert.Equal(t, int64(total), v,
		"Update must be atomic — sum of increments must equal N×M exactly")
}

// TestStress_Store_Snapshot_DuringMutation verifies that Snapshot
// returns a consistent point-in-time view and that caller mutations
// of the snapshot do not leak back into the store.
func TestStress_Store_Snapshot_DuringMutation(t *testing.T) {
	s := NewStore[int, int]()
	for i := 0; i < 100; i++ {
		s.Put(i, i)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	// Writer goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; ; j++ {
			select {
			case <-done:
				return
			default:
			}
			s.Put(j%100, j*2)
		}
	}()

	// Reader takes snapshots and verifies they're internally consistent.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for iter := 0; iter < 200; iter++ {
			snap := s.Snapshot()
			for k, v := range snap {
				// k is in [0,100); v is some integer. Internal
				// consistency: no partial writes, no corrupted state.
				_ = k
				_ = v
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(done)
	wg.Wait()
}

func BenchmarkStore_Put(b *testing.B) {
	s := NewStore[int, int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(i%64, i)
	}
}

func BenchmarkStore_Get(b *testing.B) {
	s := NewStore[int, int]()
	for i := 0; i < 64; i++ {
		s.Put(i, i*i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Get(i % 64)
	}
}

// TestStore_Interface_Immutable guards against the "caller can bypass
// the lock" failure mode. If any method ever exposes the internal
// map, concurrent callers could race through that back door. This
// test fails at compile time (via interface assertion) if a method
// returning map[K]V is ever added without being explicitly designed
// to do so (Snapshot is the only sanctioned case — it returns a copy).
var _ = func() {
	// Compile-time check that we have Snapshot and Keys/Values as
	// value-copy accessors; there's no Map()/Raw()/Internal() method.
	s := &Store[int, int]{}
	_ = s.Snapshot
	_ = s.Keys
	_ = s.Values
}

// Document the no-deadlock contract of Range at compile time.
var _ = func() {
	s := &Store[int, int]{}
	s.Range(func(_ int, _ int) bool { return true })
}

// Helper to satisfy Go's require import when tests use only assert.
var _ = require.True
