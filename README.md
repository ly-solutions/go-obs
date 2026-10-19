<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/go-obs-wordmark-dark.svg">
    <img src="docs/brand/go-obs-wordmark-light.svg" alt="go-obs." width="300">
  </picture>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/ly-solutions/go-obs"><img src="https://pkg.go.dev/badge/github.com/ly-solutions/go-obs.svg" alt="Go Reference"></a>
  <a href="https://github.com/ly-solutions/go-obs/actions/workflows/ci.yml"><img src="https://github.com/ly-solutions/go-obs/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/ly-solutions/go-obs"><img src="https://goreportcard.com/badge/github.com/ly-solutions/go-obs" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="License: Apache-2.0"></a>
</p>

**go-obs** gives every Go HTTP service the same Prometheus metrics: RED metrics (rate, errors, duration) of
incoming requests per **route template**, RED metrics of outgoing requests per host, and the Go runtime and process
collectors, in a private registry. Same names in every service means one dashboard and one set of alert rules for
all of them.

It works with:

| Router | How the route template is found |
| --- | --- |
| `net/http` `ServeMux` (Go 1.22 patterns) | `obs.NewMux()` or `obs.MuxRoute(mux)` |
| [gorilla/mux](https://github.com/gorilla/mux) | `muxobs.Route` |
| [Fiber v2](https://github.com/gofiber/fiber) | `fiberobs.Middleware` |
| [chi](https://github.com/go-chi/chi) | a one-line `RouteFunc`, see the [FAQ](#faq) |
| anything else | your own `RouteFunc`, or `Obs.ObserveRequest` from your router's middleware |

It also turns the router's own route table into a minimal OpenAPI 3.1 document (paths and methods, no
annotations), and runs an optional management server for `/metrics` on a separate port.

## Install

```sh
go get github.com/ly-solutions/go-obs
```

Requires Go 1.25 or newer (the minimum of `prometheus/client_golang` v1.24).

## Quick start

```go
o := obs.New()

mux := obs.NewMux()
mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "user %s\n", r.PathValue("id"))
})
mux.Handle("GET /metrics", o.Handler())

log.Fatal(http.ListenAndServe(":8080", o.Middleware(mux.RouteFunc())(mux)))
```

After `curl localhost:8080/users/1` and `curl localhost:8080/users/2`, `/metrics` has one series for both:

```text
http_requests_total{method="GET",route="/users/{id}",status="200"} 2
```

This program is compiled as [`Example`](example_test.go) in the test suite; every package has runnable examples on
[pkg.go.dev](https://pkg.go.dev/github.com/ly-solutions/go-obs).

## Metrics

| Metric | Type | Labels |
| --- | --- | --- |
| `http_requests_total` | counter | `method`, `route`, `status` |
| `http_request_duration_seconds` | histogram | `method`, `route` |
| `http_requests_in_flight` | gauge | — |
| `http_client_requests_total` | counter | `host`, `status` |
| `http_client_request_duration_seconds` | histogram | `host` |
| `promhttp_metric_handler_errors_total` | counter | `cause` (`gathering`, `encoding`) |
| `go_*`, `process_*` | Go and process collectors | — |

- `route` is always the template (`/users/{id}`), never the raw path. A request that matched no route is
  `route="other"`, so scanners cannot create a series per probed URL.
- `status` is the code the handler wrote (200 if it only called `Write`). A 5xx written after the client went away
  is recorded as `499`, so a user closing a tab does not look like a server failure. Server-side timeouts
  (`context.DeadlineExceeded`) stay 5xx.
- `status` on the client side is the response code, or `error` when there was no response (refused, timed out,
  canceled).
- If a collector fails, `/metrics` still answers 200 with everything else and counts the failure in
  `promhttp_metric_handler_errors_total{cause="gathering"}`.
- Histogram buckets are `obs.DefaultBuckets`: 5 ms to 10 s.
- No `service`, `job` or `instance` label is added: your scraper attaches them, so the runtime series get them too.

## Usage

### gorilla/mux

```go
o := obs.New()

r := mux.NewRouter()
r.Use(o.Middleware(muxobs.Route))
r.HandleFunc("/v1/tickets/{id}", ticketHandler).Methods(http.MethodGet)
```

### Fiber v2

```go
o := obs.New()

app := fiber.New()
app.Use(fiberobs.Middleware(o)) // before the routes
app.Get("/v1/orders/:id", orderHandler)
```

### Outgoing requests

```go
client := &http.Client{
	Transport: o.Transport(nil), // nil means http.DefaultTransport
	Timeout:   2 * time.Second,
}
```

### A separate port for metrics

```go
go func() {
	if err := o.Serve(ctx, ":9090"); err != nil { // /metrics plus anything added with o.Mount
		log.Printf("management server: %v", err)
	}
}()
```

`Serve` shuts down gracefully when `ctx` is done. `o.Mount(path, handler)` adds more handlers to it, before or after
`Serve` starts.

### Your own metrics

```go
o.Registry().MustRegister(signupsTotal)
```

### OpenAPI from the route table

```go
mux.MountOpenAPI("orders", version)             // obs.Mux
muxobs.MountOpenAPI(router, "orders", version)  // gorilla/mux
fiberobs.MountOpenAPI(app, "orders", version)   // Fiber
o.Mount("/openapi.json", obs.OpenAPIHandler("orders", version, mux.Routes)) // on the management port instead
```

The document lists every route's path and methods, read from the router on each request, so it cannot drift from
the code. It has no request or response schemas: a router does not know them. Routes registered without a method
(`mux.HandleFunc("/v1/feed", h)`) are left out rather than guessed, and `Undocumented()` lists them so a test can keep
that list empty:

```go
if left := mux.Undocumented(); len(left) > 0 {
	t.Errorf("routes without a method: %v", left)
}
```

Operational paths (`/metrics`, `/healthz`, `/readyz`, `/livez`, `/health*`, `/openapi.json`) are skipped everywhere;
see `obs.IsInternalPath`.

## Compared with plain promhttp

- `promhttp.InstrumentHandler*` labels by handler, so you wrap and name each route yourself; go-obs labels every
  request with the route template the router matched, with one middleware.
- go-obs adds what you would otherwise write by hand: `other` for unmatched requests, `499` for abandoned requests,
  an outgoing-request transport, Fiber's buffer-safe method labels, and an exposition that survives (and counts)
  a failing collector.
- It uses a private registry with fixed metric names, so every service exposes the same series; plain `promhttp`
  leaves names, labels and registry up to each service.

## FAQ

**How do I use it with chi?** chi records the matched pattern while routing, and the middleware reads the route
after the handler returns, so a `RouteFunc` is enough:

```go
func chiRoute(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
		return rc.RoutePattern() // "/v1/users/{id}", nested routers included
	}
	return obs.RouteOther
}

r := chi.NewRouter()
r.Use(o.Middleware(chiRoute))
```

**Gin, Echo or another router?** Write a `RouteFunc` if the router exposes the matched template on the request,
or call `o.ObserveRequest(method, route, status, elapsed)` and `o.InFlight()` from its own middleware (Gin:
`c.FullPath()`, Echo: `c.Path()`), the way `fiberobs` does.

**Why a private registry instead of `prometheus.DefaultRegisterer`?** The exposition then holds exactly what the
service registers, not whatever a dependency registered globally. Add your own collectors with `o.Registry()`.

**Can I add a `service` label?** Let the scraper do it (for example, Kubernetes relabeling from pod labels).
Labels set by the application would be missing from `go_*` and `process_*`.

**Does the middleware break streaming or WebSockets?** No. The response writer keeps `http.Flusher` and
`http.Hijacker` and supports `http.ResponseController`. A hijacked connection is counted with the status written
before the hijack, 200 by default.

**Why is a request that the client abandoned counted as 499 and not 5xx?** When the client disconnects, the request
context is canceled, upstream calls fail with `context.Canceled`, and handlers commonly answer 502 into a closed
connection. Alerting on that would page you for users who closed a tab. Only the label changes; 499 matches nginx's
access logs. Fiber is the exception: fasthttp does not cancel on disconnect, so `fiberobs` cannot tell.

**Why does Fiber's method label say `other`?** Fiber's `c.Method()` points into a buffer that fasthttp reuses for
the next request; used as a label, it can mutate inside the registry and eventually break `/metrics`. `fiberobs`
maps methods to constants, and anything non-standard to `other`, which also caps cardinality.

**Are Fiber paths valid OpenAPI?** They keep Fiber's syntax (`/orders/:id`); `net/http`, chi and gorilla/mux paths
already use OpenAPI's `{id}` form.

## Contributing

Issues and pull requests are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md). Report security problems privately as
described in [SECURITY.md](SECURITY.md). This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md).

## License

[Apache License 2.0](LICENSE).
