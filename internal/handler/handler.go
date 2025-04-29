package handler

import (
	"loadbalancer/internal/proxy"
	ratelimiter "loadbalancer/internal/rate_limiter"
	"log/slog"
	"net/http"
)

func SetupHandlers(proxyHandler *proxy.ReverseProxy, rateLimiter *ratelimiter.RateLimiter, headerIP string, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// эти обработчики так же будут учитывать rate limiter
	// если этого надо избежать можно в RateLimiterMiddleware
	// добавить исключение на некоторые маршруты
	mux.HandleFunc("POST /api/clients", createClientHandler(rateLimiter, log))
	mux.HandleFunc("GET /api/clients/", getClientHandler(rateLimiter, log))
	mux.HandleFunc("PUT /api/clients/", updateClientHandler(rateLimiter, log))
	mux.HandleFunc("DELETE /api/clients/", deleteClientHandler(rateLimiter, log))

	mux.Handle("/", proxyHandler)

	var handler http.Handler = mux
	if rateLimiter != nil {
		handler = RateLimiterMiddleware(rateLimiter, log, headerIP)(handler)
	}
	handler = LoggingMiddleware(handler, log)

	return handler

}
