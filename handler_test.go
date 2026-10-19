package obs_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	obs "github.com/ly-solutions/go-obs"
)

// failingCollector stands for any collector that cannot produce its metric, for
// example one whose source is unavailable.
type failingCollector struct{ desc *prometheus.Desc }

func newFailingCollector() failingCollector {
	return failingCollector{desc: prometheus.NewDesc("queue_depth", "Depth of a queue that cannot be read.", nil, nil)}
}

func (f failingCollector) Describe(ch chan<- *prometheus.Desc) { ch <- f.desc }

func (f failingCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.NewInvalidMetric(f.desc, errors.New("queue unavailable"))
}

func scrape(t *testing.T, h http.Handler) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return rec.Code, string(body)
}

func TestHandlerServesTheRestWhenACollectorFails(t *testing.T) {
	o := obs.New()
	o.Registry().MustRegister(newFailingCollector())

	code, body := scrape(t, o.Handler())

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite one failing collector", code)
	}
	for _, family := range []string{"go_goroutines", "process_cpu_seconds_total", "http_requests_in_flight"} {
		if !strings.Contains(body, family) {
			t.Errorf("exposition lacks %s:\n%s", family, body)
		}
	}
}

func TestHandlerCountsGatheringErrors(t *testing.T) {
	o := obs.New()
	o.Registry().MustRegister(newFailingCollector())

	h := o.Handler()

	// The counter is incremented after the first response has been gathered, so
	// the second scrape shows it.
	_, _ = scrape(t, h)
	_, body := scrape(t, h)

	const want = `promhttp_metric_handler_errors_total{cause="gathering"} 1`
	if !strings.Contains(body, want) {
		t.Errorf("exposition lacks %s:\n%s", want, body)
	}
}

// Handler may be called for every router it is mounted on; the error counter must
// be shared, not registered twice.
func TestHandlerCanBeCreatedRepeatedly(t *testing.T) {
	o := obs.New()
	first, second := o.Handler(), o.Handler()

	for _, h := range []http.Handler{first, second} {
		if code, _ := scrape(t, h); code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
	}
}
