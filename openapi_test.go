package obs_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	obs "github.com/ly-solutions/go-obs"
)

func paths(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	p, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("document has no paths: %+v", doc)
	}
	return p
}

func TestOpenAPICollectsPathsAndMethods(t *testing.T) {
	doc := obs.OpenAPI("catalog", "1.0.0", []obs.Route{
		{Method: "GET", Path: "/v1/feed"},
		{Method: "POST", Path: "/v1/feed"},
	})

	feed, ok := paths(t, doc)["/v1/feed"].(map[string]any)
	if !ok {
		t.Fatalf("path missing from document: %+v", doc["paths"])
	}
	if _, ok := feed["get"]; !ok {
		t.Fatal("method get lost")
	}
	if _, ok := feed["post"]; !ok {
		t.Fatal("method post lost")
	}
}

// An unknown method would make the whole document invalid, so it is dropped.
func TestOpenAPIDropsUnknownMethod(t *testing.T) {
	doc := obs.OpenAPI("x", "1", []obs.Route{{Method: "USE", Path: "/v1/feed"}})
	if len(paths(t, doc)) != 0 {
		t.Fatalf("unknown method in document: %+v", doc["paths"])
	}
}

// "other" labels requests that matched no route; it is not a path.
func TestOpenAPISkipsRouteOther(t *testing.T) {
	doc := obs.OpenAPI("x", "1", []obs.Route{{Method: "GET", Path: obs.RouteOther}})
	if len(paths(t, doc)) != 0 {
		t.Fatalf("label other in document: %+v", doc["paths"])
	}
}

func TestOpenAPIHandlerServesValidDocument(t *testing.T) {
	handler := obs.OpenAPIHandler("catalog", "1.2.3", func() []obs.Route {
		return []obs.Route{{Method: "GET", Path: "/v1/feed"}}
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi version: %v", doc["openapi"])
	}
	info, _ := doc["info"].(map[string]any)
	if info["version"] != "1.2.3" {
		t.Fatalf("service version missing: %+v", info)
	}
}

// The table is read on every request: routes may appear after startup.
func TestOpenAPIHandlerReadsTableEveryTime(t *testing.T) {
	calls := 0
	handler := obs.OpenAPIHandler("x", "1", func() []obs.Route {
		calls++
		return nil
	})
	for i := 0; i < 3; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	}
	if calls != 3 {
		t.Fatalf("table read %d times, want 3", calls)
	}
}

func TestOpenAPIHandlerWithoutTable(t *testing.T) {
	rec := httptest.NewRecorder()
	obs.OpenAPIHandler("x", "1", nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}
