package fiberobs_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"

	obs "github.com/ly-solutions/go-obs"
	"github.com/ly-solutions/go-obs/fiberobs"
)

// Register the middleware before the routes; requests are labeled with Fiber's
// route template.
func ExampleMiddleware() {
	o := obs.New()

	app := fiber.New()
	app.Use(fiberobs.Middleware(o))
	app.Get("/v1/orders/:id", func(c *fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/orders/42", nil))
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()

	c, _ := o.RequestsCounter("GET", "/v1/orders/:id", "200")
	fmt.Println(testutil.ToFloat64(c))
	// Output:
	// 1
}

// MountOpenAPI serves the app's routes at /openapi.json; Routes returns them.
// Fiber's app.Get registers HEAD as well.
func ExampleRoutes() {
	app := fiber.New()
	app.Get("/v1/orders/:id", func(c *fiber.Ctx) error { return nil })
	app.Post("/v1/orders", func(c *fiber.Ctx) error { return nil })
	fiberobs.MountOpenAPI(app, "orders", "1.0.0")

	for _, r := range fiberobs.Routes(app) {
		fmt.Println(r.Method, r.Path)
	}
	// Output:
	// GET /v1/orders/:id
	// HEAD /v1/orders/:id
	// POST /v1/orders
}
