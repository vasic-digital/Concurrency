package bulkhead

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements the Logger interface for testing.
type mockLogger struct {
	warnings []string
}

func (m *mockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.warnings = append(m.warnings, msg)
}

// TestExecute_TimeoutWithLogger tests the timeout path with a logger
// configured, covering the logger.Warn branch in Execute.
func TestExecute_TimeoutWithLogger(t *testing.T) {
	logger := &mockLogger{}
	b := New(Config{
		MaxConcurrent: 1,
		Timeout:       50 * time.Millisecond,
		Logger:        logger,
	})

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

	// This should timeout and log
	err := b.Execute(context.Background(), func() error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.Len(t, logger.warnings, 1)
	assert.Equal(t, "bulkhead timeout", logger.warnings[0])

	close(done)
}

// TestExecute_TimeoutWithNilLogger tests the timeout path without a logger,
// covering the nil logger branch.
func TestExecute_TimeoutWithNilLogger(t *testing.T) {
	b := New(Config{
		MaxConcurrent: 1,
		Timeout:       50 * time.Millisecond,
		Logger:        nil,
	})

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
