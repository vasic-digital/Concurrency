// Package safe provides rock-solid generic concurrent containers.
//
// Every operation on Store or Slice is serialised through an internal
// sync.RWMutex. Callers cannot obtain an unlocked reference to the
// underlying collection — by construction. This makes the "forgot to
// take the lock" bug class structurally impossible rather than
// review-caught.
//
// Use this package instead of bare `map[K]V + sync.RWMutex` inside
// any struct whose value is shared across goroutines. See
// docs/development/concurrency-playbook.md in HelixAgent for the
// migration rules.
package safe

import "sync"

// Store is a generic concurrent-safe key-value container.
//
// Zero-value is ready for use (returns empty on reads, initialises
// the underlying map on first write). The embedded mutex is NOT
// exposed, so callers can never acquire or bypass it directly.
type Store[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewStore constructs an empty Store.
func NewStore[K comparable, V any]() *Store[K, V] {
	return &Store[K, V]{m: make(map[K]V)}
}

// NewStoreFromMap constructs a Store pre-populated with the given
// entries. The input map is copied; callers may mutate it afterwards
// without affecting the Store.
func NewStoreFromMap[K comparable, V any](init map[K]V) *Store[K, V] {
	s := &Store[K, V]{m: make(map[K]V, len(init))}
	for k, v := range init {
		s.m[k] = v
	}
	return s
}

// Get returns the value for key and whether it was present.
func (s *Store[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

// GetOrDefault returns the value for key, or def if absent.
func (s *Store[K, V]) GetOrDefault(key K, def V) V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.m[key]; ok {
		return v
	}
	return def
}

// Put stores value under key, replacing any existing value.
func (s *Store[K, V]) Put(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lazyInit()
	s.m[key] = value
}

// PutIfAbsent stores value under key only if the key is not already
// present. Returns the current value (new if stored, existing if not)
// and whether a store happened.
func (s *Store[K, V]) PutIfAbsent(key K, value V) (V, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lazyInit()
	if existing, ok := s.m[key]; ok {
		return existing, false
	}
	s.m[key] = value
	return value, true
}

// Delete removes key and returns the previous value (if any) and
// whether the key was present.
func (s *Store[K, V]) Delete(key K) (V, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	if ok {
		delete(s.m, key)
	}
	return v, ok
}

// Has reports whether key is present.
func (s *Store[K, V]) Has(key K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[key]
	return ok
}

// Len returns the number of entries.
func (s *Store[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

// Update atomically reads, modifies, and writes the value for key.
// The callback receives the current value (or the zero-V if absent)
// and the presence flag, and returns the new value. If returning
// keep == false the entry is deleted; otherwise it is stored.
//
// The entire read-modify-write is under one write lock, so
// "check-then-act" compound operations cannot race.
func (s *Store[K, V]) Update(key K, fn func(current V, present bool) (next V, keep bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lazyInit()
	cur, present := s.m[key]
	next, keep := fn(cur, present)
	if !keep {
		delete(s.m, key)
		return
	}
	s.m[key] = next
}

// Snapshot returns a point-in-time copy of the entries, safe to
// iterate without holding the store's lock.
func (s *Store[K, V]) Snapshot() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[K]V, len(s.m))
	for k, v := range s.m {
		out[k] = v
	}
	return out
}

// Range visits entries under the read lock. The callback MUST NOT
// block, call back into the store, or retain references beyond the
// callback return — doing so can deadlock or produce stale reads.
// For non-trivial per-entry work, use Snapshot and iterate the copy.
//
// Iteration stops early if the callback returns false.
func (s *Store[K, V]) Range(fn func(K, V) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.m {
		if !fn(k, v) {
			return
		}
	}
}

// Keys returns a point-in-time copy of the keys.
func (s *Store[K, V]) Keys() []K {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]K, 0, len(s.m))
	for k := range s.m {
		out = append(out, k)
	}
	return out
}

// Values returns a point-in-time copy of the values.
func (s *Store[K, V]) Values() []V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]V, 0, len(s.m))
	for _, v := range s.m {
		out = append(out, v)
	}
	return out
}

// Clear removes all entries.
func (s *Store[K, V]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = make(map[K]V)
}

// lazyInit ensures s.m is non-nil. Must be called with s.mu.Lock held.
func (s *Store[K, V]) lazyInit() {
	if s.m == nil {
		s.m = make(map[K]V)
	}
}
