package fiberobs

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"

	obs "github.com/ly-solutions/go-obs"
)

// Routes returns the route table of a Fiber app, read from the app itself, so it
// cannot drift from the code.
//
// Two kinds of entries are left out: middleware, which Fiber also keeps as routes
// with the method "USE", and operational paths (see obs.IsInternalPath). Paths keep
// Fiber's syntax ("/v1/orders/:id"). A nil app returns nil.
func Routes(app *fiber.App) []obs.Route {
	if app == nil {
		return nil
	}

	seen := map[string]bool{}
	var routes []obs.Route
	for _, r := range app.GetRoutes(true) {
		if r.Method == "USE" || obs.IsInternalPath(r.Path) {
			continue
		}
		key := r.Method + " " + r.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		routes = append(routes, obs.Route{Method: r.Method, Path: r.Path})
	}
	return routes
}

// MountOpenAPI serves the OpenAPI document of the app's routes at
// "GET /openapi.json".
//
// The route table is read on every request, so routes registered after this call
// are included.
func MountOpenAPI(app *fiber.App, title, version string) {
	if app == nil {
		return
	}
	handler := obs.OpenAPIHandler(title, version, func() []obs.Route { return Routes(app) })
	app.Get("/openapi.json", adaptor.HTTPHandler(handler))
}
