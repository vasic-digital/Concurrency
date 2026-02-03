package queue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityQueue_New(t *testing.T) {
	pq := New[string](0)
	require.NotNil(t, pq)
	assert.True(t, pq.IsEmpty())
	assert.Equal(t, 0, pq.Len())
}

func TestPriorityQueue_PushPop_SingleItem(t *testing.T) {
	pq := New[string](0)
	pq.Push("hello", Normal)

	val, ok := pq.Pop()
	require.True(t, ok)
	assert.Equal(t, "hello", val)
	assert.True(t, pq.IsEmpty())
}

func TestPriorityQueue_Pop_Empty(t *testing.T) {
	pq := New[int](0)
	val, ok := pq.Pop()
	assert.False(t, ok)
	assert.Equal(t, 0, val)
}

func TestPriorityQueue_Peek_Empty(t *testing.T) {
	pq := New[int](0)
	val, ok := pq.Peek()
	assert.False(t, ok)
	assert.Equal(t, 0, val)
}

func TestPriorityQueue_PriorityOrder(t *testing.T) {
	tests := []struct {
		name     string
		items    []struct {
			value    string
			priority Priority
		}
		expectedOrder []string
	}{
		{
			name: "critical before low",
			items: []struct {
				value    string
				priority Priority
			}{
				{"low-item", Low},
				{"critical-item", Critical},
			},
			expectedOrder: []string{"critical-item", "low-item"},
		},
		{
			name: "all priorities",
			items: []struct {
				value    string
				priority Priority
			}{
				{"low", Low},
				{"high", High},
				{"normal", Normal},
				{"critical", Critical},
			},
			expectedOrder: []string{
				"critical", "high", "normal", "low",
			},
		},
		{
			name: "same priority preserves FIFO",
			items: []struct {
				value    string
				priority Priority
			}{
				{"first", Normal},
				{"second", Normal},
				{"third", Normal},
			},
			expectedOrder: []string{"first", "second", "third"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pq := New[string](0)
			for _, item := range tt.items {
				pq.Push(item.value, item.priority)
			}

			for _, expected := range tt.expectedOrder {
				val, ok := pq.Pop()
				require.True(t, ok)
				assert.Equal(t, expected, val)
			}
			assert.True(t, pq.IsEmpty())
		})
	}
}

func TestPriorityQueue_Peek(t *testing.T) {
	pq := New[string](0)
	pq.Push("a", Low)
	pq.Push("b", High)

	val, ok := pq.Peek()
	require.True(t, ok)
	assert.Equal(t, "b", val)
	// Peek should not remove
	assert.Equal(t, 2, pq.Len())
}

func TestPriorityQueue_Len(t *testing.T) {
	pq := New[int](10)
	assert.Equal(t, 0, pq.Len())

	pq.Push(1, Normal)
	pq.Push(2, Normal)
	pq.Push(3, Normal)
	assert.Equal(t, 3, pq.Len())

	pq.Pop()
	assert.Equal(t, 2, pq.Len())
}

func TestPriorityQueue_ConcurrentAccess(t *testing.T) {
	pq := New[int](0)
	var wg sync.WaitGroup
	n := 100

	// Concurrent pushes
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			pq.Push(v, Priority(v%4))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, n, pq.Len())

	// Concurrent pops
	popped := make([]int, 0, n)
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, ok := pq.Pop()
			if ok {
				mu.Lock()
				popped = append(popped, val)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, n, len(popped))
	assert.True(t, pq.IsEmpty())
}

func TestPriority_String(t *testing.T) {
	tests := []struct {
		priority Priority
		expected string
	}{
		{Low, "low"},
		{Normal, "normal"},
		{High, "high"},
		{Critical, "critical"},
		{Priority(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.priority.String())
		})
	}
}

func TestPriorityQueue_GenericTypes(t *testing.T) {
	// Test with struct type
	type Job struct {
		Name string
		ID   int
	}

	pq := New[Job](0)
	pq.Push(Job{Name: "build", ID: 1}, High)
	pq.Push(Job{Name: "test", ID: 2}, Low)

	val, ok := pq.Pop()
	require.True(t, ok)
	assert.Equal(t, "build", val.Name)
	assert.Equal(t, 1, val.ID)
}

func TestPriorityQueue_New_NegativeCapacity(t *testing.T) {
	// Test the path where initialCap < 0 (line 93-94)
	pq := New[string](-10) // Negative capacity should be treated as 0
	require.NotNil(t, pq)
	assert.True(t, pq.IsEmpty())

	// Queue should still work normally
	pq.Push("test", Normal)
	val, ok := pq.Pop()
	require.True(t, ok)
	assert.Equal(t, "test", val)
}

func TestPriorityQueue_New_VariousCapacities(t *testing.T) {
	tests := []struct {
		name        string
		initialCap  int
		expectedCap int // 0 means we just verify it works
	}{
		{
			name:       "negative capacity",
			initialCap: -100,
		},
		{
			name:       "zero capacity",
			initialCap: 0,
		},
		{
			name:       "positive capacity",
			initialCap: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pq := New[int](tt.initialCap)
			require.NotNil(t, pq)

			// Verify queue functions correctly regardless of initial capacity
			for i := 0; i < 10; i++ {
				pq.Push(i, Priority(i%4))
			}

			assert.Equal(t, 10, pq.Len())

			// Pop all items
			for i := 0; i < 10; i++ {
				_, ok := pq.Pop()
				require.True(t, ok)
			}

			assert.True(t, pq.IsEmpty())
		})
	}
}
