package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func Error(w http.ResponseWriter, statusCode int, message string, log *slog.Logger) {
	errorResp := ErrorResponse{
		Code:    statusCode,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		log.Error("failed to encode error response",
			slog.String("error", err.Error()),
		)
	}
}
