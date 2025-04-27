package healthchecker

import (
	"loadbalancer/internal/balancer"
	"loadbalancer/internal/config"
	"loadbalancer/internal/lib/sl"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Подразумивается что у бэкендов есть endpoint для проверки здоровья
// пример: GET /health

type HealthChecker struct {
	balancer   balancer.Balancer
	log        *slog.Logger
	interval   time.Duration
	healthPath string
	timeout    time.Duration
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

func NewHealthChecker(balancer balancer.Balancer, logger *slog.Logger, config config.HealthChecker) *HealthChecker {
	if config.Interval == 0 {
		config.Interval = 10 * time.Second
	}
	if config.HealthPath == "" {
		config.HealthPath = "/health"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	return &HealthChecker{
		balancer:   balancer,
		log:        logger,
		interval:   config.Interval,
		healthPath: config.HealthPath,
		timeout:    config.Timeout,
		stopChan:   make(chan struct{}),
	}
}

// Запускает периодические проверки здоровья бэкендов
func (hc *HealthChecker) Start() {
	hc.wg.Add(1)
	go func() {
		defer hc.wg.Done()

		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		hc.checkAllBackends()

		for {
			select {
			case <-ticker.C:
				hc.checkAllBackends()
			case <-hc.stopChan:
				return
			}
		}
	}()

	hc.log.Info("health checker started", slog.String("interval", hc.interval.String()))
}

// Останавливает проверки здоровья бэкендов
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
	hc.wg.Wait()
	hc.log.Info("health checker stopped")
}

// Проверяет здоровье всех бэкендов
func (hc *HealthChecker) checkAllBackends() {
	backends := hc.balancer.GetAllBackends()

	for _, backend := range backends {
		go hc.checkBackendHealth(backend)
	}
}

// Проверяет здоровье конкретного бэкенда
func (hc *HealthChecker) checkBackendHealth(backend *balancer.Backend) {

	healthURL := *backend.URL
	healthURL.Path = hc.healthPath

	client := http.Client{
		Timeout: hc.timeout,
	}

	resp, err := client.Get(healthURL.String())
	wasHealthy := !backend.IsDown()

	if err != nil {
		if wasHealthy {
			hc.log.Warn("backend is down", slog.String("url", backend.URL.String()), sl.Err(err))
			backend.SetHealth(true)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if !wasHealthy {
			hc.log.Info("backend is back online", slog.String("url", backend.URL.String()))
			backend.SetHealth(false)
		}
	} else {
		if wasHealthy {
			hc.log.Warn("backend is unhealthy", slog.String("url", backend.URL.String()), slog.Int("status", resp.StatusCode))
			backend.SetHealth(true)
		}
	}
}
