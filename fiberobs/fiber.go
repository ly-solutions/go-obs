// Package fiberobs connects the metrics and route tables of the obs package to
// Fiber v2.
//
// It is a separate package so that services on plain net/http do not compile
// Fiber in.
package fiberobs

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	obs "github.com/ly-solutions/go-obs"
)

// Middleware records RED metrics of incoming Fiber requests, labeled with the
// route template ("/v1/orders/:id").
//
// Register it with app.Use before the routes: Fiber runs handlers in the order
// they were added, and routes added earlier bypass it.
func Middleware(o *obs.Obs) fiber.Handler {
	inFlight := o.InFlight()
	return func(c *fiber.Ctx) error {
		inFlight.Inc()
		start := time.Now()
		err := c.Next()
		inFlight.Dec()
		o.ObserveRequest(Method(c), routeTemplate(c), statusOf(c, err), time.Since(start))
		return err
	}
}

// statusOf returns the status the client will see.
//
// When c.Next returns, the response is not final yet: Fiber's error handler runs
// further up the stack, so for an unmatched route c.Response() still holds 200 and
// the real code is inside the *fiber.Error. Without this, a dashboard would show
// 2xx where the service answers 404 or 500.
func statusOf(c *fiber.Ctx, err error) int {
	if err == nil {
		return c.Response().StatusCode()
	}
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return fiberErr.Code
	}
	return fiber.StatusInternalServerError
}

// routeTemplate returns the route template.
//
// For a request that matched no route, Fiber reports the middleware's own route,
// the root "/", or a catch-all with a star. Both collapse into "other"; otherwise
// every path a scanner tries would show up as traffic on the root.
func routeTemplate(c *fiber.Ctx) string {
	route := c.Route()
	if route == nil || route.Path == "" || strings.Contains(route.Path, "*") {
		return obs.RouteOther
	}
	if route.Path == "/" && c.Path() != "/" {
		return obs.RouteOther
	}
	return route.Path
}
