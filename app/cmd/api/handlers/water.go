package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/leogoesger/yakkkkk-api/internal/water"
)

// WaterAPI provides handler funcs for the river and gauge data endpoints.
// Request and response shapes match the original foodnome-api /api/proxy routes.
type WaterAPI struct {
	log    *slog.Logger
	client *water.Client
}

// NewWaterAPI returns a pointer to an instance of WaterAPI.
func NewWaterAPI(log *slog.Logger, client *water.Client) *WaterAPI {
	return &WaterAPI{log: log, client: client}
}

// CDEC handles POST /proxy/cdec with body {"station": "...", "start": "..."}.
func (a *WaterAPI) CDEC(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Station string `json:"station"`
		Start   string `json:"start"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Station == "" {
		respondStatus(w, http.StatusBadRequest, errorBody("station is required"))
		return
	}
	body, contentType, err := a.client.CDEC(r.Context(), req.Station, req.Start)
	if err != nil {
		a.log.Error("cdec", "err", err)
		respondStatus(w, http.StatusBadRequest, errorBody("Failed to fetch CDEC data"))
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Write(body)
}

// GaugeReadings handles POST /proxy/aw with American Whitewater gauge params.
func (a *WaterAPI) GaugeReadings(w http.ResponseWriter, r *http.Request) {
	var params water.GaugeParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil || params.GaugeSourceIdentifier == "" {
		respondStatus(w, http.StatusBadRequest, errorBody("gaugeSourceIdentifier is required"))
		return
	}
	readings, err := a.client.GaugeReadings(r.Context(), params)
	if err != nil {
		a.log.Error("american whitewater", "err", err)
		respondStatus(w, http.StatusInternalServerError, errorBody("Failed to fetch gauge readings"))
		return
	}
	respond(w, readings)
}

// EID handles GET /proxy/eid?id=10&projectid=1.
func (a *WaterAPI) EID(w http.ResponseWriter, r *http.Request) {
	siteID, projectID := r.URL.Query().Get("id"), r.URL.Query().Get("projectid")
	if siteID == "" {
		siteID = "10"
	}
	if projectID == "" {
		projectID = "1"
	}
	points, err := a.client.EID(r.Context(), a.log, siteID, projectID)
	if err != nil {
		a.log.Error("eid", "err", err)
		respondStatus(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Failed to fetch EID water data"})
		return
	}
	respond(w, points)
}

// NOAA handles GET /proxy/noaa?id=FOCC1.
func (a *WaterAPI) NOAA(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		respondStatus(w, http.StatusBadRequest, errorBody("Station ID is required"))
		return
	}
	points, err := a.client.NOAA(r.Context(), id)
	if err != nil {
		a.log.Error("noaa", "err", err)
		respondStatus(w, http.StatusBadRequest, errorBody("Failed to fetch NOAA data"))
		return
	}
	respond(w, points)
}

func errorBody(msg string) map[string]string { return map[string]string{"error": msg} }
