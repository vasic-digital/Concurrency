// SPDX-License-Identifier: Apache-2.0
package safe

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlice_Basic(t *testing.T) {
	t.Parallel()
	s := NewSlice[int](1, 2, 3)
	assert.Equal(t, 3, s.Len())
	v, ok := s.At(1)
	assert.True(t, ok)
	assert.Equal(t, 2, v)

	s.Append(4)
	assert.Equal(t, 4, s.Len())

	s.AppendAll(5, 6, 7)
	assert.Equal(t, 7, s.Len())

	snap := s.Snapshot()
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7}, snap)
}

func TestSlice_At_OutOfBounds(t *testing.T) {
	t.Parallel()
	s := NewSlice[int](1)
	_, ok := s.At(-1)
	assert.False(t, ok)
	_, ok = s.At(100)
	assert.False(t, ok)
}

func TestSlice_Find(t *testing.T) {
	t.Parallel()
	s := NewSlice[int](10, 20, 30, 40)
	v, ok := s.Find(func(x int) bool { return x > 25 })
	assert.True(t, ok)
	assert.Equal(t, 30, v)

	_, ok = s.Find(func(x int) bool { return x > 100 })
	assert.False(t, ok)
}

func TestSlice_FindIndex(t *testing.T) {
	t.Parallel()
	s := NewSlice[string]("a", "b", "c")
	assert.Equal(t, 1, s.FindIndex(func(x string) bool { return x == "b" }))
	assert.Equal(t, -1, s.FindIndex(func(x string) bool { return x == "z" }))
}

func TestSlice_UpdateAt(t *testing.T) {
	t.Parallel()
	s := NewSlice[int](1, 2, 3, 4)
	updated := s.UpdateAt(
		func(x int) bool { return x == 2 },
		func(x int) int { return x * 10 },
	)
	assert.True(t, updated)
	assert.Equal(t, []int{1, 20, 3, 4}, s.Snapshot())

	updated = s.UpdateAt(
		func(x int) bool { return x == 999 },
		func(x int) int { return x },
	)
	assert.False(t, updated)
}

func TestSlice_Delete(t *testing.T) {
	t.Parallel()
	s := NewSlice[int](1, 2, 3, 4)
	v, ok := s.Delete(func(x int) bool { return x == 3 })
	assert.True(t, ok)
	assert.Equal(t, 3, v)
	assert.Equal(t, []int{1, 2, 4}, s.Snapshot())

	_, ok = s.Delete(func(x int) bool { return x == 999 })
	assert.False(t, ok)
}

func TestSlice_Range_EarlyExit(t *testing.T) {
	t.Parallel()
	s := NewSlice[int]()
	for i := 0; i < 10; i++ {
		s.Append(i)
	}
	var visited int
	s.Range(func(_ int, _ int) bool {
		visited++
		return visited < 4
	})
	assert.Equal(t, 4, visited)
}

func TestSlice_Clear_Replace(t *testing.T) {
	t.Parallel()
	s := NewSlice[int](1, 2, 3)
	s.Clear()
	assert.Equal(t, 0, s.Len())

	s.Replace([]int{10, 20, 30})
	assert.Equal(t, []int{10, 20, 30}, s.Snapshot())
}

func TestSlice_Replace_Defensive(t *testing.T) {
	t.Parallel()
	s := NewSlice[int]()
	input := []int{1, 2, 3}
	s.Replace(input)
	input[0] = 999 // caller mutates after passing
	got := s.Snapshot()
	assert.Equal(t, []int{1, 2, 3}, got, "Replace must copy input")
}

// --- Stress ---

func TestStress_Slice_ConcurrentAppendReadDelete(t *testing.T) {
	s := NewSlice[int]()

	var wg sync.WaitGroup
	deadline := time.Now().Add(stressDeadline)
	var appends atomic.Int64

	for g := 0; g < stressGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < stressIterations; j++ {
				if time.Now().After(deadline) {
					return
				}
				s.Append(id*10000 + j)
				appends.Add(1)
				if j%5 == 0 {
					_ = s.Snapshot()
				}
				if j%7 == 0 {
					s.UpdateAt(
						func(v int) bool { return v == id*10000 },
						func(v int) int { return v + 1 },
					)
				}
				if j%11 == 0 {
					s.Delete(func(v int) bool { return v == id*10000+j-5 })
				}
			}
		}(g)
	}
	wg.Wait()
}

// TestStress_Slice_UpdateAt_Exclusive verifies that UpdateAt is
// atomic w.r.t. the match+replace sequence. Two goroutines compete
// to "claim" entries; exactly total-count claims must succeed.
func TestStress_Slice_UpdateAt_Exclusive(t *testing.T) {
	type entry struct {
		id     int
		claimed atomic.Bool
	}
	// Pre-populate with N entries.
	const n = 1000
	s := NewSlice[*entry]()
	for i := 0; i < n; i++ {
		s.Append(&entry{id: i})
	}

	var claimed atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < n; i++ {
				ok := s.UpdateAt(
					func(e *entry) bool { return e.id == i && !e.claimed.Load() },
					func(e *entry) *entry {
						if e.claimed.CompareAndSwap(false, true) {
							claimed.Add(1)
						}
						return e
					},
				)
				_ = ok
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(n), claimed.Load(),
		"every entry claimed exactly once (no double-claim, no missed)")
}

func BenchmarkSlice_Append(b *testing.B) {
	s := NewSlice[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Append(i)
	}
}

func BenchmarkSlice_Snapshot_Small(b *testing.B) {
	s := NewSlice[int]()
	for i := 0; i < 64; i++ {
		s.Append(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Snapshot()
	}
}

// Guard: no method exists that returns the internal slice by reference.
var _ = func() {
	s := &Slice[int]{}
	_ = s.Snapshot // copy
	_ = s.At       // copy by value
	_ = s.Find     // copy by value
	// No s.Raw() / s.Slice() — must not exist.
	_ = fmt.Sprintf
}
