package gateway

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var queueSendLatencyBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

type Metrics struct {
	wsConnections        atomic.Int64
	queueDepth           atomic.Int64
	queueSentTotal       atomic.Int64
	queueRejectedTotal   atomic.Int64
	queueDroppedTotal    atomic.Int64
	eventsBroadcastTotal atomic.Int64

	queueSendLatencyCount atomic.Int64
	queueSendLatencySumNs atomic.Int64
	queueSendLatencyLe05  atomic.Int64
	queueSendLatencyLe10  atomic.Int64
	queueSendLatencyLe25  atomic.Int64
	queueSendLatencyLe50  atomic.Int64
	queueSendLatencyLe100 atomic.Int64
	queueSendLatencyLe250 atomic.Int64
	queueSendLatencyLe500 atomic.Int64

	droppedBySession sync.Map
}

type MetricsSnapshot struct {
	WSConnections        int64
	QueueDepth           int64
	QueueSentTotal       int64
	QueueRejectedTotal   int64
	QueueDroppedTotal    int64
	EventsBroadcastTotal int64
	QueueSendLatency     HistogramSnapshot
	DroppedBySession     map[string]int64
}

type HistogramSnapshot struct {
	Buckets []HistogramBucket
	Sum     float64
	Count   int64
}

type HistogramBucket struct {
	Le    string
	Count int64
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

func (m *Metrics) AddQueueDroppedForSession(sessionID string, count int) {
	if sessionID == "" || count <= 0 {
		return
	}

	counterAny, _ := m.droppedBySession.LoadOrStore(sessionID, &atomic.Int64{})
	counter := counterAny.(*atomic.Int64)
	counter.Add(int64(count))
}

func (m *Metrics) IncEventsBroadcast() {
	m.eventsBroadcastTotal.Add(1)
}

func (m *Metrics) ObserveQueueSendLatency(elapsed time.Duration) {
	if elapsed < 0 {
		return
	}

	seconds := elapsed.Seconds()
	m.queueSendLatencyCount.Add(1)
	m.queueSendLatencySumNs.Add(elapsed.Nanoseconds())

	switch {
	case seconds <= queueSendLatencyBuckets[0]:
		m.queueSendLatencyLe05.Add(1)
		m.queueSendLatencyLe10.Add(1)
		m.queueSendLatencyLe25.Add(1)
		m.queueSendLatencyLe50.Add(1)
		m.queueSendLatencyLe100.Add(1)
		m.queueSendLatencyLe250.Add(1)
		m.queueSendLatencyLe500.Add(1)
	case seconds <= queueSendLatencyBuckets[1]:
		m.queueSendLatencyLe10.Add(1)
		m.queueSendLatencyLe25.Add(1)
		m.queueSendLatencyLe50.Add(1)
		m.queueSendLatencyLe100.Add(1)
		m.queueSendLatencyLe250.Add(1)
		m.queueSendLatencyLe500.Add(1)
	case seconds <= queueSendLatencyBuckets[2]:
		m.queueSendLatencyLe25.Add(1)
		m.queueSendLatencyLe50.Add(1)
		m.queueSendLatencyLe100.Add(1)
		m.queueSendLatencyLe250.Add(1)
		m.queueSendLatencyLe500.Add(1)
	case seconds <= queueSendLatencyBuckets[3]:
		m.queueSendLatencyLe50.Add(1)
		m.queueSendLatencyLe100.Add(1)
		m.queueSendLatencyLe250.Add(1)
		m.queueSendLatencyLe500.Add(1)
	case seconds <= queueSendLatencyBuckets[4]:
		m.queueSendLatencyLe100.Add(1)
		m.queueSendLatencyLe250.Add(1)
		m.queueSendLatencyLe500.Add(1)
	case seconds <= queueSendLatencyBuckets[5]:
		m.queueSendLatencyLe250.Add(1)
		m.queueSendLatencyLe500.Add(1)
	case seconds <= queueSendLatencyBuckets[6]:
		m.queueSendLatencyLe500.Add(1)
	}
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	droppedBySession := make(map[string]int64)
	m.droppedBySession.Range(func(key, value any) bool {
		sessionID, ok := key.(string)
		if !ok {
			return true
		}
		counter, ok := value.(*atomic.Int64)
		if !ok {
			return true
		}
		droppedBySession[sessionID] = counter.Load()
		return true
	})

	return MetricsSnapshot{
		WSConnections:        m.wsConnections.Load(),
		QueueDepth:           m.queueDepth.Load(),
		QueueSentTotal:       m.queueSentTotal.Load(),
		QueueRejectedTotal:   m.queueRejectedTotal.Load(),
		QueueDroppedTotal:    m.queueDroppedTotal.Load(),
		EventsBroadcastTotal: m.eventsBroadcastTotal.Load(),
		QueueSendLatency: HistogramSnapshot{
			Buckets: []HistogramBucket{
				{Le: strconv.FormatFloat(queueSendLatencyBuckets[0], 'f', -1, 64), Count: m.queueSendLatencyLe05.Load()},
				{Le: strconv.FormatFloat(queueSendLatencyBuckets[1], 'f', -1, 64), Count: m.queueSendLatencyLe10.Load()},
				{Le: strconv.FormatFloat(queueSendLatencyBuckets[2], 'f', -1, 64), Count: m.queueSendLatencyLe25.Load()},
				{Le: strconv.FormatFloat(queueSendLatencyBuckets[3], 'f', -1, 64), Count: m.queueSendLatencyLe50.Load()},
				{Le: strconv.FormatFloat(queueSendLatencyBuckets[4], 'f', -1, 64), Count: m.queueSendLatencyLe100.Load()},
				{Le: strconv.FormatFloat(queueSendLatencyBuckets[5], 'f', -1, 64), Count: m.queueSendLatencyLe250.Load()},
				{Le: strconv.FormatFloat(queueSendLatencyBuckets[6], 'f', -1, 64), Count: m.queueSendLatencyLe500.Load()},
			},
			Sum:   float64(m.queueSendLatencySumNs.Load()) / float64(time.Second),
			Count: m.queueSendLatencyCount.Load(),
		},
		DroppedBySession: droppedBySession,
	}
}
