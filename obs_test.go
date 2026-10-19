package obs_test

import (
	"sort"
	"strings"
	"testing"

	obs "github.com/ly-solutions/go-obs"
)

func TestNewRegistersRuntimeCollectors(t *testing.T) {
	o := obs.New()

	families, err := o.Registry().Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	names := map[string]bool{}
	for _, f := range families {
		names[f.GetName()] = true
	}

	for _, want := range []string{"go_goroutines", "go_memstats_alloc_bytes", "process_cpu_seconds_total"} {
		if !names[want] {
			t.Errorf("collector %q missing; got %s", want, strings.Join(sortedKeys(names), ", "))
		}
	}
}

func TestRegistryIsPrivate(t *testing.T) {
	// Two independent expositions must not see each other's metrics; otherwise a
	// test that creates two would get duplicate registrations.
	first := obs.New()
	second := obs.New()

	if first.Registry() == second.Registry() {
		t.Fatal("New returned a shared registry")
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
