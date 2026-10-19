package obs

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Mount adds a handler for an exact path on the management server that Serve
// runs, next to /metrics.
//
// It is meant for things that belong on an internal port rather than the public
// one, such as the OpenAPI document of a gateway: its route list is a map of
// everything behind it.
//
// Mount may be called before or after Serve: the management server looks the path
// up on every request. Mounting the same path again replaces the handler; a nil
// handler is ignored.
func (o *Obs) Mount(path string, handler http.Handler) {
	if handler == nil {
		return
	}
	o.extraMu.Lock()
	defer o.extraMu.Unlock()
	if o.extra == nil {
		o.extra = map[string]http.Handler{}
	}
	o.extra[path] = handler
}

// Serve runs a separate management HTTP server on addr that exposes /metrics and
// the handlers added with Mount, and shuts it down gracefully when ctx is done.
//
// A separate port keeps per-route latency, upstream health and runtime details
// off the public listener and out of the reach of the internet. Serve blocks; it
// returns nil after a shutdown caused by ctx, and the listen error otherwise (for
// example, when the port is taken).
func (o *Obs) Serve(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", o.Handler())

	// Mounted handlers are resolved per request, not now: the management server is
	// usually started at the top of main, before the router whose document it
	// serves is built, and a snapshot taken here would quietly answer 404.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		o.extraMu.Lock()
		handler, ok := o.extra[r.URL.Path]
		o.extraMu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
