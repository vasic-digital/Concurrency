package bulkhead

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Defaults(t *testing.T) {
	b := New(Config{})
	stats := b.GetStats()
	assert.Equal(t, 10, stats.MaxConcurrent)
	assert.Equal(t, 100, stats.QueueSize)
	assert.Equal(t, 10, stats.AvailablePermit)
}

func TestNew_CustomConfig(t *testing.T) {
	b := New(Config{MaxConcurrent: 5, QueueSize: 50, Timeout: 10 * time.Second})
	stats := b.GetStats()
	assert.Equal(t, 5, stats.MaxConcurrent)
	assert.Equal(t, 50, stats.QueueSize)
	assert.Equal(t, 5, stats.AvailablePermit)
}

func TestExecute_Success(t *testing.T) {
	b := New(Config{MaxConcurrent: 2, Timeout: time.Second})
	err := b.Execute(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExecute_FnError(t *testing.T) {
	b := New(Config{MaxConcurrent: 2, Timeout: time.Second})
	testErr := errors.New("test error")
	err := b.Execute(context.Background(), func() error {
		return testErr
	})
	assert.ErrorIs(t, err, testErr)
}

func TestExecute_ConcurrencyLimit(t *testing.T) {
	b := New(Config{MaxConcurrent: 3, Timeout: 5 * time.Second})

	var running int64
	var maxRunning int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Execute(context.Background(), func() error {
				cur := atomic.AddInt64(&running, 1)
				mu.Lock()
				if cur > maxRunning {
					maxRunning = cur
				}
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
				atomic.AddInt64(&running, -1)
				return nil
			})
		}()
	}

	wg.Wait()
	assert.LessOrEqual(t, maxRunning, int64(3))
}

func TestExecute_ContextCancelled(t *testing.T) {
	b := New(Config{MaxConcurrent: 1, Timeout: 5 * time.Second})

	// Exhaust the single permit
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = b.Execute(context.Background(), func() error {
			close(started)
			<-done
			return nil
		})
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := b.Execute(ctx, func() error {
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)

	close(done)
}

func TestExecute_Timeout(t *testing.T) {
	b := New(Config{MaxConcurrent: 1, Timeout: 50 * time.Millisecond})

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = b.Execute(context.Background(), func() error {
			close(started)
			<-done
			return nil
		})
	}()
	<-started

	err := b.Execute(context.Background(), func() error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")

	close(done)
}

func TestGetStats_PermitRelease(t *testing.T) {
	b := New(Config{MaxConcurrent: 3, Timeout: time.Second})

	assert.Equal(t, 3, b.GetStats().AvailablePermit)

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = b.Execute(context.Background(), func() error {
			close(started)
			<-done
			return nil
		})
	}()
	<-started

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 2, b.GetStats().AvailablePermit)

	close(done)
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 3, b.GetStats().AvailablePermit)
}
