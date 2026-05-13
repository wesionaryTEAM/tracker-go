// Package trackergin provides a Gin middleware for @wesionaryTEAM/tracker.
// Import path: github.com/wesionaryTEAM/tracker-go/gin
package trackergin

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	tracker "github.com/wesionaryTEAM/tracker-go"
)

// Middleware returns a gin.HandlerFunc that captures panics and gin errors.
//
// Panics are captured, then re-panicked so Gin's own recovery middleware
// (if registered after this one) can still handle them. Gin errors added
// via c.Error() are captured after c.Next() returns.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch v := r.(type) {
				case error:
					err = v
				default:
					err = fmt.Errorf("%v", v)
				}
				tracker.CaptureError(err, map[string]any{
					"method": c.Request.Method,
					"path":   c.FullPath(),
					"status": http.StatusInternalServerError,
				})
				panic(r) // re-panic for outer handlers
			}
		}()

		c.Next()

		if len(c.Errors) > 0 {
			last := c.Errors.Last()
			tracker.CaptureError(last.Err, map[string]any{
				"method": c.Request.Method,
				"path":   c.FullPath(),
				"status": c.Writer.Status(),
			})
		}
	}
}
