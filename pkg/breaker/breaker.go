package breaker

import (
	"fmt"
	"sync"
	"time"
)

// State represents the current state of the circuit breaker.
type State int

const (
	Closed   State = iota // Normal operation — requests pass through
	Open                  // Failing — requests are rejected immediately
	HalfOpen              // Probing — limited requests pass through
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Config holds configuration for the circuit breaker.
type Config struct {
	MaxFailures      int           // Failures before opening
	Timeout          time.Duration // How long to stay open
	HalfOpenRequests int           // Probe requests in half-open
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxFailures:      5,
		Timeout:          30 * time.Second,
		HalfOpenRequests: 1,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	config          *Config
	state           State
	failures        int
	successes       int
	halfOpenAllowed int
	lastFailure     time.Time
	mu              sync.Mutex
}

// New creates a new CircuitBreaker with the given configuration.
func New(cfg *Config) *CircuitBreaker {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.HalfOpenRequests <= 0 {
		cfg.HalfOpenRequests = 1
	}
	return &CircuitBreaker{
		config: cfg,
		state:  Closed,
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.checkStateTransition()
	return cb.state
}

// Execute wraps fn with circuit breaker protection. If the circuit
// is open, it returns an error without calling fn. In half-open
// state, only a limited number of calls are allowed through.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	cb.checkStateTransition()

	switch cb.state {
	case Open:
		cb.mu.Unlock()
		return fmt.Errorf("circuit breaker is open")

	case HalfOpen:
		if cb.halfOpenAllowed <= 0 {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker is open")
		}
		cb.halfOpenAllowed--
		cb.mu.Unlock()

		err := fn()

		cb.mu.Lock()
		defer cb.mu.Unlock()
		if err != nil {
			cb.toOpen()
			return err
		}
		cb.successes++
		if cb.successes >= cb.config.HalfOpenRequests {
			cb.toClosed()
		}
		return nil

	default: // Closed
		cb.mu.Unlock()

		err := fn()

		cb.mu.Lock()
		defer cb.mu.Unlock()
		if err != nil {
			cb.failures++
			cb.lastFailure = time.Now()
			if cb.failures >= cb.config.MaxFailures {
				cb.toOpen()
			}
			return err
		}
		cb.failures = 0
		return nil
	}
}

// Reset forces the circuit breaker back to the closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.toClosed()
}

// Failures returns the current consecutive failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

// checkStateTransition handles automatic state transitions.
// Must be called with the mutex held.
func (cb *CircuitBreaker) checkStateTransition() {
	if cb.state == Open &&
		time.Since(cb.lastFailure) >= cb.config.Timeout {
		cb.toHalfOpen()
	}
}

func (cb *CircuitBreaker) toOpen() {
	cb.state = Open
	cb.lastFailure = time.Now()
	cb.successes = 0
	cb.halfOpenAllowed = 0
}

func (cb *CircuitBreaker) toHalfOpen() {
	cb.state = HalfOpen
	cb.successes = 0
	cb.halfOpenAllowed = cb.config.HalfOpenRequests
}

func (cb *CircuitBreaker) toClosed() {
	cb.state = Closed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenAllowed = 0
}
