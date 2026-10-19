package obs

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// RouteFunc returns the route template of a request: "/v1/users/{id}", not
// "/v1/users/17". A raw URL in a label explodes cardinality: every user would get
// a time series of their own, and so would every path a scanner tries.
//
// The middleware calls it after the handler has returned, so routers that record
// the matched route on the request while routing (chi, gorilla/mux) can be read
// at that point.
type RouteFunc func(*http.Request) string

// RouteOther is the route label of a request that matched no known route.
const RouteOther = "other"

// MuxRoute reads the route template from a standard http.ServeMux.
//
// The middleware wraps the mux from the outside, while http.Request.Pattern is
// set inside the mux's ServeHTTP on a copy of the request and is not visible
// outside. So the pattern is looked up from the mux itself.
func MuxRoute(mux *http.ServeMux) RouteFunc {
	return func(r *http.Request) string {
		_, pattern := mux.Handler(r)
		return normalizePattern(pattern)
	}
}

// StaticRoute labels every request with the same route. It suits services with a
// single endpoint, and tests.
func StaticRoute(route string) RouteFunc {
	return func(*http.Request) string { return route }
}

// normalizePattern drops the method from a Go 1.22 pattern: "GET /v1/users/{id}"
// becomes "/v1/users/{id}". The method has a label of its own, and with the prefix
// one endpoint would show up as two different routes.
func normalizePattern(pattern string) string {
	if pattern == "" {
		return RouteOther
	}
	if i := strings.LastIndex(pattern, " "); i >= 0 {
		pattern = pattern[i+1:]
	}
	if pattern == "" {
		return RouteOther
	}
	return pattern
}

// StatusClientClosedRequest is the status a request is counted under when the
// client went away before the response. It is not a standard HTTP status; 499 is
// the nginx convention, so the service's metrics agree with nginx-style access logs.
const StatusClientClosedRequest = 499

// Middleware records RED metrics (rate, errors, duration) of incoming requests,
// labeled with the route template that route returns. A nil route labels every
// request as RouteOther.
//
// A request the client abandoned is not a server failure. When the client closes
// the connection, the request context is canceled, calls to upstream services
// return context.Canceled, and the handler typically writes a 502 or 500 into a
// connection nobody reads anymore. Counted as is, a user double-clicking a button
// would page whoever alerts on 5xx. So a 5xx written after the request context was
// canceled is counted as 499 (StatusClientClosedRequest). Only the label changes;
// the handler's response is left alone. An expired deadline
// (context.DeadlineExceeded) is not affected: that is the server's own timeout,
// and the client was still waiting.
func (o *Obs) Middleware(route RouteFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			o.inFlight.Inc()
			defer o.inFlight.Dec()

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)

			name := RouteOther
			if route != nil {
				name = route(r)
			}
			o.ObserveRequest(r.Method, name, observedStatus(r, rec.status), time.Since(start))
		})
	}
}

// observedStatus returns the status the request is counted under.
//
// net/http cancels the request context in two cases: the client closed the
// connection (or reset the HTTP/2 stream), or ServeHTTP returned. The middleware
// asks before its own ServeHTTP returns, so a cancellation here can only mean the
// former.
func observedStatus(r *http.Request, status int) int {
	if status >= http.StatusInternalServerError && errors.Is(r.Context().Err(), context.Canceled) {
		return StatusClientClosedRequest
	}
	return status
}

// ObserveRequest records one incoming request. It is meant for middleware of
// routers that do not use the http.Handler chain (see the fiberobs package).
// An empty route is recorded as RouteOther.
func (o *Obs) ObserveRequest(method, route string, status int, elapsed time.Duration) {
	if route == "" {
		route = RouteOther
	}
	o.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	o.duration.WithLabelValues(method, route).Observe(elapsed.Seconds())
}

// InFlight returns the gauge of requests being served, for middleware of other
// routers.
func (o *Obs) InFlight() prometheus.Gauge { return o.inFlight }

// RequestsCounter returns the http_requests_total counter of one label set. It lets
// a service's tests check their own routes without parsing the text exposition.
func (o *Obs) RequestsCounter(method, route, status string) (prometheus.Counter, error) {
	return o.requests.GetMetricWithLabelValues(method, route, status)
}

// statusRecorder remembers the status code, which http.ResponseWriter does not
// expose.
//
// It keeps the optional interfaces handlers rely on: Flush for streaming and
// server-sent events, Hijack for WebSocket upgrades, and Unwrap for
// http.ResponseController. Each delegates to the wrapped writer and reports
// http.ErrNotSupported when that writer cannot do it.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.written {
		s.status = code
		s.written = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.written = true
	return s.ResponseWriter.Write(b)
}

// Flush implements http.Flusher. Flushing sends the headers, so the status is
// fixed from here on.
func (s *statusRecorder) Flush() {
	s.written = true
	_ = http.NewResponseController(s.ResponseWriter).Flush()
}

// Hijack implements http.Hijacker.
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(s.ResponseWriter).Hijack()
}

// Unwrap lets http.ResponseController reach the wrapped writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
