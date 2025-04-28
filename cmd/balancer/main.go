package main

import (
	"flag"
	"fmt"
	"loadbalancer/internal/balancer"
	"loadbalancer/internal/config"
	"loadbalancer/internal/handler"
	healthchecker "loadbalancer/internal/health_checker"
	"loadbalancer/internal/lib/sl"
	"loadbalancer/internal/proxy"
	ratelimiter "loadbalancer/internal/rate_limiter"
	"loadbalancer/internal/server"
	"loadbalancer/internal/storage"
	"log/slog"
	"os"
)

const (
	envLocal = "local"
	envProd  = "prod"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}
	fmt.Println(cfg)

	log := setupLogger(cfg.Env)
	log.Info("starting load balancer", slog.String("with config", *configPath))

	lb := balancer.NewRoundRobinBalancer(log)

	for _, backendCfg := range cfg.Backends {
		backend, err := balancer.NewBackend(backendCfg)
		if err != nil {
			log.Error("failde to create backend", slog.String("backendURL", backendCfg.URL), sl.Err(err))
			continue
		}
		lb.AddBackend(*backend)
	}

	healthChecker := healthchecker.NewHealthChecker(lb, log, cfg.HealthChecker)
	healthChecker.Start()
	defer healthChecker.Stop()

	proxyHandler := proxy.NewReverseProxy(lb, log)

	// только существующий файл
	storage, err := storage.NewFileStorage(cfg.Storage.FilePath)
	if err != nil {
		log.Error("storage has not been created", sl.Err(err))
		return
	}

	var rateLimiter *ratelimiter.RateLimiter
	if cfg.RateLimiter.Enabled {
		rateLimiter = ratelimiter.NewRateLimiter(
			cfg.RateLimiter.DefaultCapacity,
			cfg.RateLimiter.DefaultRate,
			storage,
			log,
			ratelimiter.WithCleanupInterval(cfg.RateLimiter.CleanupInterval),
			ratelimiter.WithBucketTTL(cfg.RateLimiter.BucketTTL),
		)
		log.Info("Rate limiter initialized",
			slog.Float64("default_capacity", cfg.RateLimiter.DefaultCapacity),
			slog.Int64("default_rate", int64(cfg.RateLimiter.DefaultRate)),
		)
		defer rateLimiter.Stop()
	}

	headerIP := ""
	if cfg.RateLimiter.Enabled {
		headerIP = cfg.RateLimiter.HeaderIP
	}
	handler := handler.SetupHandlers(proxyHandler, rateLimiter, headerIP, log)

	srv := server.New(handler, &cfg.Server, log)
	if err := srv.Start(); err != nil {
		log.Error("server error", sl.Err(err))
		os.Exit(1)
	}

}

func setupLogger(env string) *slog.Logger {

	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case envProd:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return log
}
