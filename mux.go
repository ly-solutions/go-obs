package obs

import (
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Mux is a standard http.ServeMux that remembers what was registered on it.
//
// Fiber and gorilla/mux can list their routes, so an OpenAPI document built from
// them cannot drift from the code. A plain http.ServeMux cannot be asked for its
// routes at all; the only moment that table exists is registration, and Mux records
// it there. The pattern string is the very one used for routing, so there is no
// second copy to get wrong.
//
// Mux does not guess methods. A pattern without a method ("/v1/feed") accepts any
// request, and only the handler knows which methods it really serves. Writing
// "get" into the document because that is the common case would present a guess as
// a contract. Such routes stay out of the document and are listed by Undocumented
// instead.
//
// Host patterns ("example.com/path") are recorded as paths as they are.
type Mux struct {
	*http.ServeMux

	mu           sync.Mutex
	routes       []Route
	undocumented []string
}

// NewMux creates a Mux around a new http.ServeMux.
func NewMux() *Mux {
	return &Mux{ServeMux: http.NewServeMux()}
}

// Handle registers a handler and records it in the route table.
//
// Methods come from a Go 1.22 pattern ("GET /v1/feed") or, for patterns that
// deliberately accept several methods and dispatch on them inside the handler,
// from the methods arguments. A method in the pattern wins over the arguments.
func (m *Mux) Handle(pattern string, handler http.Handler, methods ...string) {
	m.ServeMux.Handle(pattern, handler)
	m.record(pattern, methods)
}

// HandleFunc is Handle for a handler function.
func (m *Mux) HandleFunc(pattern string, handler http.HandlerFunc, methods ...string) {
	m.ServeMux.HandleFunc(pattern, handler)
	m.record(pattern, methods)
}

func (m *Mux) record(pattern string, methods []string) {
	method, path := splitPattern(pattern)
	if path == "" || IsInternalPath(path) {
		return
	}

	if method != "" {
		methods = []string{method}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(methods) == 0 {
		m.undocumented = append(m.undocumented, path)
		return
	}
	for _, verb := range methods {
		m.routes = append(m.routes, Route{Method: strings.ToUpper(strings.TrimSpace(verb)), Path: path})
	}
}

// Routes returns the recorded route table, in registration order.
func (m *Mux) Routes() []Route {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Route(nil), m.routes...)
}

// Undocumented lists, sorted, the paths registered without a method, which
// therefore stay out of the OpenAPI document.
//
// It exists so that an endpoint does not drop out of the document silently: a
// service can assert in a test that this list is empty.
func (m *Mux) Undocumented() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := append([]string(nil), m.undocumented...)
	sort.Strings(out)
	return out
}

// MountOpenAPI serves the OpenAPI document of the route table at
// "GET /openapi.json".
//
// The table is read on every request, not now: routes registered after this call
// are included, so the order of calls does not matter.
func (m *Mux) MountOpenAPI(title, version string) {
	m.ServeMux.Handle("GET /openapi.json", OpenAPIHandler(title, version, m.Routes))
}

// RouteFunc returns the route template for metrics. It is MuxRoute for the
// embedded http.ServeMux.
func (m *Mux) RouteFunc() RouteFunc {
	return MuxRoute(m.ServeMux)
}

// splitPattern splits a Go 1.22 pattern: "GET /v1/feed" into a method and a path,
// "/v1/feed" into a path only.
func splitPattern(pattern string) (method, path string) {
	pattern = strings.TrimSpace(pattern)
	if i := strings.Index(pattern, " "); i >= 0 {
		method = strings.ToUpper(strings.TrimSpace(pattern[:i]))
		path = strings.TrimSpace(pattern[i+1:])
		return method, path
	}
	return "", pattern
}
