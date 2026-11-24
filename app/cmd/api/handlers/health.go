package handlers

import (
	"encoding/json"
	"net/http"
)

// CheckAPI provides handler funcs for the health check API.
type CheckAPI struct {
	version string
}

// NewCheckAPI returns a pointer to an instance of CheckAPI.
func NewCheckAPI(version string) *CheckAPI {
	return &CheckAPI{version: version}
}

// Status is the handler for retrieving the status of the application.
func (check *CheckAPI) Status(w http.ResponseWriter, _ *http.Request) {
	respond(w, map[string]string{"status": "OK"})
}

// Version is the handler for retrieving the version of the application.
func (check *CheckAPI) Version(w http.ResponseWriter, _ *http.Request) {
	respond(w, map[string]string{"version": check.version})
}

func respond(w http.ResponseWriter, v any) { respondStatus(w, http.StatusOK, v) }

func respondStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
