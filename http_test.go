package obs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	obs "github.com/ly-solutions/go-obs"
)

func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/events/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /v1/boom", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("GET /v1/silent", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func TestMiddlewareCountsRouteTemplateNotURL(t *testing.T) {
	o := obs.New()
	mux := newTestMux()
	h := o.Middleware(obs.MuxRoute(mux))(mux)

	for _, path := range []string{"/v1/events/1", "/v1/events/2"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	const want = `
# HELP http_requests_total Incoming HTTP requests by method, route template and status code.
# TYPE http_requests_total counter
http_requests_total{method="GET",route="/v1/events/{id}",status="200"} 2
`
	if err := testutil.GatherAndCompare(o.Registry(), strings.NewReader(want), "http_requests_total"); err != nil {
		t.Error(err)
	}
}

func TestMiddlewareLabelsUnknownRouteAsOther(t *testing.T) {
	o := obs.New()
	mux := newTestMux()
	h := o.Middleware(obs.MuxRoute(mux))(mux)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nope/"+strings.Repeat("x", 40), nil))

	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "other", "404")); got != 1 {
		t.Errorf("route=other counter = %v, want 1", got)
	}
}

func TestMiddlewareRecordsStatusAndDuration(t *testing.T) {
	o := obs.New()
	mux := newTestMux()
	h := o.Middleware(obs.MuxRoute(mux))(mux)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/boom", nil))

	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "/v1/boom", "500")); got != 1 {
		t.Errorf("status=500 counter = %v, want 1", got)
	}
	if got := testutil.CollectAndCount(o.Registry(), "http_request_duration_seconds"); got == 0 {
		t.Error("duration histogram not collected")
	}
}

func TestMiddlewareDefaultsStatusTo200WhenHandlerOnlyWrites(t *testing.T) {
	// A handler that never calls WriteHeader answers 200; without accounting for
	// that, the metric would record 0 and skew the 2xx ratio.
	o := obs.New()
	mux := newTestMux()
	h := o.Middleware(obs.MuxRoute(mux))(mux)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/silent", nil))

	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "/v1/silent", "200")); got != 1 {
		t.Errorf("implicit 200 counter = %v, want 1", got)
	}
}

func TestMiddlewareCountsClientCanceledFailureAs499(t *testing.T) {
	// A real server and a real disconnect: the client drops the request while the
	// handler waits for an upstream. The handler then writes 502 into a closed
	// connection, which must not be counted as a 5xx.
	o := obs.New()
	mux := http.NewServeMux()
	started := make(chan struct{})
	done := make(chan struct{})
	mux.HandleFunc("GET /v1/amount", func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		close(started)
		<-r.Context().Done()
		w.WriteHeader(http.StatusBadGateway)
	})
	srv := httptest.NewServer(o.Middleware(obs.MuxRoute(mux))(mux))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/amount", nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		<-started
		cancel()
	}()
	if resp, err := http.DefaultClient.Do(req); err == nil {
		resp.Body.Close()
		t.Fatal("request was expected to be canceled by the client")
	}
	<-done

	// The counter is written after the handler returns, on the server's goroutine.
	counter := mustCounter(t, o, "GET", "/v1/amount", "499")
	deadline := time.Now().Add(2 * time.Second)
	for testutil.ToFloat64(counter) != 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := testutil.ToFloat64(counter); got != 1 {
		t.Errorf("status=499 counter = %v, want 1", got)
	}
	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "/v1/amount", "502")); got != 0 {
		t.Errorf("status=502 counter = %v, want 0: a request the client dropped is not our failure", got)
	}
}

func TestMiddlewareKeepsFailureWhenClientStillWaits(t *testing.T) {
	// An expired deadline is the server's own timeout: the client was waiting and
	// got a failure. A successful response to a departed client stays a success too.
	cases := []struct {
		name       string
		path       string
		ctx        func() (context.Context, context.CancelFunc)
		wantStatus string
	}{
		{"deadline exceeded stays 5xx", "/v1/boom", func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Unix(0, 0))
		}, "500"},
		{"canceled success stays 200", "/v1/events/1", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}, "200"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := obs.New()
			mux := newTestMux()
			h := o.Middleware(obs.MuxRoute(mux))(mux)

			ctx, cancel := tc.ctx()
			defer cancel()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil).WithContext(ctx)
			h.ServeHTTP(httptest.NewRecorder(), req)

			route := obs.MuxRoute(mux)(req)
			if got := testutil.ToFloat64(mustCounter(t, o, "GET", route, tc.wantStatus)); got != 1 {
				t.Errorf("status=%s counter = %v, want 1", tc.wantStatus, got)
			}
			if got := testutil.ToFloat64(mustCounter(t, o, "GET", route, "499")); got != 0 {
				t.Errorf("status=499 counter = %v, want 0", got)
			}
		})
	}
}

func TestMiddlewareTracksInFlight(t *testing.T) {
	o := obs.New()
	mux := http.NewServeMux()
	var during float64
	mux.HandleFunc("GET /v1/slow", func(w http.ResponseWriter, _ *http.Request) {
		during = testutil.ToFloat64(o.InFlight())
		w.WriteHeader(http.StatusOK)
	})
	h := o.Middleware(obs.MuxRoute(mux))(mux)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/slow", nil))

	if during != 1 {
		t.Errorf("in-flight during request = %v, want 1", during)
	}
	if after := testutil.ToFloat64(o.InFlight()); after != 0 {
		t.Errorf("in-flight after request = %v, want 0", after)
	}
}

func mustCounter(t *testing.T, o *obs.Obs, method, route, status string) prometheus.Counter {
	t.Helper()
	c, err := o.RequestsCounter(method, route, status)
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	return c
}
