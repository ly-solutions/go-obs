package muxobs

import (
	"net/http"
	"sort"

	"github.com/gorilla/mux"

	obs "github.com/ly-solutions/go-obs"
)

// Routes returns the route table of a router, sorted by path and method. gorilla
// can walk its own tree, so the table cannot drift from the code.
//
// A route without explicit methods is left out: it accepts any method, and only
// its handler knows which ones it serves. Writing "get" for it would present a
// guess as a contract. Such routes are listed by Undocumented. Routes without a
// path template (matchers only) and operational paths (see obs.IsInternalPath) are
// skipped. A nil router returns nil.
func Routes(router *mux.Router) []obs.Route {
	if router == nil {
		return nil
	}

	seen := map[string]bool{}
	var routes []obs.Route
	walk(router, func(path string, methods []string) {
		for _, method := range methods {
			key := method + " " + path
			if seen[key] {
				continue
			}
			seen[key] = true
			routes = append(routes, obs.Route{Method: method, Path: path})
		}
	})
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
	return routes
}

// Undocumented lists, sorted, the paths of routes without explicit methods, which
// therefore stay out of Routes and the OpenAPI document. Like obs.Mux.Undocumented,
// it lets a test make sure no endpoint drops out of the document silently.
func Undocumented(router *mux.Router) []string {
	if router == nil {
		return nil
	}

	seen := map[string]bool{}
	var paths []string
	walk(router, func(path string, methods []string) {
		if len(methods) > 0 || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	})
	sort.Strings(paths)
	return paths
}

// MountOpenAPI serves the OpenAPI document of the router's routes at
// "GET /openapi.json".
//
// The route table is read on every request, so routes registered after this call
// are included.
func MountOpenAPI(router *mux.Router, title, version string) {
	if router == nil {
		return
	}
	handler := obs.OpenAPIHandler(title, version, func() []obs.Route { return Routes(router) })
	router.Handle("/openapi.json", handler).Methods(http.MethodGet)
}

// walk visits the path template and methods of every route in the router.
//
// Errors from GetPathTemplate and GetMethods are not failures but "this route has
// none": a route defined by matchers or a regexp has no path template, and a route
// without .Methods() has no method list.
func walk(router *mux.Router, visit func(path string, methods []string)) {
	_ = router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		path, err := route.GetPathTemplate()
		if err != nil || path == "" || obs.IsInternalPath(path) {
			return nil
		}
		methods, err := route.GetMethods()
		if err != nil {
			methods = nil
		}
		visit(path, methods)
		return nil
	})
}
