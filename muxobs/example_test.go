package muxobs_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/testutil"

	obs "github.com/ly-solutions/go-obs"
	"github.com/ly-solutions/go-obs/muxobs"
)

// Register the obs middleware with Router.Use and label requests with Route.
func ExampleRoute() {
	o := obs.New()

	r := mux.NewRouter()
	r.Use(o.Middleware(muxobs.Route))
	r.HandleFunc("/v1/tickets/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/tickets/7", nil))

	c, _ := o.RequestsCounter("GET", "/v1/tickets/{id}", "200")
	fmt.Println(testutil.ToFloat64(c))
	// Output:
	// 1
}

// Routes walks the router; routes without methods are reported by Undocumented.
func ExampleRoutes() {
	r := mux.NewRouter()
	noop := func(http.ResponseWriter, *http.Request) {}
	r.HandleFunc("/v1/tickets/{id}", noop).Methods(http.MethodGet, http.MethodDelete)
	r.HandleFunc("/v1/legacy", noop)

	for _, route := range muxobs.Routes(r) {
		fmt.Println(route.Method, route.Path)
	}
	fmt.Println("undocumented:", muxobs.Undocumented(r))
	// Output:
	// DELETE /v1/tickets/{id}
	// GET /v1/tickets/{id}
	// undocumented: [/v1/legacy]
}
