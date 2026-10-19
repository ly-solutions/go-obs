package obs_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	obs "github.com/ly-solutions/go-obs"
)

func TestServeExposesMetricsAndStopsOnContext(t *testing.T) {
	addr := freeAddr(t)

	o := obs.New()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- o.Serve(ctx, addr) }()

	var body string
	for i := 0; i < 100; i++ {
		resp, err := http.Get("http://" + addr + "/metrics")
		if err == nil {
			raw, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(raw)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Fatalf("metrics body missing go_goroutines: %q", body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Serve did not stop on context cancel")
	}
}

func TestServeReturnsListenError(t *testing.T) {
	// A busy port must come back as an error rather than fail silently: a service
	// that mixed up its metrics port with its main one should fail loudly.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	o := obs.New()
	if err := o.Serve(context.Background(), listener.Addr().String()); err == nil {
		t.Error("Serve on a busy port returned nil, want an error")
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return addr
}
