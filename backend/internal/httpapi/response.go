package httpapi

import (
	"encoding/json"
	"log/slog"
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

func writeSuccess(logger *slog.Logger, w http.ResponseWriter, status int, requestID string, data any) {
	writeJSON(logger, w, status, requestID, successResponse{
		Data: data,
		Meta: newMeta(requestID),
	})
}

func writeError(logger *slog.Logger, w http.ResponseWriter, status int, requestID, code, message string, details map[string]any) {
	writeJSON(logger, w, status, requestID, errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
		Meta: newMeta(requestID),
	})
}

func writeJSON(logger *slog.Logger, w http.ResponseWriter, status int, requestID string, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Warn("http response json write failed", "request_id", requestID, "status", status, "error", err.Error())
	}
}

func newMeta(requestID string) meta {
	return meta{
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
}
