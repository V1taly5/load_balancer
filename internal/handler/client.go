package handler

import (
	"encoding/json"
	ratelimiter "loadbalancer/internal/rate_limiter"
	"net/http"
	"strings"
)

func createClientHandler(rl *ratelimiter.RateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientID   string  `json:"client_id"`
			Capacity   float64 `json:"capacity"`
			RatePerSec float64 `json:"rate_per_sec"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// проверяем существование клиента
		if _, _, exists := rl.GetClient(req.ClientID); exists {
			http.Error(w, "Client already exists", http.StatusConflict)
			return
		}

		rl.SetClientLimit(req.ClientID, req.Capacity, req.RatePerSec)
		w.WriteHeader(http.StatusCreated)
	}
}

func getClientHandler(rl *ratelimiter.RateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
		clientID := strings.Split(path, "/")[0]

		capacity, rate, exists := rl.GetClient(clientID)
		if !exists {
			http.Error(w, "Client not found", http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"client_id":    clientID,
			"capacity":     capacity,
			"rate_per_sec": rate,
		})
	}
}

func updateClientHandler(rl *ratelimiter.RateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
		clientID := strings.Split(path, "/")[0]

		var req struct {
			Capacity   float64 `json:"capacity"`
			RatePerSec float64 `json:"rate_per_sec"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := rl.UpdateClientLimit(clientID, req.Capacity, req.RatePerSec); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func deleteClientHandler(rl *ratelimiter.RateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
		clientID := strings.Split(path, "/")[0]

		if _, _, exists := rl.GetClient(clientID); !exists {
			http.Error(w, "Client not found", http.StatusNotFound)
			return
		}

		rl.RemoveClient(clientID)
		w.WriteHeader(http.StatusNoContent)
	}
}
