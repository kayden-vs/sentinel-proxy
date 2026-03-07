package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kayden-vs/sentinel-proxy/internal/config"
	"github.com/kayden-vs/sentinel-proxy/internal/metrics"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.Logging.Level),
	}))
	slog.SetDefault(logger)

	logger.Info("sentinel-proxy starting", "config", cfg.String())

	p, err := newProxy(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize proxy", "error", err)
		os.Exit(1)
	}
	defer p.close()

	mux := http.NewServeMux()
	mux.HandleFunc("/data", p.handleData)
	mux.HandleFunc("/export", p.handleExport)
	mux.HandleFunc("/health", p.handleHealth)
	mux.HandleFunc("/simulate/normal", p.handleSimulateNormal)
	mux.HandleFunc("/simulate/attack", p.handleSimulateAttack)
	mux.HandleFunc("/simulate/export", p.handleSimulateExport)

	handler := p.concurrencyMiddleware(p.loggingMiddleware(mux))

	server := &http.Server{
		Addr:         cfg.Proxy.ListenAddr,
		Handler:      handler,
		ReadTimeout:  cfg.Proxy.ReadTimeout,
		WriteTimeout: cfg.Proxy.WriteTimeout,
		IdleTimeout:  cfg.Proxy.IdleTimeout,
	}

	var metricsServer *http.Server
	if cfg.Metrics.Enabled {
		metricsMux := http.NewServeMux()
		metricsMux.Handle(cfg.Metrics.Path, metrics.Handler())
		metricsServer = &http.Server{
			Addr:    cfg.Metrics.ListenAddr,
			Handler: metricsMux,
		}
		go func() {
			logger.Info("metrics server starting", "addr", cfg.Metrics.ListenAddr, "path", cfg.Metrics.Path)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("metrics server failed", "error", err)
			}
		}()
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Info("received shutdown signal", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if metricsServer != nil {
			metricsServer.Shutdown(ctx)
		}
		server.Shutdown(ctx)
	}()

	logger.Info("sentinel-proxy listening",
		"addr", cfg.Proxy.ListenAddr,
		"backend", cfg.Proxy.BackendAddr,
	)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("proxy server failed", "error", err)
		os.Exit(1)
	}

	logger.Info("sentinel-proxy shutdown complete")
}
