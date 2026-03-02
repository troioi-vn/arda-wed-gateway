package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSessionAlreadyConnected = errors.New("session already connected")
	ErrSessionNotConnected     = errors.New("session not connected")
	ErrUpstreamUnavailable     = errors.New("upstream unavailable")
)

type DialFunc func(context.Context, string, string) (net.Conn, error)

type TerminalEvent struct {
	Event           string `json:"event"`
	SessionID       string `json:"session_id"`
	Connected       bool   `json:"connected,omitempty"`
	Text            string `json:"text,omitempty"`
	QueueDepth      int    `json:"queue_depth,omitempty"`
	QueueMax        int    `json:"queue_max,omitempty"`
	QueueRejectCode string `json:"queue_reject_code,omitempty"`
	Timestamp       string `json:"timestamp"`
}

type Status struct {
	SessionID    string
	Connected    bool
	QueueDepth   int
	QueueMax     int
	LastActivity time.Time
}

type Manager struct {
	mu      sync.RWMutex
	logger  *slog.Logger
	metrics *Metrics
	hub     *Hub

	queueInterval time.Duration
	queueMaxDepth int
	dial          DialFunc

	session    *session
	sessionSeq atomic.Uint64
}

type session struct {
	id           string
	host         string
	port         int
	connected    bool
	lastActivity time.Time
	conn         net.Conn
	queue        *CommandQueue
}

func NewManager(queueInterval time.Duration, queueMaxDepth int, logger *slog.Logger, metrics *Metrics, dial DialFunc) *Manager {
	if dial == nil {
		dialer := &net.Dialer{Timeout: 8 * time.Second}
		dial = dialer.DialContext
	}

	m := &Manager{
		logger:        logger,
		metrics:       metrics,
		queueInterval: queueInterval,
		queueMaxDepth: queueMaxDepth,
		dial:          dial,
	}
	m.hub = NewHub(logger, metrics)
	return m
}

func (m *Manager) Hub() *Hub {
	return m.hub
}

func (m *Manager) Connect(host string, port int) (Status, error) {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := m.dial(context.Background(), "tcp", address)
	if err != nil {
		return Status{}, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}

	m.mu.Lock()
	s := m.ensureSessionLocked()
	if s.connected {
		m.mu.Unlock()
		_ = conn.Close()
		return m.statusLocked(), ErrSessionAlreadyConnected
	}

	s.host = host
	s.port = port
	s.conn = conn
	s.connected = true
	s.lastActivity = time.Now().UTC()
	s.queue.Start()
	status := m.statusLocked()
	m.mu.Unlock()

	m.logger.Info("session connected", "session_id", status.SessionID, "host", host, "port", port)
	m.Broadcast(TerminalEvent{
		Event:      "session.connected",
		SessionID:  status.SessionID,
		Connected:  true,
		Text:       fmt.Sprintf("connected to %s:%d", host, port),
		QueueDepth: status.QueueDepth,
		QueueMax:   status.QueueMax,
		Timestamp:  nowRFC3339Nano(),
	})

	go m.readUpstream(status.SessionID, conn)
	return status, nil
}

func (m *Manager) Disconnect() (Status, int, error) {
	return m.disconnect("", "manual disconnect", true)
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked()
	return m.statusLocked()
}

func (m *Manager) Enqueue(command string) (Status, error) {
	m.mu.Lock()
	s := m.ensureSessionLocked()
	if !s.connected {
		status := m.statusLocked()
		m.mu.Unlock()
		return status, ErrSessionNotConnected
	}

	depth, err := s.queue.Enqueue(command)
	if err != nil {
		status := m.statusLocked()
		m.mu.Unlock()
		if errors.Is(err, ErrQueueFull) {
			m.Broadcast(TerminalEvent{
				Event:           "queue.rejected",
				SessionID:       status.SessionID,
				Text:            "command rejected: queue full",
				QueueDepth:      depth,
				QueueMax:        s.queue.MaxDepth(),
				QueueRejectCode: "QUEUE_FULL",
				Timestamp:       nowRFC3339Nano(),
			})
		}
		return status, err
	}

	s.lastActivity = time.Now().UTC()
	status := m.statusLocked()
	status.QueueDepth = depth
	m.mu.Unlock()

	m.Broadcast(TerminalEvent{
		Event:      "queue.accepted",
		SessionID:  status.SessionID,
		Text:       command,
		QueueDepth: depth,
		QueueMax:   status.QueueMax,
		Timestamp:  nowRFC3339Nano(),
	})
	return status, nil
}

