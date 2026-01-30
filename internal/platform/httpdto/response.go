package httpdto

import (
	"encoding/json"
	"net/http"
	"time"
)

type Response[T any] struct {
	Status       string `json:"status"`        // "ok" or "error"
	Result       T      `json:"result,omitempty"`
	Error        *Error `json:"error,omitempty"`
	ResponseTime int64  `json:"responseTimeMs"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteSuccess(w http.ResponseWriter, start time.Time, result interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response[interface{}]{
		Status:       "ok",
		Result:       result,
		ResponseTime: time.Since(start).Milliseconds(),
	})
}

func WriteError(w http.ResponseWriter, start time.Time, code, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response[interface{}]{
		Status: "error",
		Error: &Error{
			Code:    code,
			Message: message,
		},
		ResponseTime: time.Since(start).Milliseconds(),
	})
}
