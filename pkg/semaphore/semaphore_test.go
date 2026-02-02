package semaphore

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemaphore_New(t *testing.T) {
	tests := []struct {
		name      string
		maxWeight int64
		expected  int64
	}{
		{"positive weight", 10, 10},
		{"zero defaults to 1", 0, 1},
		{"negative defaults to 1", -5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.maxWeight)
			require.NotNil(t, s)
			assert.Equal(t, tt.expected, s.Available())
		})
	}
}

func TestSemaphore_Acquire_Basic(t *testing.T) {
	s := New(10)
	err := s.Acquire(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), s.Current())
	assert.Equal(t, int64(5), s.Available())
}

func TestSemaphore_Acquire_ZeroWeight(t *testing.T) {
	s := New(10)
	err := s.Acquire(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), s.Current())
}

func TestSemaphore_Acquire_ExceedsMax(t *testing.T) {
	s := New(5)
	err := s.Acquire(context.Background(), 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max weight")
}

func TestSemaphore_Acquire_Blocks(t *testing.T) {
	s := New(1)
	err := s.Acquire(context.Background(), 1)
	require.NoError(t, err)

	// This should block
	ctx, cancel := context.WithTimeout(
		context.Background(), 50*time.Millisecond,
	)
	defer cancel()

	err = s.Acquire(ctx, 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestSemaphore_Acquire_UnblocksOnRelease(t *testing.T) {
	s := New(1)
	_ = s.Acquire(context.Background(), 1)

	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(
			context.Background(), 2*time.Second,
		)
		defer cancel()
		_ = s.Acquire(ctx, 1)
		close(done)
	}()

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)
	s.Release(1)

	select {
	case <-done:
		// success
	case <-time.After(time.Second):
		t.Fatal("Acquire did not unblock after Release")
	}
}

func TestSemaphore_TryAcquire_Success(t *testing.T) {
	s := New(5)
	assert.True(t, s.TryAcquire(3))
	assert.Equal(t, int64(3), s.Current())
}

func TestSemaphore_TryAcquire_Failure(t *testing.T) {
	s := New(5)
	_ = s.Acquire(context.Background(), 4)
	assert.False(t, s.TryAcquire(3))
}

func TestSemaphore_TryAcquire_ZeroWeight(t *testing.T) {
	s := New(5)
	assert.True(t, s.TryAcquire(0))
}

func TestSemaphore_TryAcquire_ExceedsMax(t *testing.T) {
	s := New(5)
	assert.False(t, s.TryAcquire(10))
}

func TestSemaphore_Release(t *testing.T) {
	s := New(10)
	_ = s.Acquire(context.Background(), 7)
	s.Release(3)
	assert.Equal(t, int64(4), s.Current())
	assert.Equal(t, int64(6), s.Available())
}

func TestSemaphore_Release_ZeroWeight(t *testing.T) {
	s := New(10)
	_ = s.Acquire(context.Background(), 5)
	s.Release(0)
	assert.Equal(t, int64(5), s.Current())
}

func TestSemaphore_Release_OverRelease(t *testing.T) {
	s := New(10)
	_ = s.Acquire(context.Background(), 3)
	s.Release(10) // release more than acquired
	assert.Equal(t, int64(0), s.Current())
}

func TestSemaphore_ConcurrentAccess(t *testing.T) {
	s := New(10)
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(
				context.Background(), 5*time.Second,
			)
			defer cancel()

			err := s.Acquire(ctx, 1)
			if err == nil {
				// Hold briefly
				time.Sleep(time.Millisecond)
				s.Release(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(0), s.Current())
}

func TestSemaphore_WeightedConcurrent(t *testing.T) {
	s := New(5)
	var wg sync.WaitGroup

	// 5 goroutines each needing weight 1
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Acquire(context.Background(), 1)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
			s.Release(1)
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(0), s.Current())
}
