// SPDX-License-Identifier: Apache-2.0
package safe

import "sync"

// Slice is a generic concurrent-safe slice.
//
// All operations are serialised through an internal sync.RWMutex.
// The underlying slice is never exposed — mutations go through
// Append / Delete / UpdateAt; reads go through Snapshot / Find /
// Range. This makes "iterate while another goroutine appends" a
// structurally impossible error.
type Slice[T any] struct {
	mu sync.RWMutex
	s  []T
}

// NewSlice constructs a Slice, optionally pre-populated.
func NewSlice[T any](init ...T) *Slice[T] {
	cp := make([]T, len(init))
	copy(cp, init)
	return &Slice[T]{s: cp}
}

// Append adds a value to the end of the slice.
func (s *Slice[T]) Append(v T) {
	s.mu.Lock()
	s.s = append(s.s, v)
	s.mu.Unlock()
}

// AppendAll adds multiple values.
func (s *Slice[T]) AppendAll(vs ...T) {
	s.mu.Lock()
	s.s = append(s.s, vs...)
	s.mu.Unlock()
}

// Len returns the number of elements.
func (s *Slice[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.s)
}

// At returns the element at index i and whether it exists.
func (s *Slice[T]) At(i int) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i < 0 || i >= len(s.s) {
		var zero T
		return zero, false
	}
	return s.s[i], true
}

// Snapshot returns a copy safe to iterate without the lock.
func (s *Slice[T]) Snapshot() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]T, len(s.s))
	copy(out, s.s)
	return out
}

// Find returns the first element matching pred, or zero-value + false.
// The predicate runs under the read lock and MUST NOT call back into
// the slice.
func (s *Slice[T]) Find(pred func(T) bool) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.s {
		if pred(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// FindIndex returns the index of the first element matching pred,
// or -1 if none match.
func (s *Slice[T]) FindIndex(pred func(T) bool) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i, v := range s.s {
		if pred(v) {
			return i
		}
	}
	return -1
}

// UpdateAt atomically finds the first element matching pred, applies
// fn to it, and writes the result back. The match-modify-replace
// sequence is under a single write lock, so "find then update" race
// conditions are impossible. Returns true if an update occurred.
func (s *Slice[T]) UpdateAt(pred func(T) bool, fn func(T) T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.s {
		if pred(v) {
			s.s[i] = fn(v)
			return true
		}
	}
	return false
}

// Delete removes the first element matching pred. Returns the
// removed element (if any) and whether a removal occurred.
func (s *Slice[T]) Delete(pred func(T) bool) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.s {
		if pred(v) {
			s.s = append(s.s[:i], s.s[i+1:]...)
			return v, true
		}
	}
	var zero T
	return zero, false
}

// Range visits every element under the read lock. Same caveats as
// Store.Range apply — callback must not block or call back into the
// slice.
func (s *Slice[T]) Range(fn func(int, T) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i, v := range s.s {
		if !fn(i, v) {
			return
		}
	}
}

// Clear empties the slice.
func (s *Slice[T]) Clear() {
	s.mu.Lock()
	s.s = nil
	s.mu.Unlock()
}

// Replace atomically replaces the entire backing slice. The caller
// gives up ownership of `next` — do not mutate after passing in.
func (s *Slice[T]) Replace(next []T) {
	cp := make([]T, len(next))
	copy(cp, next)
	s.mu.Lock()
	s.s = cp
	s.mu.Unlock()
}
