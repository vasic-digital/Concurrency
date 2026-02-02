package monitor

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemResources holds a snapshot of system resource usage.
type SystemResources struct {
	// CPU
	CPUPercent  float64   // Overall CPU usage percentage
	CPUPerCore  []float64 // Per-core CPU usage percentages
	NumCPU      int       // Number of logical CPUs

	// Memory
	MemoryTotal     uint64  // Total memory in bytes
	MemoryUsed      uint64  // Used memory in bytes
	MemoryAvailable uint64  // Available memory in bytes
	MemoryPercent   float64 // Memory usage percentage

	// Disk
	DiskTotal   uint64  // Total disk space in bytes
	DiskUsed    uint64  // Used disk space in bytes
	DiskFree    uint64  // Free disk space in bytes
	DiskPercent float64 // Disk usage percentage

	// Load averages (Unix only)
	Load1  float64 // 1-minute load average
	Load5  float64 // 5-minute load average
	Load15 float64 // 15-minute load average

	// Go runtime
	NumGoroutines int    // Number of active goroutines
	HeapAlloc     uint64 // Heap memory allocated in bytes
	HeapSys       uint64 // Heap memory obtained from OS

	// Timestamp
	CollectedAt time.Time
}

// Config holds configuration for the resource monitor.
type Config struct {
	DiskPath        string        // Path to check disk usage (default "/")
	CPUSampleTime   time.Duration // Duration to sample CPU (default 200ms)
	CollectInterval time.Duration // Interval for background collection
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DiskPath:        "/",
		CPUSampleTime:   200 * time.Millisecond,
		CollectInterval: 5 * time.Second,
	}
}

// ResourceMonitor collects system resource snapshots.
type ResourceMonitor struct {
	config *Config
	latest *SystemResources
	mu     sync.RWMutex
	cancel context.CancelFunc
}

// New creates a new ResourceMonitor with the given configuration.
func New(cfg *Config) *ResourceMonitor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.DiskPath == "" {
		cfg.DiskPath = "/"
	}
	if cfg.CPUSampleTime <= 0 {
		cfg.CPUSampleTime = 200 * time.Millisecond
	}
	return &ResourceMonitor{
		config: cfg,
	}
}

// GetSystemResources collects and returns a snapshot of current
// system resources. This method performs real I/O and may take
// up to CPUSampleTime to complete.
func (m *ResourceMonitor) GetSystemResources(
	ctx context.Context,
) (*SystemResources, error) {
	res := &SystemResources{
		NumCPU:      runtime.NumCPU(),
		CollectedAt: time.Now(),
	}

	// Go runtime stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	res.NumGoroutines = runtime.NumGoroutine()
	res.HeapAlloc = memStats.HeapAlloc
	res.HeapSys = memStats.HeapSys

	// CPU
	cpuPercent, err := cpu.PercentWithContext(
		ctx, m.config.CPUSampleTime, false,
	)
	if err == nil && len(cpuPercent) > 0 {
		res.CPUPercent = cpuPercent[0]
	}

	cpuPerCore, err := cpu.PercentWithContext(
		ctx, 0, true,
	)
	if err == nil {
		res.CPUPerCore = cpuPerCore
	}

	// Memory
	vmem, err := mem.VirtualMemoryWithContext(ctx)
	if err == nil && vmem != nil {
		res.MemoryTotal = vmem.Total
		res.MemoryUsed = vmem.Used
		res.MemoryAvailable = vmem.Available
		res.MemoryPercent = vmem.UsedPercent
	}

	// Disk
	diskUsage, err := disk.UsageWithContext(ctx, m.config.DiskPath)
	if err == nil && diskUsage != nil {
		res.DiskTotal = diskUsage.Total
		res.DiskUsed = diskUsage.Used
		res.DiskFree = diskUsage.Free
		res.DiskPercent = diskUsage.UsedPercent
	}

	// Load averages
	loadAvg, err := load.AvgWithContext(ctx)
	if err == nil && loadAvg != nil {
		res.Load1 = loadAvg.Load1
		res.Load5 = loadAvg.Load5
		res.Load15 = loadAvg.Load15
	}

	return res, nil
}

// Start begins background resource collection at the configured
// interval. Use Latest() to retrieve the most recent snapshot.
func (m *ResourceMonitor) Start(ctx context.Context) error {
	if m.config.CollectInterval <= 0 {
		return fmt.Errorf("collect interval must be positive")
	}

	ctx, m.cancel = context.WithCancel(ctx)

	// Collect once immediately
	res, err := m.GetSystemResources(ctx)
	if err != nil {
		return fmt.Errorf("initial collection failed: %w", err)
	}
	m.mu.Lock()
	m.latest = res
	m.mu.Unlock()

	go m.collectLoop(ctx)
	return nil
}

// Stop halts background collection.
func (m *ResourceMonitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// Latest returns the most recently collected resource snapshot.
// Returns nil if no collection has occurred.
func (m *ResourceMonitor) Latest() *SystemResources {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latest
}

func (m *ResourceMonitor) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res, err := m.GetSystemResources(ctx)
			if err != nil {
				continue
			}
			m.mu.Lock()
			m.latest = res
			m.mu.Unlock()
		}
	}
}
