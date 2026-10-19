// Package obs gives Go HTTP services the same Prometheus metrics everywhere: RED
// metrics (rate, errors, duration) of incoming requests labeled by route
// template, RED metrics of outgoing requests labeled by host, and the Go runtime
// and process collectors, all in a private registry.
//
// Identical metric names across services are the point: one dashboard and one set
// of alert rules then work for every service, switched by a label the scraper adds.
//
// # Incoming requests
//
// Obs.Middleware wraps any http.Handler. The route label comes from a RouteFunc,
// which must return the route template ("/v1/users/{id}"), never the raw path:
//
//   - MuxRoute and Mux.RouteFunc read it from a standard http.ServeMux (Go 1.22
//     patterns);
//   - the muxobs package reads it from gorilla/mux;
//   - the fiberobs package provides a middleware for Fiber v2;
//   - for chi, a one-line RouteFunc returns chi.RouteContext(r.Context()).RoutePattern().
//
// Requests that match no route are labeled "other" (RouteOther). A 5xx written
// after the client went away is counted as 499 (StatusClientClosedRequest), so
// abandoned requests do not look like server failures.
//
// # Outgoing requests
//
// Obs.Transport wraps an http.RoundTripper and labels requests by host; requests
// that got no response at all are counted with status="error".
//
// # Exposition
//
// Obs.Handler serves the registry on any router; Obs.Serve runs a separate
// management server for /metrics and anything added with Obs.Mount. The registry is
// private rather than prometheus.DefaultRegisterer, so the exposition holds exactly
// what the service registers, and Obs.Registry accepts the service's own metrics.
//
// # Route tables and OpenAPI
//
// The same route tables that label metrics can be served as a minimal OpenAPI 3.1
// document (paths and methods only): Mux.MountOpenAPI for net/http, and
// MountOpenAPI in muxobs and fiberobs.
package obs
