package handlers

import (
	"net/http"

	"github.com/leogoesger/yakkkkk-api/internal/proxy"
)

func (a *api) addRoutes() {
	// ================================= Health ===============================
	a.Handle(http.MethodGet, "/status", http.HandlerFunc(a.check.Status))
	a.Handle(http.MethodGet, "/version", http.HandlerFunc(a.check.Version))

	// ================================= Water data ===========================
	a.Handle(http.MethodPost, "/proxy/cdec", http.HandlerFunc(a.water.CDEC), a.maxBodyMW)
	a.Handle(http.MethodPost, "/proxy/aw", http.HandlerFunc(a.water.GaugeReadings), a.maxBodyMW)
	a.Handle(http.MethodGet, "/proxy/eid", http.HandlerFunc(a.water.EID))
	a.Handle(http.MethodGet, "/proxy/noaa", http.HandlerFunc(a.water.NOAA))

	// ================================= Upstreams ============================
	// One route per configured upstream: /{name}/* -> upstream base URL.
	// Unknown paths are 404, so only configured upstreams are reachable.
	for _, route := range a.cfg.Routes {
		a.HandleAll("/"+route.Name+"/*", proxy.New(route, a.transport, a.log), a.maxBodyMW)
	}
}
