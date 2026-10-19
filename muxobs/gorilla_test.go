package muxobs_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	obs "github.com/ly-solutions/go-obs"
	"github.com/ly-solutions/go-obs/muxobs"
)

func newRouter(o *obs.Obs) *mux.Router {
	r := mux.NewRouter()
	r.Use(mux.MiddlewareFunc(o.Middleware(muxobs.Route)))
	r.HandleFunc("/v1/tickets/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return r
}

func TestRouteReturnsPathTemplate(t *testing.T) {
	o := obs.New()
	r := newRouter(o)

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/tickets/7", nil))

	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "/v1/tickets/{id}", "200")); got != 1 {
		t.Errorf("counter = %v, want 1", got)
	}
}

func TestRouteWithoutMatchIsOther(t *testing.T) {
	// gorilla does not run middleware for unmatched requests, so Route is called
	// directly: it must cope with a request that matched nothing.
	if got := muxobs.Route(httptest.NewRequest(http.MethodGet, "/unmatched", nil)); got != obs.RouteOther {
		t.Errorf("Route() = %q, want %q", got, obs.RouteOther)
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
