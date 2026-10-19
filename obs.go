package obs

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultBuckets are the latency buckets of both request duration histograms, in
// seconds. They reach higher than prometheus.DefBuckets so that slow endpoints show
// up as a tail between 2.5 s and 10 s instead of disappearing into +Inf.
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// Obs owns a private Prometheus registry and the HTTP metrics registered in it.
// Create one per process with New and share it between the server middleware, the
// client transport and the metrics endpoint. All methods are safe for concurrent use.
type Obs struct {
	registry *prometheus.Registry

	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge

	clientRequests *prometheus.CounterVec
	clientDuration *prometheus.HistogramVec

	// Options of every exposition handler; see Handler.
	exposition promhttp.HandlerOpts

	// Extra handlers of the management server; see Mount.
	extraMu sync.Mutex
	extra   map[string]http.Handler
}

// New creates a registry with the Go runtime and process collectors and the HTTP
// server and client metrics.
//
// New adds no service, job or instance labels on purpose: the scraper attaches
// them (for example from Kubernetes pod labels). If the application set them
// itself, the go_* and process_* series would lack them and dashboards could not
// tell one service's runtime from another's.
func New() *Obs {
	o := &Obs{registry: prometheus.NewRegistry()}

	o.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	o.requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Incoming HTTP requests by method, route template and status code.",
	}, []string{"method", "route", "status"})

	o.duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Incoming HTTP request latency by method and route template.",
		Buckets: DefaultBuckets,
	}, []string{"method", "route"})

	o.inFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Incoming HTTP requests currently being served.",
	})

	o.clientRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_requests_total",
		Help: "Outgoing HTTP requests by host and status code.",
	}, []string{"host", "status"})

	o.clientDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_request_duration_seconds",
		Help:    "Outgoing HTTP request latency by host.",
		Buckets: DefaultBuckets,
	}, []string{"host"})

	o.registry.MustRegister(o.requests, o.duration, o.inFlight, o.clientRequests, o.clientDuration)

	// Serve what can be gathered and count what could not. Registry makes promhttp
	// register its error counter here; repeated Handler calls reuse that counter.
	o.exposition = promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
		Registry:      o.registry,
	}
	return o
}

// Registry returns the private registry, for example to register the service's own
// business metrics so that a single endpoint exposes everything.
func (o *Obs) Registry() *prometheus.Registry { return o.registry }

// Handler returns the Prometheus exposition of the registry, ready to be mounted on
// any router.
//
// A collector that fails while being gathered does not take the endpoint down:
// the response is still a 200 with every metric that could be collected, and the
// failure is counted in promhttp_metric_handler_errors_total{cause="gathering"} in
// the same registry. With promhttp's default, a single bad collector would turn
// the whole response into a 500, hiding the runtime and HTTP metrics while the
// target still looks healthy to the scraper.
func (o *Obs) Handler() http.Handler {
	return promhttp.HandlerFor(o.registry, o.exposition)
}
