package obs_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	obs "github.com/ly-solutions/go-obs"
)

func TestTransportCountsByHostAndStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer upstream.Close()

	o := obs.New()
	client := &http.Client{Transport: o.Transport(http.DefaultTransport)}

	resp, err := client.Get(upstream.URL + "/anything")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if got := testutil.CollectAndCount(o.Registry(), "http_client_requests_total"); got != 1 {
		t.Errorf("client series = %d, want 1", got)
	}
	if got := testutil.CollectAndCount(o.Registry(), "http_client_request_duration_seconds"); got != 1 {
		t.Errorf("client duration series = %d, want 1", got)
	}
}

func TestTransportRecordsTransportError(t *testing.T) {
	// A refused connection is the most interesting part of an upstream's picture:
	// without a label of its own it would look like no calls at all.
	o := obs.New()
	client := &http.Client{Transport: o.Transport(nil)}

	//nolint:bodyclose // the request never reaches a server, there is no body
	if _, err := client.Get("http://127.0.0.1:1/unreachable"); err == nil {
		t.Fatal("expected a transport error")
	}

	if got := testutil.CollectAndCount(o.Registry(), "http_client_requests_total"); got != 1 {
		t.Errorf("client series = %d, want 1", got)
	}
}

func TestTransportDefaultsToDefaultTransport(t *testing.T) {
	o := obs.New()
	if o.Transport(nil) == nil {
		t.Fatal("Transport(nil) returned nil")
	}
}
