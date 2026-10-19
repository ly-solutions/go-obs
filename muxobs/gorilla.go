// Package muxobs connects the metrics and route tables of the obs package to
// gorilla/mux.
//
// It is a separate package so that services that do not use gorilla/mux do not
// compile it in.
package muxobs

import (
	"net/http"

	"github.com/gorilla/mux"

	obs "github.com/ly-solutions/go-obs"
)

// Route is an obs.RouteFunc that returns the gorilla/mux path template of the
// matched route ("/v1/tickets/{id}"), or obs.RouteOther when there is none.
//
// Use it with obs.Obs.Middleware registered through Router.Use: gorilla runs such
// middleware after matching, so the current route is known.
func Route(r *http.Request) string {
	current := mux.CurrentRoute(r)
	if current == nil {
		return obs.RouteOther
	}
	template, err := current.GetPathTemplate()
	if err != nil || template == "" {
		return obs.RouteOther
	}
	return template
}
