package semaphore

import (
	"context"
	"fmt"
	"sync"
)

// Semaphore implements a weighted semaphore for controlling
// concurrent access to a shared resource. It is safe for
// concurrent use.
type Semaphore struct {
	maxWeight int64
	current   int64
	mu        sync.Mutex
	waiters   []waiter
}

type waiter struct {
	weight int64
	ready  chan struct{}
}

// New creates a new Semaphore with the given maximum weight.
func New(maxWeight int64) *Semaphore {
	if maxWeight <= 0 {
		maxWeight = 1
	}
	return &Semaphore{
		maxWeight: maxWeight,
	}
}

// Acquire blocks until weight units are available or the context
// is cancelled. Returns an error if the context is cancelled or
// the requested weight exceeds maxWeight.
func (s *Semaphore) Acquire(ctx context.Context, weight int64) error {
	if weight <= 0 {
		return nil
	}
	if weight > s.maxWeight {
		return fmt.Errorf(
			"weight %d exceeds max weight %d",
			weight, s.maxWeight,
		)
	}

	s.mu.Lock()
	if s.current+weight <= s.maxWeight && len(s.waiters) == 0 {
		s.current += weight
		s.mu.Unlock()
		return nil
	}

	// Must wait
	w := waiter{
		weight: weight,
		ready:  make(chan struct{}),
	}
	s.waiters = append(s.waiters, w)
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		// Remove ourselves from the waiter list
		s.mu.Lock()
		for i, ww := range s.waiters {
			if ww.ready == w.ready {
				s.waiters = append(
					s.waiters[:i], s.waiters[i+1:]...,
				)
				break
			}
		}
		s.mu.Unlock()
		return ctx.Err()
	case <-w.ready:
		return nil
	}
}

// TryAcquire attempts to acquire weight units without blocking.
// Returns true if successful, false otherwise.
func (s *Semaphore) TryAcquire(weight int64) bool {
	if weight <= 0 {
		return true
	}
	if weight > s.maxWeight {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current+weight <= s.maxWeight && len(s.waiters) == 0 {
		s.current += weight
		return true
	}
	return false
}

// Release returns weight units to the semaphore, potentially
// unblocking waiting goroutines.
func (s *Semaphore) Release(weight int64) {
	if weight <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.current -= weight
	if s.current < 0 {
		s.current = 0
	}

	// Wake up waiting goroutines that can now proceed
	s.notifyWaiters()
}

// Current returns the currently acquired weight.
func (s *Semaphore) Current() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// Available returns the available weight.
func (s *Semaphore) Available() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxWeight - s.current
}

// notifyWaiters wakes waiters that can now proceed.
// Must be called with the mutex held.
func (s *Semaphore) notifyWaiters() {
	i := 0
	for i < len(s.waiters) {
		w := s.waiters[i]
		if s.current+w.weight <= s.maxWeight {
			s.current += w.weight
			close(w.ready)
			s.waiters = append(
				s.waiters[:i], s.waiters[i+1:]...,
			)
		} else {
			i++
		}
	}
}
