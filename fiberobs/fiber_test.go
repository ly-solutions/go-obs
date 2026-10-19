package fiberobs_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	obs "github.com/ly-solutions/go-obs"
	"github.com/ly-solutions/go-obs/fiberobs"
)

func newApp(o *obs.Obs) *fiber.App {
	app := fiber.New()
	app.Use(fiberobs.Middleware(o))
	app.Get("/v1/orders/:id", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Get("/v1/boom", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusInternalServerError) })
	return app
}

func TestMiddlewareUsesRouteTemplate(t *testing.T) {
	o := obs.New()
	app := newApp(o)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/orders/42", nil))
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "/v1/orders/:id", "200")); got != 1 {
		t.Errorf("counter = %v, want 1", got)
	}
}

func TestMiddlewareLabelsUnmatchedAsOther(t *testing.T) {
	o := obs.New()
	app := newApp(o)

	serve(t, app, "/nothing-here")

	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "other", "404")); got != 1 {
		t.Errorf("route=other counter = %v, want 1", got)
	}
}

func TestMiddlewareRecordsErrorStatus(t *testing.T) {
	o := obs.New()
	app := newApp(o)

	serve(t, app, "/v1/boom")

	if got := testutil.ToFloat64(mustCounter(t, o, "GET", "/v1/boom", "500")); got != 1 {
		t.Errorf("status=500 counter = %v, want 1", got)
	}
}

func TestMiddlewareReleasesInFlight(t *testing.T) {
	o := obs.New()
	app := newApp(o)

	serve(t, app, "/v1/orders/1")

	if got := testutil.ToFloat64(o.InFlight()); got != 0 {
		t.Errorf("in-flight after request = %v, want 0", got)
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

func serve(t *testing.T, app *fiber.App, path string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	_ = resp.Body.Close()
}
