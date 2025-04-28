package handler

import (
	"encoding/json"
	"loadbalancer/internal/lib/api/response"
	ratelimiter "loadbalancer/internal/rate_limiter"
	"log/slog"
	"net/http"
	"strings"
)

func createClientHandler(rl *ratelimiter.RateLimiter, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientID   string  `json:"client_id"`
			Capacity   float64 `json:"capacity"`
			RatePerSec float64 `json:"rate_per_sec"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid request", log)
			return
		}

		// проверяем существование клиента
		if _, _, exists := rl.GetClient(req.ClientID); exists {
			response.Error(w, http.StatusConflict, "Client already exists", log)
			return
		}

		rl.SetClientLimit(req.ClientID, req.Capacity, req.RatePerSec)

		w.WriteHeader(http.StatusCreated)
	}
}

func getClientHandler(rl *ratelimiter.RateLimiter, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
		clientID := strings.Split(path, "/")[0]

		capacity, rate, exists := rl.GetClient(clientID)
		if !exists {
			response.Error(w, http.StatusNotFound, "Client not found", log)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"client_id":    clientID,
			"capacity":     capacity,
			"rate_per_sec": rate,
		})
	}
}

func updateClientHandler(rl *ratelimiter.RateLimiter, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
		clientID := strings.Split(path, "/")[0]

		var req struct {
			Capacity   float64 `json:"capacity"`
			RatePerSec float64 `json:"rate_per_sec"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid requst", log)
			return
		}

		if err := rl.UpdateClientLimit(clientID, req.Capacity, req.RatePerSec); err != nil {
			response.Error(w, http.StatusNotFound, "Client not found", log)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func deleteClientHandler(rl *ratelimiter.RateLimiter, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
		clientID := strings.Split(path, "/")[0]

		if _, _, exists := rl.GetClient(clientID); !exists {
			response.Error(w, http.StatusNotFound, "Client not found", log)
			return
		}

		rl.RemoveClient(clientID)
		w.WriteHeader(http.StatusNoContent)
	}
}
