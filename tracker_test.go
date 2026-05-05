package tracker_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tracker "github.com/wesionary-team/tracker-go"
)

// capturePayload starts an httptest.Server that decodes the first payload
// posted to it and sends it on the returned channel.
func capturePayload(t *testing.T) (*httptest.Server, chan tracker.ErrorPayload) {
	t.Helper()
	ch := make(chan tracker.ErrorPayload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p tracker.ErrorPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		ch <- p
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, ch
}

// wait drains ch with a 2-second timeout, failing the test if nothing arrives.
func wait(t *testing.T, ch chan tracker.ErrorPayload) tracker.ErrorPayload {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for payload")
		return tracker.ErrorPayload{}
	}
}

func flush(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tracker.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestCaptureError_PostsCorrectPayload(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "test-key", Environment: "test"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.CaptureError(errors.New("something broke"))
	flush(t)

	p := wait(t, ch)
	if p.APIKey != "test-key" {
		t.Errorf("apiKey = %q, want %q", p.APIKey, "test-key")
	}
	if p.Error.Message != "something broke" {
		t.Errorf("error.message = %q, want %q", p.Error.Message, "something broke")
	}
	if p.Source != "backend" {
		t.Errorf("source = %q, want %q", p.Source, "backend")
	}
	if p.Environment != "test" {
		t.Errorf("environment = %q, want %q", p.Environment, "test")
	}
	if p.SessionID == "" {
		t.Error("sessionId must not be empty")
	}
	if p.ID == "" {
		t.Error("id must not be empty")
	}
}

func TestCaptureError_IncludesStackTrace(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.CaptureError(errors.New("trace test"))
	flush(t)

	p := wait(t, ch)
	if len(p.Error.StackTrace) == 0 {
		t.Fatal("expected non-empty stack trace")
	}
	top := p.Error.StackTrace[0].File
	if !strings.Contains(top, "tracker_test.go") {
		t.Errorf("top frame file = %q, want tracker_test.go", top)
	}
}

func TestCaptureError_WithExtraContext(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.CaptureError(errors.New("ctx test"), map[string]any{"method": "GET", "status": 500})
	flush(t)

	p := wait(t, ch)
	if p.Context.Custom["method"] != "GET" {
		t.Errorf("custom.method = %v, want GET", p.Context.Custom["method"])
	}
}

func TestCaptureMessage_SetsType(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.CaptureMessage("hello world", tracker.LevelWarn)
	flush(t)

	p := wait(t, ch)
	if p.Error.Message != "hello world" {
		t.Errorf("message = %q, want %q", p.Error.Message, "hello world")
	}
	if p.Error.Type != "Message[warn]" {
		t.Errorf("type = %q, want %q", p.Error.Type, "Message[warn]")
	}
}

func TestSetUser_AttachesToPayload(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.SetUser(&tracker.UserContext{ID: "u1", Email: "a@b.com", Name: "Alice"})
	tracker.CaptureError(errors.New("user test"))
	flush(t)

	p := wait(t, ch)
	if p.User == nil {
		t.Fatal("expected user, got nil")
	}
	if p.User.ID != "u1" {
		t.Errorf("user.id = %q, want %q", p.User.ID, "u1")
	}
}

func TestSetUser_ClearsWithNil(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.SetUser(&tracker.UserContext{ID: "u1"})
	tracker.SetUser(nil)
	tracker.CaptureError(errors.New("no user"))
	flush(t)

	p := wait(t, ch)
	if p.User != nil {
		t.Errorf("expected nil user, got %+v", p.User)
	}
}

func TestAddBreadcrumb_AttachesToPayload(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.AddBreadcrumb(tracker.Breadcrumb{Message: "clicked button", Category: "ui"})
	tracker.CaptureError(errors.New("after click"))
	flush(t)

	p := wait(t, ch)
	if len(p.Breadcrumbs) == 0 {
		t.Fatal("expected breadcrumbs, got none")
	}
	if p.Breadcrumbs[0].Message != "clicked button" {
		t.Errorf("breadcrumb[0].message = %q, want %q", p.Breadcrumbs[0].Message, "clicked button")
	}
}

func TestWithContext_AttachesToCustom(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.WithContext("requestId", "req-123")
	tracker.CaptureError(errors.New("ctx test"))
	flush(t)

	p := wait(t, ch)
	if p.Context.Custom["requestId"] != "req-123" {
		t.Errorf("custom.requestId = %v, want req-123", p.Context.Custom["requestId"])
	}
}

func TestRecover_CapturesPanicAndRepanics(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	// outer recover prevents the test itself from crashing
	func() {
		defer func() { recover() }()
		func() {
			defer tracker.Recover()
			panic("test panic value")
		}()
	}()

	flush(t)
	p := wait(t, ch)
	if p.Error.Message != "test panic value" {
		t.Errorf("panic message = %q, want %q", p.Error.Message, "test panic value")
	}
}

func TestFlush_ReturnsOnContextCancel(t *testing.T) {
	// Server that never responds — ensures Flush respects ctx cancellation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	tracker.CaptureError(errors.New("slow"))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := tracker.Flush(ctx)
	if err == nil {
		t.Error("expected context error, got nil")
	}
}

func TestNoop_BeforeInit(t *testing.T) {
	tracker.Reset() // ensure no client
	// All calls before Init should be silent no-ops (no panic).
	tracker.CaptureError(errors.New("no init"))
	tracker.CaptureMessage("msg", tracker.LevelInfo)
	tracker.SetUser(nil)
	tracker.AddBreadcrumb(tracker.Breadcrumb{Message: "x"})
	tracker.WithContext("k", "v")
	tracker.Recover()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	tracker.Flush(ctx) //nolint:errcheck
}