func (m *Manager) ensureSessionLocked() *session {
	if m.session != nil {
		return m.session
	}

	sessionID := fmt.Sprintf("s-%d", m.sessionSeq.Add(1))
	s := &session{
		id:           sessionID,
		connected:    false,
		lastActivity: time.Now().UTC(),
	}

	s.queue = NewCommandQueue(sessionID, m.queueMaxDepth, m.queueInterval, func(ctx context.Context, command string) error {
		_ = ctx
		return m.writeCommand(sessionID, command)
	}, m.logger.With("session_id", sessionID), m.metrics)

	m.session = s
	return s
}

func (m *Manager) writeCommand(sessionID, command string) error {
	m.mu.RLock()
	if m.session == nil || m.session.id != sessionID || !m.session.connected || m.session.conn == nil {
		m.mu.RUnlock()
		return ErrSessionNotConnected
	}
	conn := m.session.conn
	m.mu.RUnlock()

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if _, err := io.WriteString(conn, command+"\n"); err != nil {
		return err
	}

	m.mu.Lock()
	if m.session != nil && m.session.id == sessionID {
		m.session.lastActivity = time.Now().UTC()
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) readUpstream(sessionID string, conn net.Conn) {
	filter := &TelnetFilter{}
	decoder := NewTextDecoder()
	buffer := make([]byte, 4096)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
			m.handleUpstreamDisconnect(sessionID, fmt.Errorf("set read deadline failed: %w", err))
			return
		}

		n, err := conn.Read(buffer)
		if n > 0 {
			payload := filter.Filter(buffer[:n])
			if len(payload) > 0 {
				text := decoder.Decode(payload)
				m.onTerminalOutput(sessionID, text)
			}
		}

		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			m.handleUpstreamDisconnect(sessionID, err)
			return
		}
	}
}

func (m *Manager) onTerminalOutput(sessionID, text string) {
	m.mu.Lock()
	if m.session != nil && m.session.id == sessionID {
		m.session.lastActivity = time.Now().UTC()
	}
	m.mu.Unlock()

	m.Broadcast(TerminalEvent{
		Event:     "terminal.output",
		SessionID: sessionID,
		Text:      text,
		Timestamp: nowRFC3339Nano(),
	})
}

func (m *Manager) handleUpstreamDisconnect(sessionID string, err error) {
	reason := "upstream disconnected"
	if err != nil && !errors.Is(err, io.EOF) {
		reason = fmt.Sprintf("upstream disconnected: %v", err)
	}
	_, _, _ = m.disconnect(sessionID, reason, false)
}

func (m *Manager) disconnect(expectedSessionID, reason string, failIfNotConnected bool) (Status, int, error) {
	m.mu.Lock()
	s := m.ensureSessionLocked()
	if !s.connected {
		status := m.statusLocked()
		m.mu.Unlock()
		if failIfNotConnected {
			return status, 0, ErrSessionNotConnected
		}
		return status, 0, nil
	}
	if expectedSessionID != "" && s.id != expectedSessionID {
		status := m.statusLocked()
		m.mu.Unlock()
		return status, 0, nil
	}

	conn := s.conn
	s.conn = nil
	s.connected = false
	s.lastActivity = time.Now().UTC()
	queue := s.queue
	status := m.statusLocked()
	m.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	dropped := queue.StopAndDrop()
	status.QueueDepth = 0

	m.logger.Info("session disconnected", "session_id", status.SessionID, "reason", reason, "dropped_unsent", dropped)
	m.Broadcast(TerminalEvent{
		Event:      "session.disconnected",
		SessionID:  status.SessionID,
		Connected:  false,
		Text:       fmt.Sprintf("%s, dropped_unsent=%d", reason, dropped),
		QueueDepth: 0,
		QueueMax:   status.QueueMax,
		Timestamp:  nowRFC3339Nano(),
	})
	return status, dropped, nil
}

func (m *Manager) BuildStatusEvent() TerminalEvent {
	status := m.Status()
	return TerminalEvent{
		Event:      "session.status",
		SessionID:  status.SessionID,
		Connected:  status.Connected,
		QueueDepth: status.QueueDepth,
		QueueMax:   status.QueueMax,
		Timestamp:  nowRFC3339Nano(),
	}
}

func (m *Manager) Broadcast(event TerminalEvent) {
	m.metrics.IncEventsBroadcast()
	m.hub.Broadcast(event)
}

func (m *Manager) statusLocked() Status {
	if m.session == nil {
		return Status{}
	}

	return Status{
		SessionID:    m.session.id,
		Connected:    m.session.connected,
		QueueDepth:   m.session.queue.Depth(),
		QueueMax:     m.session.queue.MaxDepth(),
		LastActivity: m.session.lastActivity,
	}
}

func nowRFC3339Nano() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
