package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
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
	requestID := r.requestID(req)

	var payload sessionConnectRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(r.logger, w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "invalid JSON body", nil)
		return
	}

	if strings.TrimSpace(payload.Host) == "" || payload.Port <= 0 {
		writeError(r.logger, w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "host and port are required", nil)
		return
	}

	status, err := r.manager.Connect(payload.Host, payload.Port)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrSessionAlreadyConnected):
			writeError(r.logger, w, http.StatusConflict, requestID, "SESSION_ALREADY_CONNECTED", "session is already connected", map[string]any{
				"session_id": status.SessionID,
			})
			return
		case errors.Is(err, gateway.ErrUpstreamUnavailable):
			writeError(r.logger, w, http.StatusBadGateway, requestID, "UPSTREAM_UNAVAILABLE", "failed to reach upstream host", map[string]any{
				"host": payload.Host,
				"port": payload.Port,
			})
			return
		}

		r.logger.Error("session connect failed", "request_id", requestID, "host", payload.Host, "port", payload.Port, "error", err.Error())
		writeError(r.logger, w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "failed to connect session", nil)
		return
	}

	r.logger.Info("session connect request handled", "request_id", requestID, "session_id", status.SessionID)
	writeSuccess(r.logger, w, http.StatusOK, requestID, toSessionStatusPayload(status))
}

func (r *Router) handleSessionDisconnect(w http.ResponseWriter, req *http.Request) {
	requestID := r.requestID(req)

	status, dropped, err := r.manager.Disconnect()
	if err != nil {
		if errors.Is(err, gateway.ErrSessionNotConnected) {
			writeError(r.logger, w, http.StatusConflict, requestID, "SESSION_NOT_CONNECTED", "session is not connected", map[string]any{
				"session_id": status.SessionID,
			})
			return
		}

		r.logger.Error("session disconnect failed", "request_id", requestID, "session_id", status.SessionID, "error", err.Error())
		writeError(r.logger, w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "failed to disconnect session", nil)
		return
	}

	r.logger.Info("session disconnect request handled", "request_id", requestID, "session_id", status.SessionID, "dropped_unsent", dropped)
	writeSuccess(r.logger, w, http.StatusOK, requestID, toSessionStatusPayload(status))
}

func (r *Router) handleSessionStatus(w http.ResponseWriter, req *http.Request) {
	requestID := r.requestID(req)
	status := r.manager.Status()
	writeSuccess(r.logger, w, http.StatusOK, requestID, toSessionStatusPayload(status))
}

func (r *Router) handleEnqueueCommand(w http.ResponseWriter, req *http.Request) {
	requestID := r.requestID(req)

	var payload enqueueRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(r.logger, w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "invalid JSON body", nil)
		return
	}

	status, err := r.manager.Enqueue(payload.Command)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrSessionNotConnected):
			writeError(r.logger, w, http.StatusConflict, requestID, "SESSION_NOT_CONNECTED", "session is not connected", map[string]any{
				"session_id": status.SessionID,
			})
		case errors.Is(err, gateway.ErrQueueFull):
			writeError(r.logger, w, http.StatusTooManyRequests, requestID, "QUEUE_FULL", "command queue is full", map[string]any{
				"session_id":  status.SessionID,
				"queue_depth": status.QueueDepth,
				"queue_max":   status.QueueMax,
			})
		default:
			r.logger.Error("command enqueue failed", "request_id", requestID, "session_id", status.SessionID, "error", err.Error())
			writeError(r.logger, w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "failed to enqueue command", nil)
		}
		return
	}

	writeSuccess(r.logger, w, http.StatusOK, requestID, enqueuePayload{
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
			"# TYPE gateway_queue_send_failed_total counter\n"+
			"gateway_queue_send_failed_total %d\n"+
			"# TYPE gateway_queue_rejected_total counter\n"+
			"gateway_queue_rejected_total %d\n"+
			"# TYPE gateway_queue_dropped_total counter\n"+
			"gateway_queue_dropped_total %d\n"+
			"# TYPE gateway_events_broadcast_total counter\n"+
			"gateway_events_broadcast_total %d\n"+
			"# TYPE gateway_queue_send_latency_seconds histogram\n%s"+
			"gateway_queue_send_latency_seconds_sum %f\n"+
			"gateway_queue_send_latency_seconds_count %d\n"+
			"# TYPE gateway_queue_dropped_unsent_total counter\n%s",
		snapshot.WSConnections,
		snapshot.QueueDepth,
		snapshot.QueueSentTotal,
		snapshot.QueueSendFailedTotal,
		snapshot.QueueRejectedTotal,
		snapshot.QueueDroppedTotal,
		snapshot.EventsBroadcastTotal,
		formatLatencyBuckets(snapshot.QueueSendLatency),
		snapshot.QueueSendLatency.Sum,
		snapshot.QueueSendLatency.Count,
		formatDroppedBySession(snapshot.DroppedBySession),
	)
}

func formatLatencyBuckets(hist gateway.HistogramSnapshot) string {
	var b strings.Builder
	for _, bucket := range hist.Buckets {
		_, _ = fmt.Fprintf(&b, "gateway_queue_send_latency_seconds_bucket{le=%q} %d\n", bucket.Le, bucket.Count)
	}
	_, _ = fmt.Fprintf(&b, "gateway_queue_send_latency_seconds_bucket{le=%q} %d\n", "+Inf", hist.Count)
	return b.String()
}

func formatDroppedBySession(perSession map[string]int64) string {
	if len(perSession) == 0 {
		return ""
	}

	sessionIDs := make([]string, 0, len(perSession))
	for sessionID := range perSession {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Strings(sessionIDs)

	var b strings.Builder
	for _, sessionID := range sessionIDs {
		_, _ = fmt.Fprintf(&b, "gateway_queue_dropped_unsent_total{session_id=%q} %s\n", sessionID, strconv.FormatInt(perSession[sessionID], 10))
	}
	return b.String()
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
