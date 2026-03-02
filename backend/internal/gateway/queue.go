package gateway

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var ErrQueueFull = errors.New("command queue is full")

type SenderFunc func(context.Context, string) error

type CommandQueue struct {
	mu       sync.Mutex
	commands []string

	sessionID string
	maxDepth  int
	interval  time.Duration
	sender    SenderFunc
	logger    *slog.Logger
	metrics   *Metrics

	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

func NewCommandQueue(sessionID string, maxDepth int, interval time.Duration, sender SenderFunc, logger *slog.Logger, metrics *Metrics) *CommandQueue {
	return &CommandQueue{
		commands:  make([]string, 0, maxDepth),
		sessionID: sessionID,
		maxDepth:  maxDepth,
		interval:  interval,
		sender:    sender,
		logger:    logger,
		metrics:   metrics,
	}
}

func (q *CommandQueue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.running {
		return
	}

	q.stopCh = make(chan struct{})
	q.doneCh = make(chan struct{})
	q.running = true
	go q.run(q.stopCh, q.doneCh)
}

func (q *CommandQueue) Enqueue(command string) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.commands) >= q.maxDepth {
		q.metrics.IncQueueRejected()
		return len(q.commands), ErrQueueFull
	}

	q.commands = append(q.commands, command)
	depth := len(q.commands)
	q.metrics.SetQueueDepth(depth)
	return depth, nil
}

func (q *CommandQueue) Depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.commands)
}

func (q *CommandQueue) MaxDepth() int {
	return q.maxDepth
}

func (q *CommandQueue) StopAndDrop() int {
	q.mu.Lock()
	dropped := len(q.commands)
	q.commands = q.commands[:0]
	q.metrics.SetQueueDepth(0)

	if !q.running {
		q.mu.Unlock()
		return dropped
	}

	stopCh := q.stopCh
	doneCh := q.doneCh
	q.running = false
	q.stopCh = nil
	q.doneCh = nil
	q.mu.Unlock()

	close(stopCh)
	<-doneCh

	if dropped > 0 {
		q.metrics.AddQueueDropped(dropped)
		q.metrics.AddQueueDroppedForSession(q.sessionID, dropped)
	}

	return dropped
}

func (q *CommandQueue) run(stopCh <-chan struct{}, doneCh chan<- struct{}) {
	defer close(doneCh)

	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			command, ok := q.pop()
			if !ok {
				continue
			}

			start := time.Now()
			if err := q.sender(context.Background(), command); err != nil {
				q.logger.Error("command send failed", "error", err.Error())
				continue
			}

			elapsed := time.Since(start)
			q.metrics.IncQueueSent()
			q.metrics.ObserveQueueSendLatency(elapsed)
			q.logger.Info("command sent", "command", command, "elapsed_ms", elapsed.Milliseconds())
		}
	}
}

func (q *CommandQueue) pop() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.commands) == 0 {
		return "", false
	}

	command := q.commands[0]
	q.commands = q.commands[1:]
	q.metrics.SetQueueDepth(len(q.commands))
	return command, true
}
