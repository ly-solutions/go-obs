package obs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	obs "github.com/ly-solutions/go-obs"
)

// The quick start from the README: RED metrics per route template for a standard
// http.ServeMux, exposed on /metrics.
func Example() {
	o := obs.New()

	mux := obs.NewMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "user %s\n", r.PathValue("id"))
	})
	mux.Handle("GET /metrics", o.Handler())

	log.Fatal(http.ListenAndServe(":8080", o.Middleware(mux.RouteFunc())(mux)))
}

// Requests to /users/1 and /users/2 share one series, labeled with the route
// template. Requests that match no route are labeled "other".
func ExampleObs_Middleware() {
	o := obs.New()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := o.Middleware(obs.MuxRoute(mux))(mux)

	for _, path := range []string{"/users/1", "/users/2", "/nope"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	users, _ := o.RequestsCounter("GET", "/users/{id}", "200")
	other, _ := o.RequestsCounter("GET", obs.RouteOther, "404")
	fmt.Println("GET /users/{id} 200:", testutil.ToFloat64(users))
	fmt.Println("GET other 404:", testutil.ToFloat64(other))
	// Output:
	// GET /users/{id} 200: 2
	// GET other 404: 1
}

// Outgoing requests are counted per upstream host.
func ExampleObs_Transport() {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	o := obs.New()
	client := &http.Client{Transport: o.Transport(nil), Timeout: 2 * time.Second}

	resp, err := client.Get(upstream.URL + "/ping")
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()

	fmt.Println("series:", testutil.CollectAndCount(o.Registry(), "http_client_requests_total"))
	// Output:
	// series: 1
}

// Serve runs a management server for /metrics on its own port, away from the
// public listener, and stops it when the context is done. Mount adds more
// handlers to it, before or after Serve starts.
func ExampleObs_Serve() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	o := obs.New()
	public := obs.NewMux()
	public.HandleFunc("GET /v1/orders/{id}", func(w http.ResponseWriter, _ *http.Request) {})

	o.Mount("/openapi.json", obs.OpenAPIHandler("orders", "1.0.0", public.Routes))

	go func() {
		if err := o.Serve(ctx, ":9090"); err != nil {
			log.Printf("management server: %v", err)
		}
	}()

	srv := &http.Server{Addr: ":8080", Handler: o.Middleware(public.RouteFunc())(public)}
	go func() { _ = srv.ListenAndServe() }()

	<-ctx.Done()
	_ = srv.Shutdown(context.Background())
}

// Business metrics go into the same registry, so one endpoint exposes everything.
func ExampleObs_Registry() {
	o := obs.New()

	signups := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "signups_total",
		Help: "Completed sign-ups.",
	})
	o.Registry().MustRegister(signups)
	signups.Inc()

	fmt.Println(testutil.ToFloat64(signups))
	// Output:
	// 1
}

// Mux records its routes as they are registered. Routes without a method stay out
// of the table and are listed by Undocumented.
func ExampleMux() {
	mux := obs.NewMux()
	noop := func(http.ResponseWriter, *http.Request) {}

	mux.HandleFunc("GET /v1/orders/{id}", noop)
	mux.HandleFunc("/v1/search", noop, http.MethodGet, http.MethodPost)
	mux.HandleFunc("/v1/legacy", noop)
	mux.HandleFunc("GET /healthz", noop) // operational paths are skipped

	for _, r := range mux.Routes() {
		fmt.Println(r.Method, r.Path)
	}
	fmt.Println("undocumented:", mux.Undocumented())
	// Output:
	// GET /v1/orders/{id}
	// GET /v1/search
	// POST /v1/search
	// undocumented: [/v1/legacy]
}

// The OpenAPI document lists the routes the router knows, with no annotations.
func ExampleMux_MountOpenAPI() {
	mux := obs.NewMux()
	mux.HandleFunc("GET /v1/orders/{id}", func(http.ResponseWriter, *http.Request) {})
	mux.MountOpenAPI("orders", "1.0.0")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	var doc struct {
		OpenAPI string                    `json:"openapi"`
		Paths   map[string]map[string]any `json:"paths"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&doc); err != nil {
		fmt.Println(err)
		return
	}
	for path, ops := range doc.Paths {
		for method := range ops {
			fmt.Println(doc.OpenAPI, method, path)
		}
	}
	// Output:
	// 3.1.0 get /v1/orders/{id}
}
