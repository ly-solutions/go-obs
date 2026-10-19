package obs

import (
	"net/http"
	"strconv"
	"time"
)

// Transport wraps an http.RoundTripper and records RED metrics of outgoing
// requests. A nil base means http.DefaultTransport.
//
// The label is the host (with port, if the URL has one), not the URL: an upstream
// API has many paths, and the question a dashboard answers is whether that
// neighbor is up and how fast it answers. A request that got no response at all
// (connection refused, timeout, canceled) is counted with status="error", so a
// broken link to an upstream is not mistaken for the absence of calls.
func (o *Obs) Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		start := time.Now()
		resp, err := base.RoundTrip(r)
		elapsed := time.Since(start).Seconds()

		status := "error"
		if err == nil {
			status = strconv.Itoa(resp.StatusCode)
		}

		o.clientRequests.WithLabelValues(r.URL.Host, status).Inc()
		o.clientDuration.WithLabelValues(r.URL.Host).Observe(elapsed)
		return resp, err
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
