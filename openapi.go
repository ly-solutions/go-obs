package obs

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

// Route is one endpoint of a service: a method and a path template such as
// "/v1/users/{id}".
type Route struct {
	Method string
	Path   string
}

// IsInternalPath reports whether path is an operational endpoint that every
// service has (metrics, health probes, the OpenAPI document itself) and that says
// nothing about the service's API. Such paths are left out of route tables and
// OpenAPI documents by every router integration in this module.
//
// The list is /metrics, /healthz, /health, /health/live, /health/ready, /readyz,
// /livez and /openapi.json.
func IsInternalPath(path string) bool {
	switch path {
	case "/metrics", "/healthz", "/health", "/health/live", "/health/ready",
		"/readyz", "/livez", "/openapi.json":
		return true
	}
	return false
}

// RoutesFunc returns the route table of a service.
type RoutesFunc func() []Route

// The methods OpenAPI knows. Anything else is left out: an unknown method would
// make the whole document invalid.
var openAPIMethods = map[string]string{
	"GET": "get", "POST": "post", "PUT": "put", "PATCH": "patch",
	"DELETE": "delete", "HEAD": "head", "OPTIONS": "options", "TRACE": "trace",
}

// OpenAPI builds an OpenAPI 3.1 document from a route table, as a value ready for
// encoding/json.
//
// The document lists paths and methods only; every operation has a single
// "default" response. Request and response schemas are not there, because a router
// does not know them. What it does answer precisely, and without annotations that
// drift from the code, is the first question about any service: which endpoints it
// has. The table comes from the router itself, the same source the metrics are
// labeled from.
//
// Routes with a method OpenAPI does not define, an empty path or the RouteOther
// path are skipped. Paths are written as given; OpenAPI expects "{name}"
// parameters, which is the syntax of http.ServeMux, chi and gorilla/mux.
func OpenAPI(title, version string, routes []Route) map[string]any {
	paths := map[string]any{}

	for _, route := range routes {
		method, ok := openAPIMethods[strings.ToUpper(strings.TrimSpace(route.Method))]
		if !ok || route.Path == "" || route.Path == RouteOther {
			continue
		}

		item, ok := paths[route.Path].(map[string]any)
		if !ok {
			item = map[string]any{}
			paths[route.Path] = item
		}
		item[method] = map[string]any{
			"responses": map[string]any{
				"default": map[string]any{"description": "Response of the service."},
			},
		}
	}

	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   title,
			"version": version,
			"description": "Endpoints collected from the service's route table. Request and " +
				"response schemas are not included: the router does not know them.",
		},
		"paths": paths,
	}
}

// OpenAPIHandler serves the OpenAPI document of a route table as JSON, with
// Cache-Control: no-store.
//
// routes is called on every request rather than once at startup: some routes are
// registered while the service initializes, and a snapshot taken too early would be
// incomplete without any error. A nil routes serves a document without paths.
func OpenAPIHandler(title, version string, routes RoutesFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var list []Route
		if routes != nil {
			list = routes()
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Path != list[j].Path {
				return list[i].Path < list[j].Path
			}
			return list[i].Method < list[j].Method
		})

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(OpenAPI(title, version, list))
	})
}
