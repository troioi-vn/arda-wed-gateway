package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

type sentCommand struct {
	command string
	at      time.Time
}

func TestCommandQueueBurstOrderAndRateLimit(t *testing.T) {
	t.Parallel()

	interval := 40 * time.Millisecond
	sent := make(chan sentCommand, 10)

	queue := NewCommandQueue("s-test", 20, interval, func(_ context.Context, command string) error {
		sent <- sentCommand{command: command, at: time.Now()}
		return nil
	}, slog.Default(), NewMetrics())

	queue.Start()
	defer queue.StopAndDrop()

	for i := 0; i < 10; i++ {
		command := fmt.Sprintf("cmd-%d", i)
		if _, err := queue.Enqueue(command); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	results := make([]sentCommand, 0, 10)
	timeout := time.After(2 * time.Second)
	for len(results) < 10 {
		select {
		case cmd := <-sent:
			results = append(results, cmd)
		case <-timeout:
			t.Fatalf("timed out waiting for sent commands, got %d", len(results))
		}
	}

	gotOrder := make([]string, 0, 10)
	wantOrder := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		wantOrder = append(wantOrder, fmt.Sprintf("cmd-%d", i))
		gotOrder = append(gotOrder, results[i].command)
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("commands out of order\nwant: %v\ngot:  %v", wantOrder, gotOrder)
	}

	for i := 1; i < len(results); i++ {
		delta := results[i].at.Sub(results[i-1].at)
		if delta < interval-15*time.Millisecond {
			t.Fatalf("command %d sent too quickly: delta=%s interval=%s", i, delta, interval)
		}
	}
}

func TestCommandQueueRejectsWhenFull(t *testing.T) {
	t.Parallel()

	queue := NewCommandQueue("s-test", 3, time.Second, func(_ context.Context, _ string) error {
		return nil
	}, slog.Default(), NewMetrics())

	for i := 0; i < 3; i++ {
		if _, err := queue.Enqueue("x"); err != nil {
			t.Fatalf("unexpected enqueue error: %v", err)
		}
	}

	depth, err := queue.Enqueue("overflow")
	if err == nil {
		t.Fatalf("expected queue full error")
	}
	if err != ErrQueueFull {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
	if depth != 3 {
		t.Fatalf("expected depth 3, got %d", depth)
	}
}

func TestCommandQueueStopDropsPending(t *testing.T) {
	t.Parallel()

	queue := NewCommandQueue("s-test", 20, time.Hour, func(_ context.Context, _ string) error {
		return nil
	}, slog.Default(), NewMetrics())
	queue.Start()

	for i := 0; i < 5; i++ {
		if _, err := queue.Enqueue("x"); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	dropped := queue.StopAndDrop()
	if dropped != 5 {
		t.Fatalf("expected 5 dropped commands, got %d", dropped)
	}
	if depth := queue.Depth(); depth != 0 {
		t.Fatalf("expected depth 0 after stop, got %d", depth)
	}
}
