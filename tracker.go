package tracker

import (
	"context"
	"sync"
)

var (
	globalClient *client
	globalMu     sync.RWMutex
)

// Init initialises the global tracker client. Safe to call multiple times.
func Init(cfg Config) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalClient = newClient(cfg)
}

func getClient() *client {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalClient
}

// CaptureError sends err to the ingest endpoint with the current stack trace.
// Optional ctx map is merged into the payload's custom context.
func CaptureError(err error, ctx ...map[string]any) {
	c := getClient()
	if c == nil {
		return
	}
	var extra map[string]any
	if len(ctx) > 0 {
		extra = ctx[0]
	}
	c.captureError(err, extra)
}

// CaptureMessage sends a string message as an error payload.
func CaptureMessage(msg string, level Level) {
	if c := getClient(); c != nil {
		c.captureMessage(msg, level)
	}
}

// SetUser attaches user identity to all subsequent errors. Pass nil to clear.
func SetUser(u *UserContext) {
	if c := getClient(); c != nil {
		c.setUser(u)
	}
}

// AddBreadcrumb appends a breadcrumb to the ring buffer (capped at 20).
func AddBreadcrumb(b Breadcrumb) {
	if c := getClient(); c != nil {
		c.addBreadcrumb(b)
	}
}

// WithContext attaches a key-value pair to all subsequent error payloads.
func WithContext(key string, value any) {
	if c := getClient(); c != nil {
		c.withContext(key, value)
	}
}

// Recover must be called as `defer tracker.Recover()`. It catches any panic,
// captures it, then re-panics so the program still crashes normally.
func Recover() {
	r := recover()
	if r == nil {
		return
	}
	if c := getClient(); c != nil {
		c.captureRecovered(r)
	}
	panic(r)
}

// Flush blocks until all in-flight sends complete or ctx expires.
func Flush(ctx context.Context) error {
	c := getClient()
	if c == nil {
		return nil
	}
	return c.flush(ctx)
}

// Reset clears the global client. For tests only.
func Reset() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalClient = nil
}
