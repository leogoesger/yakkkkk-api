// Package web is a small router: global middleware set once on the App, plus
// optional per-route middleware on each Handle call.
package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Middleware runs code before and/or after another Handler.
type Middleware func(http.Handler) http.Handler

// App routes requests and applies the application's middleware to every route.
type App struct {
	mux *chi.Mux
	mw  []Middleware
}

// NewApp constructs an App. Middleware provided runs for every route.
func NewApp(mw ...Middleware) *App {
	return &App{mux: chi.NewRouter(), mw: mw}
}

// Use adds middleware that runs before routing, for every request including
// unmatched paths and methods (e.g. CORS preflight). Call it before adding routes.
func (a *App) Use(mw ...Middleware) {
	for _, m := range mw {
		a.mux.Use(m)
	}
}

// Handle associates a handler with an HTTP method and URL pattern.
// Route middleware runs after the application middleware, in the order given.
func (a *App) Handle(method, url string, h http.Handler, mw ...Middleware) {
	a.mux.Method(method, url, a.wrap(h, mw))
}

// HandleAll associates a handler with all HTTP methods for a URL pattern.
func (a *App) HandleAll(url string, h http.Handler, mw ...Middleware) {
	a.mux.Handle(url, a.wrap(h, mw))
}

// NotFound sets the handler for unknown routes and methods.
func (a *App) NotFound(h http.HandlerFunc) {
	a.mux.NotFound(a.wrap(h, nil).ServeHTTP)
	a.mux.MethodNotAllowed(a.wrap(h, nil).ServeHTTP)
}

// ServeHTTP implements http.Handler.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *App) wrap(h http.Handler, mw []Middleware) http.Handler {
	h = wrapMiddleware(mw, h)
	return wrapMiddleware(a.mw, h)
}

// wrapMiddleware wraps h so the first middleware in mw runs first.
func wrapMiddleware(mw []Middleware, h http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		if mw[i] != nil {
			h = mw[i](h)
		}
	}
	return h
}
