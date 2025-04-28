package healthchecker

import (
	"loadbalancer/internal/balancer"
	"loadbalancer/internal/config"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthChecker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

	t.Run("NewHealthChecker set default values", func(t *testing.T) {
		mockBalancer := new(MockBalancer)
		cfg := config.HealthChecker{}
		hc := NewHealthChecker(mockBalancer, logger, cfg)

		assert.Equal(t, 10*time.Second, hc.interval)
		assert.Equal(t, "/health", hc.healthPath)
		assert.Equal(t, 5*time.Second, hc.timeout)
		assert.NotNil(t, hc.stopChan)
	})

	t.Run("Start and Stop health checker", func(t *testing.T) {
		mockBalancer := new(MockBalancer)
		cfg := config.HealthChecker{
			Interval:   100 * time.Millisecond,
			HealthPath: "/health",
			Timeout:    50 * time.Millisecond,
		}
		hc := NewHealthChecker(mockBalancer, logger, cfg)

		mockBalancer.On("GetAllBackends").Return([]*balancer.Backend{}).Maybe()

		done := make(chan struct{})
		go func() {
			hc.Start()
			close(done)
		}()

		time.Sleep(250 * time.Millisecond)
		hc.Stop()

		select {
		case <-done:
			// горутины завершились успешно
		case <-time.After(500 * time.Millisecond):
			t.Error("Goroutines didn't finish on time")
		}
	})

	t.Run("checkAllBackends invokes checkBackendHealth", func(t *testing.T) {
		mockBalancer := new(MockBalancer)
		cfg := config.HealthChecker{
			Interval: 1 * time.Second,
			Timeout:  1 * time.Second,
		}
		hc := NewHealthChecker(mockBalancer, logger, cfg)

		u, _ := url.Parse("http://localhost:8080")
		mockBackend := &balancer.Backend{URL: u}
		mockBalancer.On("GetAllBackends").Return([]*balancer.Backend{mockBackend})

		hc.checkAllBackends()

		mockBalancer.AssertCalled(t, "GetAllBackends")
	})

	t.Run("checkBackendHealth marks backend as healthy", func(t *testing.T) {
		mockBalancer := new(MockBalancer)
		cfg := config.HealthChecker{
			Timeout: 500 * time.Millisecond,
		}
		hc := NewHealthChecker(mockBalancer, logger, cfg)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		u, _ := url.Parse(server.URL)
		mockBackend := &balancer.Backend{URL: u}
		mockBackend.SetHealth(false)

		hc.checkBackendHealth(mockBackend)
		time.Sleep(100 * time.Millisecond)

		assert.False(t, mockBackend.IsDown())
	})
}
