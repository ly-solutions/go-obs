package muxobs_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	obs "github.com/ly-solutions/go-obs"
	"github.com/ly-solutions/go-obs/muxobs"
)

func noop(http.ResponseWriter, *http.Request) {}

func TestRoutesFromRouter(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/v1/events/{id}", noop).Methods(http.MethodGet)
	r.HandleFunc("/v1/orders", noop).Methods(http.MethodPost, http.MethodGet)

	want := []obs.Route{
		{Method: "GET", Path: "/v1/events/{id}"},
		{Method: "GET", Path: "/v1/orders"},
		{Method: "POST", Path: "/v1/orders"},
	}
	got := muxobs.Routes(r)
	if len(got) != len(want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("routes = %v, want %v", got, want)
		}
	}
}

// A route without explicit methods accepts any request, and the router does not
// know which methods the handler really serves. It must not invent one.
func TestRouteWithoutMethodsIsSkipped(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/v1/anything", noop)

	if got := muxobs.Routes(r); len(got) != 0 {
		t.Errorf("Routes() = %v, want empty", got)
	}
	if got := muxobs.Undocumented(r); len(got) != 1 || got[0] != "/v1/anything" {
		t.Errorf("Undocumented() = %v, want [/v1/anything]", got)
	}
}

func TestRoutesSkipInternalPaths(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/healthz", noop).Methods(http.MethodGet)
	r.HandleFunc("/metrics", noop).Methods(http.MethodGet)
	r.HandleFunc("/v1/feed", noop).Methods(http.MethodGet)

	got := muxobs.Routes(r)
	if len(got) != 1 || got[0].Path != "/v1/feed" {
		t.Errorf("Routes() = %v, want only /v1/feed", got)
	}
}

func TestRoutesNilRouter(t *testing.T) {
	if got := muxobs.Routes(nil); got != nil {
		t.Errorf("Routes(nil) = %v, want nil", got)
	}
}

func TestMountOpenAPI(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/v1/events/{id}", noop).Methods(http.MethodGet)
	muxobs.MountOpenAPI(r, "gateway", "2.0.0")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var spec struct {
		Info struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if spec.Info.Title != "gateway" || spec.Info.Version != "2.0.0" {
		t.Errorf("info = %+v, want gateway/2.0.0", spec.Info)
	}
	if _, ok := spec.Paths["/v1/events/{id}"]["get"]; !ok {
		t.Errorf("document lacks GET /v1/events/{id}: %v", spec.Paths)
	}
	if _, ok := spec.Paths["/openapi.json"]; ok {
		t.Errorf("document describes itself: %v", spec.Paths)
	}
}

// Mounting the document must not break routes registered before it.
func TestMountOpenAPIKeepsRouting(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/v1/feed", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}).Methods(http.MethodGet)
	muxobs.MountOpenAPI(r, "gateway", "")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/feed", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}
