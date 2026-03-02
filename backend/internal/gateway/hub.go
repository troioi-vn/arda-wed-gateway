package gateway

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

type Hub struct {
	mu      sync.RWMutex
	logger  *slog.Logger
	metrics *Metrics
	clients map[*client]struct{}
}

type client struct {
	conn net.Conn
	send chan TerminalEvent
}

func NewHub(logger *slog.Logger, metrics *Metrics) *Hub {
	return &Hub{
		logger:  logger,
		metrics: metrics,
		clients: make(map[*client]struct{}),
	}
}

func (h *Hub) Add(conn net.Conn) *client {
	c := &client{
		conn: conn,
		send: make(chan TerminalEvent, 32),
	}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	h.metrics.IncWSConnections()

	go c.writePump(h.logger)
	return c
}

func (h *Hub) Remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; !ok {
		h.mu.Unlock()
		return
	}
	delete(h.clients, c)
	h.mu.Unlock()

	h.metrics.DecWSConnections()
	close(c.send)
	_ = c.conn.Close()
}

func (h *Hub) Broadcast(event TerminalEvent) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.send <- event:
		default:
			h.logger.Warn("websocket client slow, disconnecting")
			h.Remove(c)
		}
	}
}

func (h *Hub) ReadLoop(c *client) {
	for {
		if err := c.conn.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return
		}

		opcode, _, err := readFrame(c.conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				h.logger.Debug("websocket read failed", "error", err.Error())
			}
			return
		}

		if opcode == wsOpcodeClose {
			return
		}
	}
}

func (c *client) writePump(logger *slog.Logger) {
	for event := range c.send {
		if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return
		}

		payload, err := marshalEvent(event)
		if err != nil {
			logger.Debug("websocket event marshal failed", "error", err.Error())
			continue
		}

		if err := writeTextFrame(c.conn, payload); err != nil {
			logger.Debug("websocket write failed", "error", err.Error())
			return
		}
	}
}
