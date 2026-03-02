package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

type meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

type successResponse struct {
	Data any  `json:"data"`
	Meta meta `json:"meta"`
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
	Meta  meta      `json:"meta"`
}

func writeSuccess(w http.ResponseWriter, status int, requestID string, data any) {
	writeJSON(w, status, successResponse{
		Data: data,
		Meta: newMeta(requestID),
	})
}

func writeError(w http.ResponseWriter, status int, requestID, code, message string, details map[string]any) {
	writeJSON(w, status, errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
		Meta: newMeta(requestID),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func newMeta(requestID string) meta {
	return meta{
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
}
