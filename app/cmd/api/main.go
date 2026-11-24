package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // America/Los_Angeles for NOAA times, even in minimal images

	"github.com/leogoesger/yakkkkk-api/cmd/api/handlers"
	"github.com/leogoesger/yakkkkk-api/internal/config"
	"github.com/leogoesger/yakkkkk-api/internal/proxy"
	"github.com/leogoesger/yakkkkk-api/internal/water"
)

// version is set at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.FromEnv()
	if err != nil {
		log.Error("invalid config", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handlers.NewAPI(cfg, log, version, proxy.Transport(cfg.UpstreamTimeout), water.NewClient()),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		// No WriteTimeout: upstream responses may stream for a long time.
	}

	routes := make([]string, len(cfg.Routes))
	for i, r := range cfg.Routes {
		routes[i] = r.Name
	}
	log.Info("listening", "addr", cfg.Addr, "version", version, "routes", routes)

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errc:
		log.Error("server failed", "err", err)
		os.Exit(1)
	case <-ctx.Done():
	}

	// Stop accepting connections and let in-flight requests finish.
	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Error("shutdown", "err", err)
	}
	log.Info("shut down")
}
