# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-10-19

First public release.

### Added

- `obs.New`: a private Prometheus registry with the Go and process collectors and the HTTP metrics
  `http_requests_total`, `http_request_duration_seconds`, `http_requests_in_flight`,
  `http_client_requests_total` and `http_client_request_duration_seconds`, plus
  `promhttp_metric_handler_errors_total`.
- `Obs.Middleware` for `net/http` with route-template labels from a `RouteFunc`: `MuxRoute` for `http.ServeMux`
  (Go 1.22 patterns), `StaticRoute`, and `Mux`, a `ServeMux` that records its route table.
- Unmatched requests are labeled `other`; a 5xx written after the client disconnected is recorded as `499`.
- The response writer wrapper keeps `http.Flusher` and `http.Hijacker` and supports `http.ResponseController`.
- `Obs.Transport` for outgoing requests, labeled by host, with `status="error"` when there is no response.
- `Obs.Handler`, which keeps serving when a collector fails and counts the failure in
  `promhttp_metric_handler_errors_total`, and `Obs.Serve` with `Obs.Mount` for a separate
  management port.
- `OpenAPI`, `OpenAPIHandler` and `Mux.MountOpenAPI`: a minimal OpenAPI 3.1 document built from the route table.
- `muxobs`: route templates, route tables and OpenAPI for gorilla/mux.
- `fiberobs`: middleware, buffer-safe method labels, route tables and OpenAPI for Fiber v2.

[Unreleased]: https://github.com/ly-solutions/go-obs/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ly-solutions/go-obs/releases/tag/v0.1.0
