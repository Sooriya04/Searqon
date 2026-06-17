package utils

import (
	"encoding/json"
	"net/http"
)

// APIResponse wraps all API responses.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents structured error responses.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Global API error codes
const (
	ErrCodeInvalidJSON     = "INVALID_JSON"
	ErrCodeMissingParam    = "MISSING_PARAM"
	ErrCodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	ErrCodeScrapeFailed    = "SCRAPE_FAILED"
	ErrCodeSearchFailed    = "SEARCH_FAILED"
	ErrCodeInternalError   = "INTERNAL_ERROR"
	ErrCodeTimeout         = "TIMEOUT"
)

func writeJSON(w http.ResponseWriter, status int, val interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(val)
}

// WriteSuccess writes a structured success response.
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// WriteError writes a structured error response.
func WriteError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: msg,
		},
	})
}
