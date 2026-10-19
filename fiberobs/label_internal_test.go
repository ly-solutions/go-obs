package fiberobs

import (
	"net/http"
	"testing"
	"unsafe"
)

func TestMethodLabelBuckets(t *testing.T) {
	for raw, want := range map[string]string{
		http.MethodGet:     http.MethodGet,
		http.MethodHead:    http.MethodHead,
		http.MethodPost:    http.MethodPost,
		http.MethodPut:     http.MethodPut,
		http.MethodPatch:   http.MethodPatch,
		http.MethodDelete:  http.MethodDelete,
		http.MethodConnect: http.MethodConnect,
		http.MethodOptions: http.MethodOptions,
		http.MethodTrace:   http.MethodTrace,
		"get":              otherMethod, // methods are case-sensitive
		"PROPFIND":         otherMethod,
		"BREW":             otherMethod,
		"PO":               otherMethod,
		"":                 otherMethod,
	} {
		if got := methodLabel(raw); got != want {
			t.Errorf("methodLabel(%q) = %q, want %q", raw, got, want)
		}
	}
}

// The label must not share memory with its input, which may be a view into a
// buffer that gets reused for the next request.
func TestMethodLabelOwnsItsMemory(t *testing.T) {
	canonical := map[*byte]bool{unsafe.StringData(otherMethod): true}
	for _, label := range methodLabels {
		canonical[unsafe.StringData(label)] = true
	}

	for _, raw := range []string{http.MethodGet, http.MethodDelete, "MKCOL"} {
		buf := []byte(raw)
		view := unsafe.String(unsafe.SliceData(buf), len(buf))

		label := methodLabel(view)
		for i := range buf {
			buf[i] = 'X'
		}

		if !canonical[unsafe.StringData(label)] {
			t.Errorf("label for %q points into the caller's memory", raw)
		}
		if label != methodLabel(raw) {
			t.Errorf("label for %q changed to %q when the input buffer was overwritten", raw, label)
		}
	}
}
