package fiberobs

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// otherMethod is the method label of every request whose method is not one of the
// standard ones.
const otherMethod = "other"

// methodLabels maps each standard method to the label used for it. Lookups return
// the map's own value, which is a constant of this package, never the caller's
// string.
var methodLabels = func() map[string]string {
	labels := map[string]string{}
	for _, m := range []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace,
	} {
		labels[m] = m
	}
	return labels
}()

// Method returns the method of the request as a label value that is safe to keep
// in a Prometheus registry: one of the nine standard methods, or "other".
//
// Without Fiber's Immutable setting, c.Method() is a view into a fasthttp buffer
// that is recycled for later requests. A registry keeps label values for the life
// of the process, so storing that view would let a series' labels change after
// the fact, until two series look identical and gathering fails. Method never
// returns memory it did not allocate itself. Mapping unknown methods to "other"
// also keeps clients from creating a series per invented method.
func Method(c *fiber.Ctx) string {
	return methodLabel(c.Method())
}

func methodLabel(raw string) string {
	if label, ok := methodLabels[raw]; ok {
		return label
	}
	return otherMethod
}
