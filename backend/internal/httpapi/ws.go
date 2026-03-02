package httpapi

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"

	"github.com/athanasius/arda-web-gateway/backend/internal/gateway"
)

func (r *Router) handleTerminalWS(w http.ResponseWriter, req *http.Request) {
	if !headerContainsToken(req.Header, "Connection", "upgrade") || !headerContainsToken(req.Header, "Upgrade", "websocket") {
		requestID := r.nextRequestID()
		writeError(w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "websocket upgrade required", nil)
		return
	}

	key := strings.TrimSpace(req.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		requestID := r.nextRequestID()
		writeError(w, http.StatusBadRequest, requestID, "INVALID_REQUEST", "missing Sec-WebSocket-Key", nil)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		requestID := r.nextRequestID()
		writeError(w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "websocket upgrade unsupported", nil)
		return
	}

	netConn, rw, err := hijacker.Hijack()
	if err != nil {
		r.logger.Warn("websocket hijack failed", "error", err.Error())
		return
	}

	if err := writeUpgradeResponse(rw, key); err != nil {
		r.logger.Warn("websocket upgrade response failed", "error", err.Error())
		_ = netConn.Close()
		return
	}

	client := r.manager.Hub().Add(netConn)
	r.manager.Hub().Broadcast(r.manager.BuildStatusEvent())

	defer func() {
		r.manager.Hub().Remove(client)
	}()

	r.manager.Hub().ReadLoop(client)
}

func writeUpgradeResponse(rw *bufio.ReadWriter, key string) error {
	response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Accept: %s\r\n"+
		"\r\n", gateway.WebsocketAcceptKey(key))

	if _, err := rw.WriteString(response); err != nil {
		return err
	}

	return rw.Flush()
}

func headerContainsToken(header http.Header, key, want string) bool {
	value := strings.ToLower(header.Get(key))
	for _, part := range strings.Split(value, ",") {
		if strings.TrimSpace(part) == want {
			return true
		}
	}
	return false
}
