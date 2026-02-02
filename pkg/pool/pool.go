package pool

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents a unit of work to be executed by the worker pool
type Task interface {
	// ID returns a unique identifier for the task
	ID() string
	// Execute runs the task and returns the result
	Execute(ctx context.Context) (interface{}, error)
}

// TaskFunc wraps a function as a Task
type TaskFunc struct {
	id string
	fn func(ctx context.Context) (interface{}, error)
}

// NewTaskFunc creates a new TaskFunc with the given ID and function
func NewTaskFunc(
	id string,
	fn func(ctx context.Context) (interface{}, error),
) *TaskFunc {
	return &TaskFunc{id: id, fn: fn}
}

func (t *TaskFunc) ID() string { return t.id }
func (t *TaskFunc) Execute(
	ctx context.Context,
) (interface{}, error) {
	return t.fn(ctx)
}

// Result represents the outcome of a task execution
type Result struct {
	TaskID    string
	Value     interface{}
	Error     error
	StartTime time.Time
	Duration  time.Duration
}

// PoolConfig holds configuration for the worker pool
type PoolConfig struct {
	Workers       int                            // Number of concurrent workers
	QueueSize     int                            // Size of the task queue
	TaskTimeout   time.Duration                  // Maximum time for a single task
	ShutdownGrace time.Duration                  // Grace period during shutdown
	OnError       func(taskID string, err error) // Error callback
	OnComplete    func(result Result)            // Completion callback
}

// DefaultPoolConfig returns a default configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		Workers:       runtime.NumCPU(),
		QueueSize:     1000,
		TaskTimeout:   30 * time.Second,
		ShutdownGrace: 5 * time.Second,
	}
}

// PoolMetrics tracks worker pool statistics
type PoolMetrics struct {
	ActiveWorkers  int64
	QueuedTasks    int64
	CompletedTasks int64
	FailedTasks    int64
	TotalLatencyUs int64 // Total latency in microseconds
	TaskCount      int64 // For calculating average
}

// AverageLatency returns the average task latency
func (m *PoolMetrics) AverageLatency() time.Duration {
	count := atomic.LoadInt64(&m.TaskCount)
	if count == 0 {
		return 0
	}
	totalUs := atomic.LoadInt64(&m.TotalLatencyUs)
	return time.Duration(totalUs/count) * time.Microsecond
}

// WorkerPool provides bounded concurrency with configurable workers
type WorkerPool struct {
	config  *PoolConfig
	tasks   chan Task
	results chan Result
	sem     chan struct{}
	metrics *PoolMetrics
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	closed  bool
	mu      sync.Mutex
}

// NewWorkerPool creates a new worker pool with the given configuration
func NewWorkerPool(config *PoolConfig) *WorkerPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		config:  config,
		tasks:   make(chan Task, config.QueueSize),
		results: make(chan Result, config.QueueSize),
		sem:     make(chan struct{}, config.Workers),
		metrics: &PoolMetrics{},
		ctx:     ctx,
		cancel:  cancel,
	}

	return pool
}

// Start begins the worker pool processing
func (p *WorkerPool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startLocked()
}

// startLocked starts the pool assuming the lock is already held
func (p *WorkerPool) startLocked() {
	if p.started || p.closed {
		return
	}

	p.started = true

	// Start workers
	for i := 0; i < p.config.Workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// worker processes tasks from the queue
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.tasks:
			if !ok {
				return
			}

			// Acquire semaphore
			select {
			case p.sem <- struct{}{}:
				atomic.AddInt64(&p.metrics.ActiveWorkers, 1)
			case <-p.ctx.Done():
				return
			}

			// Execute the task
			result := p.executeTask(task)

			// Release semaphore
			<-p.sem
			atomic.AddInt64(&p.metrics.ActiveWorkers, -1)

			// Send result (non-blocking if channel is full)
			select {
			case p.results <- result:
			default:
				// Result channel full, drop or log
			}

			// Call callbacks
			if result.Error != nil && p.config.OnError != nil {
				p.config.OnError(result.TaskID, result.Error)
			}
			if p.config.OnComplete != nil {
				p.config.OnComplete(result)
			}
		}
	}
}

// executeTask runs a single task with timeout
func (p *WorkerPool) executeTask(task Task) Result {
	startTime := time.Now()

	// Create context with timeout
	ctx := p.ctx
	if p.config.TaskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(p.ctx, p.config.TaskTimeout)
		defer cancel()
	}

	// Execute the task
	value, err := task.Execute(ctx)
	duration := time.Since(startTime)

	// Update metrics
	atomic.AddInt64(
		&p.metrics.TotalLatencyUs,
		int64(duration.Microseconds()),
	)
	atomic.AddInt64(&p.metrics.TaskCount, 1)
	if err != nil {
		atomic.AddInt64(&p.metrics.FailedTasks, 1)
	} else {
		atomic.AddInt64(&p.metrics.CompletedTasks, 1)
	}

	return Result{
		TaskID:    task.ID(),
		Value:     value,
		Error:     err,
		StartTime: startTime,
		Duration:  duration,
	}
}

// Submit adds a task to the pool
// Returns an error if the pool is closed or the queue is full
func (p *WorkerPool) Submit(task Task) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return fmt.Errorf("pool is closed")
	}
	if !p.started {
		p.startLocked()
	}
	p.mu.Unlock()

	atomic.AddInt64(&p.metrics.QueuedTasks, 1)

	select {
	case p.tasks <- task:
		return nil
	case <-p.ctx.Done():
		atomic.AddInt64(&p.metrics.QueuedTasks, -1)
		return fmt.Errorf("pool context cancelled")
	default:
		atomic.AddInt64(&p.metrics.QueuedTasks, -1)
		return fmt.Errorf("task queue is full")
	}
}

