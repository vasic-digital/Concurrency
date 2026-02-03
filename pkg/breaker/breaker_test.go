package breaker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_New_Default(t *testing.T) {
	cb := New(nil)
	require.NotNil(t, cb)
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_New_Custom(t *testing.T) {
	cb := New(&Config{
		MaxFailures:      3,
		Timeout:          10 * time.Second,
		HalfOpenRequests: 2,
	})
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := New(&Config{MaxFailures: 3, Timeout: time.Second})

	err := cb.Execute(func() error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, Closed, cb.State())
	assert.Equal(t, 0, cb.Failures())
}

func TestCircuitBreaker_Execute_FailuresBelowThreshold(t *testing.T) {
	cb := New(&Config{
		MaxFailures: 3,
		Timeout:     time.Second,
	})

	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return fmt.Errorf("error %d", i)
		})
	}
	assert.Equal(t, Closed, cb.State())
	assert.Equal(t, 2, cb.Failures())
}

func TestCircuitBreaker_Execute_OpensAfterMaxFailures(t *testing.T) {
	cb := New(&Config{
		MaxFailures: 3,
		Timeout:     time.Second,
	})

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error {
			return fmt.Errorf("error")
		})
	}
	assert.Equal(t, Open, cb.State())
}

func TestCircuitBreaker_Execute_RejectsWhenOpen(t *testing.T) {
	cb := New(&Config{
		MaxFailures: 1,
		Timeout:     10 * time.Second,
	})

	// Trip the breaker
	_ = cb.Execute(func() error {
		return fmt.Errorf("error")
	})
	assert.Equal(t, Open, cb.State())

	// Should reject without calling fn
	called := false
	err := cb.Execute(func() error {
		called = true
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
	assert.False(t, called)
}

func TestCircuitBreaker_Execute_TransitionsToHalfOpen(t *testing.T) {
	cb := New(&Config{
		MaxFailures:      1,
		Timeout:          50 * time.Millisecond,
		HalfOpenRequests: 1,
	})

	// Trip the breaker
	_ = cb.Execute(func() error {
		return fmt.Errorf("error")
	})
	assert.Equal(t, Open, cb.State())

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, HalfOpen, cb.State())
}

func TestCircuitBreaker_Execute_HalfOpenSuccess(t *testing.T) {
	cb := New(&Config{
		MaxFailures:      1,
		Timeout:          50 * time.Millisecond,
		HalfOpenRequests: 1,
	})

	// Trip the breaker
	_ = cb.Execute(func() error {
		return fmt.Errorf("error")
	})

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Successful probe should close the circuit
	err := cb.Execute(func() error { return nil })
	require.NoError(t, err)
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_Execute_HalfOpenFailure(t *testing.T) {
	cb := New(&Config{
		MaxFailures:      1,
		Timeout:          50 * time.Millisecond,
		HalfOpenRequests: 1,
	})

	// Trip the breaker
	_ = cb.Execute(func() error {
		return fmt.Errorf("error")
	})

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Failed probe should re-open the circuit
	_ = cb.Execute(func() error {
		return fmt.Errorf("still failing")
	})
	assert.Equal(t, Open, cb.State())
}

func TestCircuitBreaker_Execute_SuccessResetsFailures(t *testing.T) {
	cb := New(&Config{
		MaxFailures: 3,
		Timeout:     time.Second,
	})

	// Two failures
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return fmt.Errorf("error")
		})
	}
	assert.Equal(t, 2, cb.Failures())

	// Success resets
	_ = cb.Execute(func() error { return nil })
	assert.Equal(t, 0, cb.Failures())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := New(&Config{
		MaxFailures: 1,
		Timeout:     10 * time.Second,
	})

	_ = cb.Execute(func() error {
		return fmt.Errorf("error")
	})
	assert.Equal(t, Open, cb.State())

	cb.Reset()
	assert.Equal(t, Closed, cb.State())
	assert.Equal(t, 0, cb.Failures())
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := New(&Config{
		MaxFailures:      100,
		Timeout:          time.Second,
		HalfOpenRequests: 1,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				_ = cb.Execute(func() error { return nil })
			} else {
				_ = cb.Execute(func() error {
					return fmt.Errorf("err")
				})
			}
		}(i)
	}
	wg.Wait()

	// Should not have panicked
	_ = cb.State()
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{Closed, "closed"},
		{Open, "open"},
		{HalfOpen, "half-open"},
		{State(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestCircuitBreaker_HalfOpen_MultipleProbes(t *testing.T) {
	cb := New(&Config{
		MaxFailures:      1,
		Timeout:          50 * time.Millisecond,
		HalfOpenRequests: 3,
	})

	// Trip
	_ = cb.Execute(func() error {
		return fmt.Errorf("error")
	})

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, HalfOpen, cb.State())

	// Need 3 successes to close
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error { return nil })
		require.NoError(t, err)
	}
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_HalfOpen_RejectsExcessRequests(t *testing.T) {
	// Test the path where halfOpenAllowed <= 0 in half-open state
	cb := New(&Config{
		MaxFailures:      1,
		Timeout:          50 * time.Millisecond,
		HalfOpenRequests: 1, // Only allow 1 probe request
	})

	// Trip the breaker
	_ = cb.Execute(func() error {
		return fmt.Errorf("error")
	})
	assert.Equal(t, Open, cb.State())

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, HalfOpen, cb.State())

	// First request should be allowed (uses up the halfOpenAllowed)
	var firstCallMade bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := cb.Execute(func() error {
			firstCallMade = true
			time.Sleep(50 * time.Millisecond) // Hold the lock
			return nil
		})
		require.NoError(t, err)
	}()

	// Give the first goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Second request should be rejected because halfOpenAllowed is 0
	err := cb.Execute(func() error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")

	wg.Wait()
	assert.True(t, firstCallMade)
}

func TestCircuitBreaker_New_ZeroHalfOpenRequests(t *testing.T) {
	// Test that HalfOpenRequests <= 0 defaults to 1
	cb := New(&Config{
		MaxFailures:      1,
		Timeout:          50 * time.Millisecond,
		HalfOpenRequests: 0, // Should default to 1
	})

	// Verify the config was adjusted
	assert.Equal(t, 1, cb.config.HalfOpenRequests)
}
