package gin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"digital.vasic.concurrency/pkg/semaphore"
)

// SemaphoreMiddleware returns a Gin middleware that limits concurrent
// request handling using the provided semaphore. When the semaphore
// cannot be acquired (server is at capacity), requests receive a
// 503 Service Unavailable response.
func SemaphoreMiddleware(sem *semaphore.Semaphore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sem.TryAcquire(1) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "server too busy, try again later",
			})
			return
		}
		defer sem.Release(1)
		c.Next()
	}
}
