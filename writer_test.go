package obs_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	obs "github.com/ly-solutions/go-obs"
)

// Streaming handlers type-assert http.Flusher; the middleware must not hide it.
func TestMiddlewareKeepsFlusher(t *testing.T) {
	o := obs.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/stream", func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("response writer does not implement http.Flusher")
			return
		}
		_, _ = io.WriteString(w, "data: 1\n\n")
		f.Flush()
	})
	h := o.Middleware(obs.MuxRoute(mux))(mux)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/stream", nil))

	if !rec.Flushed {
		t.Error("Flush did not reach the underlying writer")
	}
	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "/v1/stream", "200")); got != 1 {
		t.Errorf("status=200 counter = %v, want 1", got)
	}
}

// WebSocket libraries hijack the connection, directly or through
// http.ResponseController.
func TestMiddlewareKeepsHijacker(t *testing.T) {
	o := obs.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/ws", func(w http.ResponseWriter, _ *http.Request) {
		conn, rw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 418 I'm a teapot\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		_ = rw.Flush()
	})
	srv := httptest.NewServer(o.Middleware(obs.MuxRoute(mux))(mux))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/ws")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTeapot {
		t.Errorf("status = %d, want %d from the hijacked connection", resp.StatusCode, http.StatusTeapot)
	}
}

// A writer that supports neither flushing nor hijacking must report that instead
// of panicking.
func TestMiddlewareReportsUnsupported(t *testing.T) {
	o := obs.New()
	var hijackErr error
	h := o.Middleware(obs.StaticRoute("/x"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.(http.Flusher).Flush()
		_, _, hijackErr = w.(http.Hijacker).Hijack()
	}))

	h.ServeHTTP(plainWriter{httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/x", nil))

	if !errors.Is(hijackErr, http.ErrNotSupported) {
		t.Errorf("hijack error = %v, want http.ErrNotSupported", hijackErr)
	}
}

// plainWriter hides every optional interface of the recorder.
type plainWriter struct{ rec *httptest.ResponseRecorder }

func (p plainWriter) Header() http.Header         { return p.rec.Header() }
func (p plainWriter) Write(b []byte) (int, error) { return p.rec.Write(b) }
func (p plainWriter) WriteHeader(code int)        { p.rec.WriteHeader(code) }
