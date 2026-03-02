package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/athanasius/arda-web-gateway/backend/internal/gateway"
)

type sessionConnectRequest struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	CredentialAlias string `json:"credential_alias"`
}

type sessionStatusPayload struct {
	SessionID      string `json:"session_id"`
	Connected      bool   `json:"connected"`
	QueueDepth     int    `json:"queue_depth"`
	QueueMax       int    `json:"queue_max"`
	LastActivityAt string `json:"last_activity_at,omitempty"`
}

type enqueueRequest struct {
	Command string `json:"command"`
}

type enqueuePayload struct {
	Accepted   bool `json:"accepted"`
	QueueDepth int  `json:"queue_depth"`
	QueueMax   int  `json:"queue_max"`
}

func (r *Router) handleSessionConnect(w http.ResponseWriter, req *http.Request) {
	requestID := r.nextRequestID()

	var payload sessionConnectRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "invalid JSON body", nil)
		return
	}

	if strings.TrimSpace(payload.Host) == "" || payload.Port <= 0 {
		writeError(w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "host and port are required", nil)
		return
	}

	status, err := r.manager.Connect(payload.Host, payload.Port)
	if err != nil {
		if errors.Is(err, gateway.ErrSessionAlreadyConnected) {
			writeError(w, http.StatusConflict, requestID, "SESSION_ALREADY_CONNECTED", "session is already connected", map[string]any{
				"session_id": status.SessionID,
			})
			return
		}

		writeError(w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "failed to connect session", nil)
		return
	}

	r.logger.Info("session connect request handled", "request_id", requestID, "session_id", status.SessionID)
	writeSuccess(w, http.StatusOK, requestID, toSessionStatusPayload(status))
}

func (r *Router) handleSessionDisconnect(w http.ResponseWriter, req *http.Request) {
	requestID := r.nextRequestID()

	status, dropped, err := r.manager.Disconnect()
	if err != nil {
		if errors.Is(err, gateway.ErrSessionNotConnected) {
			writeError(w, http.StatusConflict, requestID, "SESSION_NOT_CONNECTED", "session is not connected", map[string]any{
				"session_id": status.SessionID,
			})
			return
		}

		writeError(w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "failed to disconnect session", nil)
		return
	}

	r.logger.Info("session disconnect request handled", "request_id", requestID, "session_id", status.SessionID, "dropped_unsent", dropped)
	writeSuccess(w, http.StatusOK, requestID, toSessionStatusPayload(status))
}

func (r *Router) handleSessionStatus(w http.ResponseWriter, req *http.Request) {
	requestID := r.nextRequestID()
	status := r.manager.Status()
	writeSuccess(w, http.StatusOK, requestID, toSessionStatusPayload(status))
}

func (r *Router) handleEnqueueCommand(w http.ResponseWriter, req *http.Request) {
	requestID := r.nextRequestID()

	var payload enqueueRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "invalid JSON body", nil)
		return
	}

	payload.Command = strings.TrimSpace(payload.Command)
	if payload.Command == "" {
		writeError(w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "command is required", nil)
		return
	}

	status, err := r.manager.Enqueue(payload.Command)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrSessionNotConnected):
			writeError(w, http.StatusConflict, requestID, "SESSION_NOT_CONNECTED", "session is not connected", map[string]any{
				"session_id": status.SessionID,
			})
		case errors.Is(err, gateway.ErrQueueFull):
			writeError(w, http.StatusTooManyRequests, requestID, "QUEUE_FULL", "command queue is full", map[string]any{
				"session_id":  status.SessionID,
				"queue_depth": status.QueueDepth,
				"queue_max":   status.QueueMax,
			})
		default:
			writeError(w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "failed to enqueue command", nil)
		}
		return
	}

	writeSuccess(w, http.StatusOK, requestID, enqueuePayload{
		Accepted:   true,
		QueueDepth: status.QueueDepth,
		QueueMax:   status.QueueMax,
	})
}

func (r *Router) handleMetrics(w http.ResponseWriter, req *http.Request) {
	snapshot := r.metrics.Snapshot()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w,
		"# TYPE gateway_ws_connections gauge\n"+
			"gateway_ws_connections %d\n"+
			"# TYPE gateway_queue_depth gauge\n"+
			"gateway_queue_depth %d\n"+
			"# TYPE gateway_queue_sent_total counter\n"+
			"gateway_queue_sent_total %d\n"+
			"# TYPE gateway_queue_rejected_total counter\n"+
			"gateway_queue_rejected_total %d\n"+
			"# TYPE gateway_queue_dropped_total counter\n"+
			"gateway_queue_dropped_total %d\n"+
			"# TYPE gateway_events_broadcast_total counter\n"+
			"gateway_events_broadcast_total %d\n",
		snapshot.WSConnections,
		snapshot.QueueDepth,
		snapshot.QueueSentTotal,
		snapshot.QueueRejectedTotal,
		snapshot.QueueDroppedTotal,
		snapshot.EventsBroadcastTotal,
	)
}

func toSessionStatusPayload(status gateway.Status) sessionStatusPayload {
	payload := sessionStatusPayload{
		SessionID:  status.SessionID,
		Connected:  status.Connected,
		QueueDepth: status.QueueDepth,
		QueueMax:   status.QueueMax,
	}

	if !status.LastActivity.IsZero() {
		payload.LastActivityAt = status.LastActivity.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}

	return payload
}
