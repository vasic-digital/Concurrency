package gin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.concurrency/pkg/semaphore"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(sem *semaphore.Semaphore) *gin.Engine {
	r := gin.New()
	r.Use(SemaphoreMiddleware(sem))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestSemaphoreMiddleware_AllowsWithinLimit(t *testing.T) {
	sem := semaphore.New(10)
	router := setupRouter(sem)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
}

func TestSemaphoreMiddleware_BlocksWhenFull(t *testing.T) {
	sem := semaphore.New(1)

	// Exhaust the semaphore
	acquired := sem.TryAcquire(1)
	require.True(t, acquired, "should acquire the only slot")

	router := setupRouter(sem)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "server too busy, try again later", body["error"])

	// Release and verify subsequent requests succeed
	sem.Release(1)

	w2 := httptest.NewRecorder()
	req2, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)

	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestSemaphoreMiddleware_ReleasesAfterRequest(t *testing.T) {
	sem := semaphore.New(1)
	router := setupRouter(sem)

	// First request
	w1 := httptest.NewRecorder()
	req1, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request — should also succeed because the first
	// released its slot
	w2 := httptest.NewRecorder()
	req2, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Semaphore should be fully available
	assert.Equal(t, int64(0), sem.Current())
}

func TestSemaphoreMiddleware_ConcurrentRequests(t *testing.T) {
	const maxConcurrent int64 = 5
	const totalRequests = 10

	sem := semaphore.New(maxConcurrent)

	// Use a handler that blocks until all goroutines have fired
	// their requests, ensuring maximum contention.
	var activeCount atomic.Int64
	var peakCount atomic.Int64
	barrier := make(chan struct{})

	r := gin.New()
	r.Use(SemaphoreMiddleware(sem))
	r.GET("/test", func(c *gin.Context) {
		current := activeCount.Add(1)
		// Track peak concurrency
		for {
			old := peakCount.Load()
			if current <= old || peakCount.CompareAndSwap(old, current) {
				break
			}
		}
		<-barrier
		activeCount.Add(-1)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var busyCount atomic.Int64

	recorders := make([]*httptest.ResponseRecorder, totalRequests)
	for i := 0; i < totalRequests; i++ {
		recorders[i] = httptest.NewRecorder()
	}

	// Launch all requests concurrently
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodGet, "/test", nil)
			r.ServeHTTP(recorders[idx], req)
			if recorders[idx].Code == http.StatusOK {
				successCount.Add(1)
			} else if recorders[idx].Code == http.StatusServiceUnavailable {
				busyCount.Add(1)
			}
		}(i)
	}

	// Wait a moment for goroutines to start, then release the barrier
	// We need the blocked handlers to finish
	// Use a polling approach: wait until we see some busy responses
	// or the active count reaches max
	for {
		if activeCount.Load() >= maxConcurrent {
			break
		}
		if busyCount.Load() > 0 {
			break
		}
	}

	// Release the barrier so blocked handlers can complete
	close(barrier)
	wg.Wait()

	successes := successCount.Load()
	busies := busyCount.Load()

	// At least maxConcurrent should succeed (they got through)
	assert.GreaterOrEqual(t, successes, maxConcurrent,
		"at least %d requests should succeed", maxConcurrent)
	// Total should add up
	assert.Equal(t, int64(totalRequests), successes+busies,
		"all requests should either succeed or get 503")
	// Peak concurrency should not exceed the semaphore limit
	assert.LessOrEqual(t, peakCount.Load(), maxConcurrent,
		"peak concurrency should not exceed semaphore limit")
}
