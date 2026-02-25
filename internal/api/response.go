package api

import (
	"encoding/json"
	"infinite-experiment/politburo/internal/models/dtos/responses"
	"net/http"
	"time"
)

// DEPRECATED: Use common.RespondSuccess instead.
// These functions lack response time tracking and use inconsistent response structures.
// They will be removed in a future release.
func respondWithSuccess[T any](w http.ResponseWriter, statusCode int, data *T) {
	resp := responses.APIResponse[T]{
		Status:    "success",
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}

// DEPRECATED: Use common.RespondError instead.
// These functions lack response time tracking and use inconsistent response structures.
// They will be removed in a future release.
func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	resp := responses.APIResponse[any]{
		Status:    "error",
		Timestamp: time.Now().UTC(),
		Error:     message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(resp)
}
