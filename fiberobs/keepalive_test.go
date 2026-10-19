package fiberobs_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	obs "github.com/ly-solutions/go-obs"
	"github.com/ly-solutions/go-obs/fiberobs"
)

// Several methods on one keep-alive connection, which is where fasthttp reuses
// request buffers. Afterwards the registry must gather cleanly and hold exactly one
// series per method that was sent.
func TestMethodLabelsSurviveConnectionReuse(t *testing.T) {
	o := obs.New()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(fiberobs.Middleware(o))
	app.All("/v1/items", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	defer func() { _ = app.Shutdown() }()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)

	sent := []string{"POST", "GET", "OPTIONS", "DELETE", "BREW", "GET", "OPTIONS", "PUT"}
	for _, method := range sent {
		if _, err := fmt.Fprintf(conn, "%s /v1/items HTTP/1.1\r\nHost: example.com\r\nContent-Length: 0\r\n\r\n", method); err != nil {
			t.Fatalf("write %s: %v", method, err)
		}
		resp, err := http.ReadResponse(reader, &http.Request{Method: method})
		if err != nil {
			t.Fatalf("read response to %s: %v", method, err)
		}
		_ = resp.Body.Close()
	}

	families, err := o.Registry().Gather()
	if err != nil {
		t.Fatalf("gather after connection reuse: %v", err)
	}
	var got []string
	for _, f := range families {
		if f.GetName() != "http_requests_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "method" {
					got = append(got, l.GetValue())
				}
			}
		}
	}
	sort.Strings(got)
	// Fiber answers BREW with 501 itself, before any handler runs, so it is not
	// counted; methods outside the standard set only reach the middleware when the
	// app is configured with custom RequestMethods.
	want := []string{"DELETE", "GET", "OPTIONS", "POST", "PUT"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("method labels = %v, want %v", got, want)
	}
}
