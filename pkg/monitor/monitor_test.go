package monitor

import (
	"context"
	"runtime"
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
