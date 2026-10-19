package obs_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	obs "github.com/ly-solutions/go-obs"
)

func noop(http.ResponseWriter, *http.Request) {}

func TestMuxRecordsMethodFromPattern(t *testing.T) {
	m := obs.NewMux()
	m.HandleFunc("GET /v1/feed", noop)
	m.HandleFunc("POST /v1/orders/{id}", noop)

	want := []obs.Route{
		{Method: "GET", Path: "/v1/feed"},
		{Method: "POST", Path: "/v1/orders/{id}"},
	}
	assertRoutes(t, m.Routes(), want)
}

func TestMuxRecordsExplicitMethods(t *testing.T) {
	m := obs.NewMux()
	m.HandleFunc("/v1/events/", noop, http.MethodGet, http.MethodHead)

	want := []obs.Route{
		{Method: "GET", Path: "/v1/events/"},
		{Method: "HEAD", Path: "/v1/events/"},
	}
	assertRoutes(t, m.Routes(), want)
}

// The router does not know which methods the handler accepts, and a document with
// an invented method is worse than none. Such a route stays out of the document
// but is not lost: it is listed separately.
func TestMuxWithoutMethodIsUndocumented(t *testing.T) {
	m := obs.NewMux()
	m.HandleFunc("/v1/wallet", noop)

	if got := m.Routes(); len(got) != 0 {
		t.Errorf("Routes() = %v, want empty", got)
	}
	want := []string{"/v1/wallet"}
	if got := m.Undocumented(); len(got) != 1 || got[0] != want[0] {
		t.Errorf("Undocumented() = %v, want %v", got, want)
	}
}

// Every service has operational paths; they say nothing about its API.
func TestMuxSkipsInternalPaths(t *testing.T) {
	m := obs.NewMux()
	m.HandleFunc("GET /healthz", noop)
	m.HandleFunc("GET /readyz", noop)
	m.HandleFunc("GET /metrics", noop)
	m.HandleFunc("GET /v1/feed", noop)

	assertRoutes(t, m.Routes(), []obs.Route{{Method: "GET", Path: "/v1/feed"}})
	if got := m.Undocumented(); len(got) != 0 {
		t.Errorf("Undocumented() = %v, want empty: operational paths are not missing docs", got)
	}
}

func TestMuxStillRoutes(t *testing.T) {
	m := obs.NewMux()
	m.HandleFunc("GET /v1/feed", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/feed", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

func TestMuxMountOpenAPI(t *testing.T) {
	m := obs.NewMux()
	m.HandleFunc("GET /v1/feed", noop)
	m.MountOpenAPI("catalog", "1.2.3")

	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var spec struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if spec.Info.Title != "catalog" || spec.Info.Version != "1.2.3" {
		t.Errorf("info = %+v, want catalog/1.2.3", spec.Info)
	}
	if _, ok := spec.Paths["/v1/feed"]["get"]; !ok {
		t.Errorf("document lacks GET /v1/feed: %v", spec.Paths)
	}
	// The document endpoint itself is not in the document.
	if _, ok := spec.Paths["/openapi.json"]; ok {
		t.Errorf("document describes itself: %v", spec.Paths)
	}
}

// Routes are read per request: some are registered after the document is mounted,
// and a snapshot taken at mount time would be silently incomplete.
func TestMuxOpenAPISeesLateRoutes(t *testing.T) {
	m := obs.NewMux()
	m.MountOpenAPI("catalog", "")
	m.HandleFunc("GET /v1/late", noop)

	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if _, ok := spec.Paths["/v1/late"]["get"]; !ok {
		t.Errorf("late route missing from document: %v", spec.Paths)
	}
}

func TestMuxRouteFuncGivesPattern(t *testing.T) {
	m := obs.NewMux()
	m.HandleFunc("GET /v1/orders/{id}", noop)

	got := m.RouteFunc()(httptest.NewRequest(http.MethodGet, "/v1/orders/17", nil))
	if got != "/v1/orders/{id}" {
		t.Errorf("RouteFunc() = %q, want /v1/orders/{id}", got)
	}
}

func assertRoutes(t *testing.T, got, want []obs.Route) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("routes = %v, want %v", got, want)
		}
	}
}
