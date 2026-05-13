package trackergin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	tracker "github.com/wesionaryTEAM/tracker-go"
	trackergin "github.com/wesionaryTEAM/tracker-go/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func capturePayload(t *testing.T) (*httptest.Server, chan tracker.ErrorPayload) {
	t.Helper()
	ch := make(chan tracker.ErrorPayload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p tracker.ErrorPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode: %v", err)
		}
		ch <- p
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, ch
}

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

func TestMiddleware_CapturesPanic(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	router := gin.New()
	router.Use(trackergin.Middleware())
	router.GET("/boom", func(c *gin.Context) {
		panic("handler exploded")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()

	// Gin's default recovery will catch the re-panic; we just need the payload.
	func() {
		defer func() { recover() }()
		router.ServeHTTP(w, req)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tracker.Flush(ctx) //nolint:errcheck

	p := wait(t, ch)
	if p.Error.Message != "handler exploded" {
		t.Errorf("message = %q, want %q", p.Error.Message, "handler exploded")
	}
	if p.Context.Custom["method"] != http.MethodGet {
		t.Errorf("custom.method = %v, want GET", p.Context.Custom["method"])
	}
}

func TestMiddleware_CapturesGinErrors(t *testing.T) {
	srv, ch := capturePayload(t)
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, srv.URL)
	t.Cleanup(tracker.Reset)

	router := gin.New()
	router.Use(trackergin.Middleware())
	router.GET("/err", func(c *gin.Context) {
		_ = c.Error(http.ErrNoCookie) // add error to gin's error chain
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tracker.Flush(ctx) //nolint:errcheck

	p := wait(t, ch)
	if p.Error.Message != http.ErrNoCookie.Error() {
		t.Errorf("message = %q, want %q", p.Error.Message, http.ErrNoCookie.Error())
	}
}

func TestMiddleware_PassesThrough_NoError(t *testing.T) {
	// No ingest server needed — any unexpected POST would fail to connect.
	tracker.InitWithEndpoint(tracker.Config{APIKey: "k"}, "http://127.0.0.1:1")
	t.Cleanup(tracker.Reset)

	router := gin.New()
	router.Use(trackergin.Middleware())
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
