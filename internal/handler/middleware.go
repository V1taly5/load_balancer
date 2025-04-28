package handler

import (
	ratelimiter "loadbalancer/internal/rate_limiter"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// Ограничение частоты запросов
func RateLimiterMiddleware(
	limiter *ratelimiter.RateLimiter,
	log *slog.Logger,
	headerIP string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r, headerIP)

			if !limiter.Allow(clientID) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				log.Warn("rate limit exeeded",
					slog.String("client_id", clientID),
					slog.String("path", r.URL.Path),
					slog.String("method", r.Method),
				)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Лигрируем информацию о запросе
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

// Получаем id клиента из запроса
func getClientID(r *http.Request, headerIP string) string {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return "api:" + apiKey
	}

	var ip string

	if headerIP != "" {
		headerIPs := r.Header.Get(headerIP)
		if headerIPs != "" {
			ip = strings.Split(headerIPs, ",")[0]
			ip = strings.TrimSpace(ip)
		}
	}

	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
	}

	return "ip:" + ip
}
