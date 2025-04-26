package handler

import (
	"loadbalancer/internal/proxy"
	"log/slog"
	"net/http"
	"time"
)

// лигрируем информацию о запросе
func LoggingMiddleware(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		duration := time.Since(start)

		log.Info("request processed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
			slog.String("duration", duration.String()),
		)

	})
}

func SetupHandler(proxyHandler *proxy.ReverseProxy, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/", proxyHandler)

	return LoggingMiddleware(mux, log)
}
