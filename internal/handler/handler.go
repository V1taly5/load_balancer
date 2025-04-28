package handler

import (
	"loadbalancer/internal/proxy"
	ratelimiter "loadbalancer/internal/rate_limiter"
	"log/slog"
	"net/http"
)

func SetupHandlers(proxyHandler *proxy.ReverseProxy, rateLimiter *ratelimiter.RateLimiter, headerIP string, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/clients", createClientHandler(rateLimiter))
	mux.HandleFunc("GET /api/clients/", getClientHandler(rateLimiter))
	mux.HandleFunc("PUT /api/clients/", updateClientHandler(rateLimiter))
	mux.HandleFunc("DELETE /api/clients/", deleteClientHandler(rateLimiter))

	mux.Handle("/", proxyHandler)

	var handler http.Handler = mux
	if rateLimiter != nil {
		handler = RateLimiterMiddleware(rateLimiter, log, headerIP)(handler)
	}
	handler = LoggingMiddleware(handler, log)

	return handler

}
