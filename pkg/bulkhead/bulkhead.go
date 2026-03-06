// Package bulkhead implements the bulkhead pattern for resource isolation.
//
// A bulkhead limits concurrent access to a resource, preventing a single
// component from consuming all available capacity. It uses a semaphore
// to control concurrency with configurable timeout.
//
// Design pattern: Bulkhead (resource isolation).
package bulkhead

import (
	"context"
	"fmt"
	"time"
)

// Logger is a minimal logging interface.
type Logger interface {
	Warn(msg string, keysAndValues ...interface{})
}

// Config contains configuration for the bulkhead.
type Config struct {
	MaxConcurrent int
	QueueSize     int
	Timeout       time.Duration
	Logger        Logger
}

// Bulkhead implements the bulkhead pattern for resource isolation.
type Bulkhead struct {
	semaphore chan struct{}
	config    Config
	logger    Logger
}

// New creates a new bulkhead with the given configuration.
func New(cfg Config) *Bulkhead {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 10
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 100
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	b := &Bulkhead{
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
		config:    cfg,
		logger:    cfg.Logger,
	}

	// Pre-fill semaphore with permits
	for i := 0; i < cfg.MaxConcurrent; i++ {
		b.semaphore <- struct{}{}
	}

	return b
}

// Execute executes fn with bulkhead protection. It blocks until a permit
// is available, the context is cancelled, or the timeout expires.
func (b *Bulkhead) Execute(ctx context.Context, fn func() error) error {
	select {
	case <-b.semaphore:
		defer func() {
			b.semaphore <- struct{}{}
		}()
		return fn()
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(b.config.Timeout):
		if b.logger != nil {
			b.logger.Warn("bulkhead timeout", "timeout", b.config.Timeout)
		}
		return fmt.Errorf("bulkhead: timeout after %v", b.config.Timeout)
	}
}

// Stats returns bulkhead statistics.
type Stats struct {
	MaxConcurrent   int `json:"max_concurrent"`
	AvailablePermit int `json:"available_permits"`
	QueueSize       int `json:"queue_size"`
}

// GetStats returns current bulkhead statistics.
func (b *Bulkhead) GetStats() Stats {
	return Stats{
		MaxConcurrent:   b.config.MaxConcurrent,
		AvailablePermit: len(b.semaphore),
		QueueSize:       b.config.QueueSize,
	}
}