// SubmitWait submits a task and waits for its result
func (p *WorkerPool) SubmitWait(
	ctx context.Context,
	task Task,
) (Result, error) {
	if err := p.Submit(task); err != nil {
		return Result{}, err
	}

	// Wait for the specific result
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case result := <-p.results:
			if result.TaskID == task.ID() {
				return result, result.Error
			}
			// Put back non-matching results
			select {
			case p.results <- result:
			default:
			}
		}
	}
}

// SubmitBatch submits multiple tasks and returns a channel of results
func (p *WorkerPool) SubmitBatch(tasks []Task) <-chan Result {
	resultChan := make(chan Result, len(tasks))

	go func() {
		defer close(resultChan)

		var submitted int
		taskIDs := make(map[string]bool)

		// Submit all tasks
		for _, task := range tasks {
			if err := p.Submit(task); err == nil {
				taskIDs[task.ID()] = true
				submitted++
			}
		}

		if submitted == 0 {
			return
		}

		// Collect results
		collected := 0
		for collected < submitted {
			select {
			case <-p.ctx.Done():
				return
			case result := <-p.results:
				if taskIDs[result.TaskID] {
					resultChan <- result
					collected++
				} else {
					// Put back non-matching results
					select {
					case p.results <- result:
					default:
					}
				}
			}
		}
	}()

	return resultChan
}

// SubmitBatchWait submits multiple tasks and waits for all results
func (p *WorkerPool) SubmitBatchWait(
	ctx context.Context,
	tasks []Task,
) ([]Result, error) {
	resultChan := p.SubmitBatch(tasks)
	results := make([]Result, 0, len(tasks))

	for result := range resultChan {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
			results = append(results, result)
		}
	}

	return results, nil
}

// Results returns the results channel
func (p *WorkerPool) Results() <-chan Result {
	return p.results
}

// Metrics returns a snapshot of pool metrics
func (p *WorkerPool) Metrics() *PoolMetrics {
	return &PoolMetrics{
		ActiveWorkers: atomic.LoadInt64(
			&p.metrics.ActiveWorkers,
		),
		QueuedTasks: atomic.LoadInt64(
			&p.metrics.QueuedTasks,
		),
		CompletedTasks: atomic.LoadInt64(
			&p.metrics.CompletedTasks,
		),
		FailedTasks: atomic.LoadInt64(
			&p.metrics.FailedTasks,
		),
		TotalLatencyUs: atomic.LoadInt64(
			&p.metrics.TotalLatencyUs,
		),
		TaskCount: atomic.LoadInt64(
			&p.metrics.TaskCount,
		),
	}
}

// QueueLength returns the current number of tasks in the queue
func (p *WorkerPool) QueueLength() int {
	return len(p.tasks)
}

// ActiveWorkers returns the number of currently active workers
func (p *WorkerPool) ActiveWorkers() int {
	return int(atomic.LoadInt64(&p.metrics.ActiveWorkers))
}

// IsRunning returns whether the pool is running
func (p *WorkerPool) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.started && !p.closed
}

// Shutdown gracefully shuts down the worker pool
func (p *WorkerPool) Shutdown(timeout time.Duration) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Close the task queue to signal workers to stop
	close(p.tasks)

	// Create a done channel for the waitgroup
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// Wait for workers with timeout
	if timeout <= 0 {
		timeout = p.config.ShutdownGrace
	}

	select {
	case <-done:
		// All workers finished
	case <-time.After(timeout):
		// Timeout, cancel remaining work
		p.cancel()
		<-done
	}

	// Close results channel
	close(p.results)

	return nil
}

// Stop immediately stops the worker pool
func (p *WorkerPool) Stop() {
	p.cancel()
	_ = p.Shutdown(0)
}

// WaitForDrain waits until all queued tasks are processed
func (p *WorkerPool) WaitForDrain(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if len(p.tasks) == 0 && p.ActiveWorkers() == 0 {
				return nil
			}
		}
	}
}

// ParallelExecute executes multiple functions in parallel and
// returns results. This is a convenience function for simple
// parallel execution.
func ParallelExecute(
	ctx context.Context,
	fns []func(ctx context.Context) (interface{}, error),
) ([]Result, error) {
	pool := NewWorkerPool(&PoolConfig{
		Workers:   len(fns),
		QueueSize: len(fns),
	})
	defer pool.Stop()

	tasks := make([]Task, len(fns))
	for i, fn := range fns {
		tasks[i] = NewTaskFunc(fmt.Sprintf("task-%d", i), fn)
	}

	return pool.SubmitBatchWait(ctx, tasks)
}

// Map applies a function to each element in parallel and returns
// results
func Map[T any, R any](
	ctx context.Context,
	items []T,
	workers int,
	fn func(ctx context.Context, item T) (R, error),
) ([]R, error) {
	p := NewWorkerPool(&PoolConfig{
		Workers:   workers,
		QueueSize: len(items),
	})
	defer p.Stop()

	tasks := make([]Task, len(items))
	for i, item := range items {
		item := item // Capture loop variable
		tasks[i] = NewTaskFunc(
			fmt.Sprintf("map-%d", i),
			func(ctx context.Context) (interface{}, error) {
				return fn(ctx, item)
			},
		)
	}

	results, err := p.SubmitBatchWait(ctx, tasks)
	if err != nil {
		return nil, err
	}

	output := make([]R, len(results))
	for i, result := range results {
		if result.Error != nil {
			return nil, result.Error
		}
		output[i] = result.Value.(R)
	}

	return output, nil
}
