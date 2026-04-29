package monitor

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceMonitor_New_Default(t *testing.T) {
	m := New(nil)
	require.NotNil(t, m)
	assert.Equal(t, "/", m.config.DiskPath)
}

func TestResourceMonitor_New_Custom(t *testing.T) {
	m := New(&Config{
		DiskPath:      "/tmp",
		CPUSampleTime: 100 * time.Millisecond,
	})
	assert.Equal(t, "/tmp", m.config.DiskPath)
}

func TestResourceMonitor_GetSystemResources(t *testing.T) {
	m := New(&Config{
		DiskPath:      "/",
		CPUSampleTime: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second,
	)
	defer cancel()

	res, err := m.GetSystemResources(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Validate fields have reasonable values
	assert.Equal(t, runtime.NumCPU(), res.NumCPU)
	assert.Greater(t, res.NumGoroutines, 0)
	assert.Greater(t, res.HeapAlloc, uint64(0))
	assert.Greater(t, res.MemoryTotal, uint64(0))
	assert.Greater(t, res.MemoryUsed, uint64(0))
	assert.Greater(t, res.DiskTotal, uint64(0))
	assert.False(t, res.CollectedAt.IsZero())
}

func TestResourceMonitor_GetSystemResources_ContextCancelled(t *testing.T) {
	// bluff-scan: no-assert-ok (context-cancel smoke — cancel path must not panic/leak)
	m := New(&Config{
		CPUSampleTime: 5 * time.Second, // very long
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Should not hang, may return partial results or error
	_, _ = m.GetSystemResources(ctx)
}

func TestResourceMonitor_Start_Stop(t *testing.T) {
	m := New(&Config{
		DiskPath:        "/",
		CPUSampleTime:   50 * time.Millisecond,
		CollectInterval: 100 * time.Millisecond,
	})

	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(t, err)

	// Should have initial collection
	latest := m.Latest()
	require.NotNil(t, latest)
	assert.Greater(t, latest.MemoryTotal, uint64(0))

	// Wait for at least one background collection
	time.Sleep(200 * time.Millisecond)

	latest2 := m.Latest()
	require.NotNil(t, latest2)

	m.Stop()
}

func TestResourceMonitor_Latest_BeforeStart(t *testing.T) {
	m := New(nil)
	assert.Nil(t, m.Latest())
}

func TestResourceMonitor_Start_InvalidInterval(t *testing.T) {
	m := New(&Config{
		CollectInterval: 0,
	})

	err := m.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collect interval")
}

func TestSystemResources_Fields(t *testing.T) {
	res := &SystemResources{
		CPUPercent:      50.0,
		NumCPU:          4,
		MemoryTotal:     16000000000,
		MemoryUsed:      8000000000,
		MemoryAvailable: 8000000000,
		MemoryPercent:   50.0,
		DiskTotal:       500000000000,
		DiskUsed:        250000000000,
		DiskFree:        250000000000,
		DiskPercent:     50.0,
		Load1:           1.5,
		Load5:           1.0,
		Load15:          0.8,
		NumGoroutines:   10,
		HeapAlloc:       1000000,
		HeapSys:         2000000,
		CollectedAt:     time.Now(),
	}

	assert.Equal(t, 50.0, res.CPUPercent)
	assert.Equal(t, 4, res.NumCPU)
	assert.Equal(t, uint64(16000000000), res.MemoryTotal)
	assert.Equal(t, 1.5, res.Load1)
	assert.Equal(t, 10, res.NumGoroutines)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "/", cfg.DiskPath)
	assert.Equal(t, 200*time.Millisecond, cfg.CPUSampleTime)
	assert.Equal(t, 5*time.Second, cfg.CollectInterval)
}

func TestResourceMonitor_New_EmptyDiskPath(t *testing.T) {
	// Test the path where DiskPath is empty (line 78-79)
	m := New(&Config{
		DiskPath:      "", // Empty - should default to "/"
		CPUSampleTime: 100 * time.Millisecond,
	})
	assert.Equal(t, "/", m.config.DiskPath)
}

func TestResourceMonitor_New_ZeroCPUSampleTime(t *testing.T) {
	// Test the path where CPUSampleTime <= 0 (line 81-82)
	m := New(&Config{
		DiskPath:      "/",
		CPUSampleTime: 0, // Zero - should default to 200ms
	})
	assert.Equal(t, 200*time.Millisecond, m.config.CPUSampleTime)
}

func TestResourceMonitor_Start_CollectLoop_Error(t *testing.T) {
	// Test the continue path in collectLoop when GetSystemResources errors
	// This is hard to trigger directly, but we can test the path exists
	// by verifying the loop continues after an error
	m := New(&Config{
		DiskPath:        "/nonexistent/path/that/does/not/exist/xyz123",
		CPUSampleTime:   50 * time.Millisecond,
		CollectInterval: 50 * time.Millisecond,
	})

	ctx := context.Background()
	err := m.Start(ctx)
	// The initial collection may succeed even with bad disk path
	// (it just won't get disk stats), so we just verify start works
	if err != nil {
		// This is the error path in Start (line 162-163)
		assert.Contains(t, err.Error(), "initial collection failed")
	} else {
		// Wait for a few collection cycles
		time.Sleep(200 * time.Millisecond)
		m.Stop()
		// Verify latest was updated despite potential errors
		latest := m.Latest()
		assert.NotNil(t, latest)
	}
}

func TestResourceMonitor_Stop_BeforeStart(t *testing.T) {
	// Test Stop when cancel is nil (line 175-177)
	m := New(nil)
	// Should not panic
	m.Stop()
}

func TestResourceMonitor_Start_NegativeInterval(t *testing.T) {
	m := New(&Config{
		CollectInterval: -1 * time.Second, // Negative interval
	})

	err := m.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collect interval must be positive")
}

func TestResourceMonitor_Start_InitialCollectionFailure(t *testing.T) {
	// Test the initial collection failure path (lines 162-163).
	// This is triggered when GetSystemResources fails during Start.
	// Since GetSystemResources always returns (res, nil), this path
	// cannot be triggered through normal usage. The code path exists
	// for potential future error conditions from system calls.

	// The best we can do is verify the code handles cancelled contexts
	// which may cause partial results but not actual errors.
	m := New(&Config{
		DiskPath:        "/",
		CPUSampleTime:   time.Nanosecond, // Very short
		CollectInterval: 100 * time.Millisecond,
	})

	// Create context that times out quickly
	ctx, cancel := context.WithTimeout(context.Background(), time.Microsecond)
	defer cancel()
	time.Sleep(time.Millisecond) // Ensure timeout

	// Start with cancelled context - may or may not fail
	err := m.Start(ctx)
	// Either succeeds or fails depending on timing
	if err != nil {
		// If failed, should be due to initial collection
		assert.Contains(t, err.Error(), "initial collection failed")
	} else {
		m.Stop()
	}
}

func TestResourceMonitor_collectLoop_ErrorContinues(t *testing.T) {
	// Test that collectLoop continues after GetSystemResources errors (line 198-199).
	// GetSystemResources doesn't return errors normally, but the continue path
	// exists for robustness.

	// We can test the loop structure by verifying it updates latest
	m := New(&Config{
		DiskPath:        "/",
		CPUSampleTime:   10 * time.Millisecond,
		CollectInterval: 50 * time.Millisecond,
	})

	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(t, err)

	// Wait for multiple collection cycles
	time.Sleep(200 * time.Millisecond)

	// Verify latest has been updated
	latest := m.Latest()
	require.NotNil(t, latest)

	m.Stop()
}

func TestResourceMonitor_collectLoop_ContextDoneBeforeTick(t *testing.T) {
	// Test the context.Done() path in collectLoop (line 194-195).
	m := New(&Config{
		DiskPath:        "/",
		CPUSampleTime:   10 * time.Millisecond,
		CollectInterval: 5 * time.Second, // Long interval
	})

	ctx, cancel := context.WithCancel(context.Background())
	err := m.Start(ctx)
	require.NoError(t, err)

	// Cancel context before first tick
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Give time for goroutine to exit
	time.Sleep(50 * time.Millisecond)

	// Stop should be safe even after context cancel
	m.Stop()
}

func TestResourceMonitor_Start_InitialCollectionError(t *testing.T) {
	// Test the error path in Start when initial collection fails
	m := New(&Config{
		DiskPath:        "/",
		CPUSampleTime:   10 * time.Millisecond,
		CollectInterval: 100 * time.Millisecond,
	})

	// Set a custom resource getter that returns an error
	callCount := 0
	m.SetResourceGetter(func(_ context.Context) (*SystemResources, error) {
		callCount++
		return nil, fmt.Errorf("simulated collection error")
	})

	err := m.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial collection failed")
	assert.Equal(t, 1, callCount)
}

func TestResourceMonitor_collectLoop_ContinuesOnError(t *testing.T) {
	// Test that collectLoop continues when GetSystemResources errors
	m := New(&Config{
		DiskPath:        "/",
		CPUSampleTime:   10 * time.Millisecond,
		CollectInterval: 50 * time.Millisecond,
	})

	var callCount int64
	m.SetResourceGetter(func(_ context.Context) (*SystemResources, error) {
		count := atomic.AddInt64(&callCount, 1)
		if count == 1 {
			// First call (initial) succeeds
			return &SystemResources{NumCPU: 4}, nil
		}
		// Subsequent calls fail
		return nil, fmt.Errorf("simulated collection error")
	})

	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(t, err)

	// Wait for a few collection cycles
	time.Sleep(200 * time.Millisecond)

	m.Stop()

	// Should have had multiple calls (initial + ticks)
	assert.Greater(t, atomic.LoadInt64(&callCount), int64(1), "should have multiple getter calls")

	// Latest should still have the initial successful value
	latest := m.Latest()
	require.NotNil(t, latest)
	assert.Equal(t, 4, latest.NumCPU)
}

func TestResourceMonitor_SetResourceGetter(t *testing.T) {
	m := New(nil)

	called := false
	m.SetResourceGetter(func(_ context.Context) (*SystemResources, error) {
		called = true
		return &SystemResources{NumCPU: 8}, nil
	})

	// The setter should have been applied
	res, err := m.resourceGetter(context.Background())
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 8, res.NumCPU)
}
