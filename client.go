package tracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultEndpoint    = "https://wtracker.manjish.com/errors/ingest"
	defaultEnvironment = "production"
	maxBreadcrumbs     = 20
)

type resolvedConfig struct {
	apiKey      string
	endpoint    string
	environment string
	release     string
	debug       bool
}

type client struct {
	cfg       resolvedConfig
	user      *UserContext
	crumbs    []Breadcrumb
	customCtx map[string]any
	sessionID string
	mu        sync.RWMutex
	pending   sync.WaitGroup
	http      *http.Client
}

func newClient(cfg Config) *client {
	env := cfg.Environment
	if env == "" {
		env = defaultEnvironment
	}
	return &client{
		cfg: resolvedConfig{
			apiKey:      cfg.APIKey,
			endpoint:    defaultEndpoint,
			environment: env,
			release:     cfg.Release,
			debug:       cfg.Debug,
		},
		crumbs:    []Breadcrumb{},
		customCtx: make(map[string]any),
		sessionID: uuid.New().String(),
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func newClientWithEndpoint(cfg Config, endpoint string) *client {
	c := newClient(cfg)
	c.cfg.endpoint = endpoint
	return c
}

func (c *client) setUser(u *UserContext) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.user = u
}

func (c *client) addBreadcrumb(b Breadcrumb) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if b.Timestamp == "" {
		b.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	c.crumbs = append(c.crumbs, b)
	if len(c.crumbs) > maxBreadcrumbs {
		trimmed := make([]Breadcrumb, maxBreadcrumbs)
		copy(trimmed, c.crumbs[len(c.crumbs)-maxBreadcrumbs:])
		c.crumbs = trimmed
	}
}

func (c *client) withContext(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.customCtx[key] = value
}

// captureError captures err with the current call stack (skip=2: captureError + CaptureError).
func (c *client) captureError(err error, extraCtx map[string]any) {
	stack := captureStack(2)
	p := c.buildPayloadWithType(err.Error(), fmt.Sprintf("%T", err), stack, extraCtx)
	c.logf("Capturing error: %s", err.Error())
	c.send(p)
}

func (c *client) captureMessage(msg string, level Level) {
	stack := captureStack(2)
	p := c.buildPayloadWithType(msg, fmt.Sprintf("Message[%s]", level), stack, nil)
	c.logf("Capturing message: %s", msg)
	c.send(p)
}

// captureRecovered captures a panic value r that has already been recovered by the caller.
func (c *client) captureRecovered(r any) {
	var err error
	switch v := r.(type) {
	case error:
		err = v
	default:
		err = fmt.Errorf("%v", v)
	}
	stack := parsePanicStack(debug.Stack())
	p := c.buildPayloadWithType(err.Error(), fmt.Sprintf("%T", err), stack, nil)
	c.logf("Capturing panic: %s", err.Error())
	c.send(p)
}

func (c *client) buildPayloadWithType(errMsg, errType string, stack []StackFrame, extraCtx map[string]any) ErrorPayload {
	c.mu.RLock()
	user := c.user
	crumbs := make([]Breadcrumb, len(c.crumbs))
	copy(crumbs, c.crumbs)
	custom := make(map[string]any, len(c.customCtx)+len(extraCtx))
	for k, v := range c.customCtx {
		custom[k] = v
	}
	c.mu.RUnlock()

	for k, v := range extraCtx {
		custom[k] = v
	}

	return ErrorPayload{
		ID:          uuid.New().String(),
		APIKey:      c.cfg.apiKey,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Environment: c.cfg.environment,
		Release:     c.cfg.release,
		Source:      "backend",
		Error: errorDetail{
			Message:    errMsg,
			Type:       errType,
			StackTrace: stack,
		},
		Context: nodeContext{
			OS: osInfo{
				Name:    runtime.GOOS,
				Version: osVersion(),
			},
			Custom: custom,
		},
		User:        user,
		Breadcrumbs: crumbs,
		SessionID:   c.sessionID,
	}
}

func (c *client) send(p ErrorPayload) {
	c.pending.Add(1)
	go func() {
		defer c.pending.Done()
		data, err := json.Marshal(p)
		if err != nil {
			c.logf("marshal error: %v", err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, c.cfg.endpoint, bytes.NewReader(data))
		if err != nil {
			c.logf("request error: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.cfg.apiKey)
		resp, err := c.http.Do(req)
		if err != nil {
			c.logf("send error: %v", err)
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		c.logf("sent: status %d", resp.StatusCode)
	}()
}

func (c *client) flush(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		c.pending.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *client) logf(format string, args ...any) {
	if c.cfg.debug {
		fmt.Printf("[tracker] "+format+"\n", args...)
	}
}
