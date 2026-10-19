package fiberobs_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/ly-solutions/go-obs/fiberobs"
)

func has(t *testing.T, routes []struct{ Method, Path string }, method, path string) bool {
	t.Helper()
	for _, r := range routes {
		if r.Method == method && r.Path == path {
			return true
		}
	}
	return false
}

func TestRoutesReturnsTemplates(t *testing.T) {
	app := fiber.New()
	app.Get("/v1/events/:id", func(c *fiber.Ctx) error { return nil })
	app.Post("/v1/events", func(c *fiber.Ctx) error { return nil })

	routes := fiberobs.Routes(app)

	var flat []struct{ Method, Path string }
	for _, r := range routes {
		flat = append(flat, struct{ Method, Path string }{r.Method, r.Path})
	}

	if !has(t, flat, "GET", "/v1/events/:id") {
		t.Fatalf("parameterized template missing: %+v", flat)
	}
	if !has(t, flat, "POST", "/v1/events") {
		t.Fatalf("POST route missing: %+v", flat)
	}
}

// Middleware is not an endpoint; in the document it would look like a handler that does not exist.
func TestRoutesSkipsMiddleware(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { return c.Next() })
	app.Get("/v1/ping", func(c *fiber.Ctx) error { return nil })

	for _, r := range fiberobs.Routes(app) {
		if r.Method == "USE" {
			t.Fatalf("middleware in route table: %+v", r)
		}
	}
}

// Every service has operational paths; they say nothing about its API.
func TestRoutesSkipsInternalPaths(t *testing.T) {
	app := fiber.New()
	app.Get("/metrics", func(c *fiber.Ctx) error { return nil })
	app.Get("/healthz", func(c *fiber.Ctx) error { return nil })
	app.Get("/v1/ping", func(c *fiber.Ctx) error { return nil })

	for _, r := range fiberobs.Routes(app) {
		if r.Path == "/metrics" || r.Path == "/healthz" {
			t.Fatalf("operational path in document: %+v", r)
		}
	}
}

// The endpoint must serve a valid document and include routes registered after
// it: services mount it right after the middleware, before the handlers.
func TestMountOpenAPISeesLateRoutes(t *testing.T) {
	app := fiber.New()
	fiberobs.MountOpenAPI(app, "orders", "1.0.0")
	app.Get("/v1/late", func(c *fiber.Ctx) error { return nil })

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}

	var doc struct {
		OpenAPI string                    `json:"openapi"`
		Paths   map[string]map[string]any `json:"paths"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if doc.OpenAPI != "3.1.0" {
		t.Fatalf("openapi version %q", doc.OpenAPI)
	}
	if _, ok := doc.Paths["/v1/late"]; !ok {
		t.Fatalf("route registered after mounting is missing: %+v", doc.Paths)
	}
	if _, ok := doc.Paths["/openapi.json"]; ok {
		t.Fatal("document describes itself")
	}
}
