package gateway

import "sync/atomic"

type Metrics struct {
	wsConnections        atomic.Int64
	queueDepth           atomic.Int64
	queueSentTotal       atomic.Int64
	queueRejectedTotal   atomic.Int64
	queueDroppedTotal    atomic.Int64
	eventsBroadcastTotal atomic.Int64
}

type MetricsSnapshot struct {
	WSConnections        int64
	QueueDepth           int64
	QueueSentTotal       int64
	QueueRejectedTotal   int64
	QueueDroppedTotal    int64
	EventsBroadcastTotal int64
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncWSConnections() {
	m.wsConnections.Add(1)
}

func (m *Metrics) DecWSConnections() {
	m.wsConnections.Add(-1)
}

func (m *Metrics) SetQueueDepth(depth int) {
	m.queueDepth.Store(int64(depth))
}

func (m *Metrics) IncQueueSent() {
	m.queueSentTotal.Add(1)
}

func (m *Metrics) IncQueueRejected() {
	m.queueRejectedTotal.Add(1)
}

func (m *Metrics) AddQueueDropped(count int) {
	m.queueDroppedTotal.Add(int64(count))
}

func (m *Metrics) IncEventsBroadcast() {
	m.eventsBroadcastTotal.Add(1)
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		WSConnections:        m.wsConnections.Load(),
		QueueDepth:           m.queueDepth.Load(),
		QueueSentTotal:       m.queueSentTotal.Load(),
		QueueRejectedTotal:   m.queueRejectedTotal.Load(),
		QueueDroppedTotal:    m.queueDroppedTotal.Load(),
		EventsBroadcastTotal: m.eventsBroadcastTotal.Load(),
	}
}
