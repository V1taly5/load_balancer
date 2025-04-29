package proxy

import (
	"fmt"
	"loadbalancer/internal/balancer"
	"loadbalancer/internal/lib/sl"
	"log/slog"
	"net/http"
	"sync"
)

type retryRoundTripper struct {
	next        http.RoundTripper
	maxRetries  int
	maxBackends int
	balancer    balancer.Balancer
	// initBackend *balancer.Backend
	log  *slog.Logger
	mu   *sync.Mutex
	pool *ConnectionPool
}

func (rt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error

	// возвращаем транспорт в pool по завершению запроса
	defer func() {
		if rt.pool != nil && rt.next != nil {
			if transport, ok := rt.next.(*http.Transport); ok {
				rt.pool.ReleaseTransport(transport)
			}
		}
	}()

	for backendCount := range rt.maxBackends {
		rt.mu.Lock()
		backend, err := rt.balancer.Next()
		rt.mu.Unlock()
		if err != nil {
			rt.log.Error("failed to get backend", sl.Err(err))
			return nil, err
		}

		rt.log.Debug("selected backend",
			slog.String("backendURL", backend.URL.String()),
			slog.Int("backendAttempt", backendCount+1),
		)

		for retryBackend := range rt.maxRetries {
			reqCopy := req.Clone(req.Context())

			reqCopy.URL.Scheme = backend.URL.Scheme
			reqCopy.URL.Host = backend.URL.Host

			reqCopy.Header.Add("X-Forwarded-Host", req.Host)

			rt.log.Debug("trying backend",
				slog.String("backendURL", backend.URL.String()),
				slog.Int("backendCount", backendCount+1),
				slog.Int("retryBackend", retryBackend+1),
			)

			resp, err := rt.next.RoundTrip(reqCopy)

			if err == nil && resp.StatusCode < 500 {
				return resp, nil
			}

			if err != nil {
				rt.log.Error("request failed",
					slog.String("backendURL", backend.URL.String()),
					sl.Err(err),
					slog.Int("backendCount", backendCount+1),
					slog.Int("retryBackend", retryBackend+1),
				)
				lastErr = err
			} else {
				rt.log.Error("backend returned error",
					slog.String("backendURL", backend.URL.String()),
					slog.Int("status", resp.StatusCode),
					slog.Int("backendCount", backendCount+1),
					slog.Int("retryBackend", retryBackend+1),
				)
				resp.Body.Close()
			}
			if retryBackend == rt.maxRetries-1 {
				rt.log.Warn("marking backend as down",
					slog.String("backendURL", backend.URL.String()),
				)
				rt.mu.Lock()
				rt.balancer.MarkAsDown(backend)
				rt.mu.Unlock()
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all backends failed")
}
