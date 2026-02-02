package queue

import (
	"container/heap"
	"fmt"
	"sync"
	"sync/atomic"
)

// Priority represents the priority level of a queued item
type Priority int

const (
	Low      Priority = 0
	Normal   Priority = 1
	High     Priority = 2
	Critical Priority = 3
)

// String returns a string representation of the priority
func (p Priority) String() string {
	switch p {
	case Low:
		return "low"
	case Normal:
		return "normal"
	case High:
		return "high"
	case Critical:
		return "critical"
	default:
		return fmt.Sprintf("unknown(%d)", int(p))
	}
}

// item is an internal wrapper around a queued value
type item[T any] struct {
	value    T
	priority Priority
	seq      uint64 // insertion order for stable sorting
	index    int    // index in the heap
}

// innerHeap implements heap.Interface for item[T]
type innerHeap[T any] struct {
	items []*item[T]
}

func (h *innerHeap[T]) Len() int { return len(h.items) }

func (h *innerHeap[T]) Less(i, j int) bool {
	// Higher priority first; if equal, earlier insertion first
	if h.items[i].priority != h.items[j].priority {
		return h.items[i].priority > h.items[j].priority
	}
	return h.items[i].seq < h.items[j].seq
}

func (h *innerHeap[T]) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].index = i
	h.items[j].index = j
}

func (h *innerHeap[T]) Push(x interface{}) {
	it := x.(*item[T])
	it.index = len(h.items)
	h.items = append(h.items, it)
}

func (h *innerHeap[T]) Pop() interface{} {
	old := h.items
	n := len(old)
	it := old[n-1]
	old[n-1] = nil // avoid memory leak
	it.index = -1
	h.items = old[:n-1]
	return it
}

// PriorityQueue is a generic, thread-safe priority queue.
// Items with higher priority are dequeued first. Items with
// equal priority are dequeued in FIFO order.
type PriorityQueue[T any] struct {
	h   *innerHeap[T]
	mu  sync.Mutex
	seq uint64
}

// New creates a new PriorityQueue. If initialCap > 0, the
// underlying slice is pre-allocated with that capacity.
func New[T any](initialCap int) *PriorityQueue[T] {
	if initialCap < 0 {
		initialCap = 0
	}
	pq := &PriorityQueue[T]{
		h: &innerHeap[T]{
			items: make([]*item[T], 0, initialCap),
		},
	}
	heap.Init(pq.h)
	return pq
}

// Push adds a value with the given priority to the queue.
func (pq *PriorityQueue[T]) Push(value T, priority Priority) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	it := &item[T]{
		value:    value,
		priority: priority,
		seq:      atomic.AddUint64(&pq.seq, 1),
	}
	heap.Push(pq.h, it)
}

// Pop removes and returns the highest-priority item.
// Returns the zero value of T and false if the queue is empty.
func (pq *PriorityQueue[T]) Pop() (T, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.h.Len() == 0 {
		var zero T
		return zero, false
	}

	it := heap.Pop(pq.h).(*item[T])
	return it.value, true
}

// Peek returns the highest-priority item without removing it.
// Returns the zero value of T and false if the queue is empty.
func (pq *PriorityQueue[T]) Peek() (T, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.h.Len() == 0 {
		var zero T
		return zero, false
	}

	return pq.h.items[0].value, true
}

// Len returns the number of items in the queue.
func (pq *PriorityQueue[T]) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.h.Len()
}

// IsEmpty returns true if the queue has no items.
func (pq *PriorityQueue[T]) IsEmpty() bool {
	return pq.Len() == 0
}
