package proxy

import (
	"fmt"
	"loadbalancer/internal/balancer"
	"loadbalancer/internal/lib/sl"
	"log/slog"
	"net/http"
)

type retryRoundTripper struct {
	next        http.RoundTripper
	maxRetries  int
	maxBackends int
	balancer    balancer.Balancer
	// initBackend *balancer.Backend
	log *slog.Logger
}

func (rt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error

	for backendCount := range rt.maxBackends {
		backend, err := rt.balancer.Next()
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
			// reqCopy.Header.Add("X-Origin-Host", backend.URL.Host)

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
				rt.balancer.MarkAsDown(backend)
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all backends failed")
}
