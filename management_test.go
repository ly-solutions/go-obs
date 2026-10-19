package obs_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	obs "github.com/ly-solutions/go-obs"
)

// The OpenAPI document of an internet-facing service belongs on the management
// port, not the public one: a gateway's route list maps everything behind it.
func TestMountServesOnManagementPort(t *testing.T) {
	o := obs.New()
	o.Mount("/openapi.json", obs.OpenAPIHandler("gateway", "1.0.0", func() []obs.Route {
		return []obs.Route{{Method: "GET", Path: "/v1/events"}}
	}))

	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = o.Serve(ctx, addr) }()

	body := getWhenUp(t, "http://"+addr+"/openapi.json")

	var spec struct {
		Info  struct{ Title string } `json:"info"`
		Paths map[string]map[string]any
	}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if spec.Info.Title != "gateway" {
		t.Errorf("title = %q, want gateway", spec.Info.Title)
	}
	if _, ok := spec.Paths["/v1/events"]["get"]; !ok {
		t.Errorf("document lacks GET /v1/events: %v", spec.Paths)
	}
}

// Mount after Serve must work: the management server is started at the top of
// main, and the router whose document it serves is built further down.
func TestMountAfterServe(t *testing.T) {
	o := obs.New()
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = o.Serve(ctx, addr) }()

	if body := getWhenUp(t, "http://"+addr+"/metrics"); len(body) == 0 {
		t.Fatal("/metrics is empty")
	}

	o.Mount("/openapi.json", obs.OpenAPIHandler("late", "", func() []obs.Route {
		return []obs.Route{{Method: "GET", Path: "/v1/late"}}
	}))

	var spec struct {
		Paths map[string]map[string]any
	}
	if err := json.Unmarshal(getWhenUp(t, "http://"+addr+"/openapi.json"), &spec); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if _, ok := spec.Paths["/v1/late"]["get"]; !ok {
		t.Errorf("handler mounted after Serve is not served: %v", spec.Paths)
	}
}

// Serving metrics is this server's main job; mounted handlers must not displace it.
func TestMountKeepsMetrics(t *testing.T) {
	o := obs.New()
	o.Mount("/openapi.json", http.NotFoundHandler())

	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = o.Serve(ctx, addr) }()

	if body := getWhenUp(t, "http://"+addr+"/metrics"); len(body) == 0 {
		t.Error("/metrics is empty")
	}
}

func getWhenUp(t *testing.T, url string) []byte {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(20 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s returned %d", url, resp.StatusCode)
		}
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return raw
	}
	t.Fatalf("server did not come up: %v", lastErr)
	return nil
}
