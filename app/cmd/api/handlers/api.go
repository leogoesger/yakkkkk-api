package handlers

import (
	"log/slog"
	"net/http"

	"github.com/leogoesger/yakkkkk-api/internal/config"
	"github.com/leogoesger/yakkkkk-api/internal/mid"
	"github.com/leogoesger/yakkkkk-api/internal/water"
	"github.com/leogoesger/yakkkkk-api/internal/web"
)

type api struct {
	*web.App

	cfg       *config.Config
	log       *slog.Logger
	transport http.RoundTripper
	check     *CheckAPI
	water     *WaterAPI

	maxBodyMW web.Middleware
}

// NewAPI returns a Handler that handles all APIs for the application.
func NewAPI(cfg *config.Config, log *slog.Logger, version string, transport http.RoundTripper, waterClient *water.Client) http.Handler {
	// Initialize application with shared middleware.
	app := web.NewApp(
		mid.Logger(log),
	)
	app.Use(mid.CORS)

	handler := api{
		App:       app,
		cfg:       cfg,
		log:       log,
		transport: transport,
		check:     NewCheckAPI(version),
		water:     NewWaterAPI(log, waterClient),
		maxBodyMW: mid.MaxBody(cfg.MaxBodyBytes),
	}

	handler.addRoutes()

	return handler
}
