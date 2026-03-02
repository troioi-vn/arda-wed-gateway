package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSessionAlreadyConnected = errors.New("session already connected")
	ErrSessionNotConnected     = errors.New("session not connected")
)

type TerminalEvent struct {
	Event      string `json:"event"`
	SessionID  string `json:"session_id"`
	Text       string `json:"text,omitempty"`
	QueueDepth int    `json:"queue_depth,omitempty"`
	QueueMax   int    `json:"queue_max,omitempty"`
	Timestamp  string `json:"timestamp"`
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

	session    *session
	sessionSeq atomic.Uint64
}

type session struct {
	id           string
	connected    bool
	lastActivity time.Time
	queue        *CommandQueue
}

func NewManager(queueInterval time.Duration, queueMaxDepth int, logger *slog.Logger, metrics *Metrics) *Manager {
	m := &Manager{
		logger:        logger,
		metrics:       metrics,
		queueInterval: queueInterval,
		queueMaxDepth: queueMaxDepth,
	}
	m.hub = NewHub(logger, metrics)
	return m
}

func (m *Manager) Hub() *Hub {
	return m.hub
}

func (m *Manager) Connect(host string, port int) (Status, error) {
	m.mu.Lock()
	s := m.ensureSessionLocked()
	if s.connected {
		status := m.statusLocked()
		m.mu.Unlock()
		return status, ErrSessionAlreadyConnected
	}

	s.connected = true
	s.lastActivity = time.Now().UTC()
	s.queue.Start()
	status := m.statusLocked()
	m.mu.Unlock()

	m.logger.Info("session connected", "session_id", status.SessionID, "host", host, "port", port)
	m.Broadcast(TerminalEvent{
		Event:      "session.connected",
		SessionID:  status.SessionID,
		Text:       fmt.Sprintf("connected to %s:%d", host, port),
		QueueDepth: status.QueueDepth,
		QueueMax:   status.QueueMax,
		Timestamp:  nowRFC3339Nano(),
	})
	return status, nil
}

func (m *Manager) Disconnect() (Status, int, error) {
	m.mu.Lock()
	s := m.ensureSessionLocked()
	if !s.connected {
		status := m.statusLocked()
		m.mu.Unlock()
		return status, 0, ErrSessionNotConnected
	}

	dropped := s.queue.StopAndDrop()
	s.connected = false
	s.lastActivity = time.Now().UTC()
	status := m.statusLocked()
	m.mu.Unlock()

	m.logger.Info("session disconnected", "session_id", status.SessionID, "dropped_unsent", dropped)
	m.Broadcast(TerminalEvent{
		Event:      "session.disconnected",
		SessionID:  status.SessionID,
		Text:       fmt.Sprintf("session disconnected, dropped_unsent=%d", dropped),
		QueueDepth: status.QueueDepth,
		QueueMax:   status.QueueMax,
		Timestamp:  nowRFC3339Nano(),
	})
	return status, dropped, nil
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
				Event:      "queue.rejected",
				SessionID:  status.SessionID,
				Text:       "command rejected: queue full",
				QueueDepth: depth,
				QueueMax:   s.queue.MaxDepth(),
				Timestamp:  nowRFC3339Nano(),
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
		QueueMax:   s.queue.MaxDepth(),
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

	s.queue = NewCommandQueue(m.queueMaxDepth, m.queueInterval, func(ctx context.Context, command string) error {
		_ = ctx
		m.onCommandSent(sessionID, command)
		return nil
	}, m.logger.With("session_id", sessionID), m.metrics)

	m.session = s
	return s
}

func (m *Manager) onCommandSent(sessionID, command string) {
	m.mu.Lock()
	if m.session != nil && m.session.id == sessionID {
		m.session.lastActivity = time.Now().UTC()
	}
	m.mu.Unlock()

	m.Broadcast(TerminalEvent{
		Event:     "terminal.output",
		SessionID: sessionID,
		Text:      "> " + command,
		Timestamp: nowRFC3339Nano(),
	})
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
