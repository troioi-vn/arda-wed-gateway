package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
)

func TestMetricsExposeLatencyAndDroppedBySession(t *testing.T) {
	t.Parallel()

	upstream := newMetricsFakeUpstream(t)
	defer upstream.Close()

	handler := NewRouter(config.Config{
		Host:              "127.0.0.1",
		Port:              "8080",
		BuildVersion:      "test",
		BuildCommit:       "abc123",
		QueueSendInterval: time.Hour,
		QueueMaxDepth:     20,
	}, slog.Default())

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	baseURL := "http://" + listener.Addr().String()

	status := postJSONMetrics(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if status != http.StatusOK {
		t.Fatalf("connect status: expected 200, got %d", status)
	}

	status = postJSONMetrics(t, baseURL+"/api/v0/commands/enqueue", map[string]any{
		"command": "look",
	})
	if status != http.StatusOK {
		t.Fatalf("enqueue status: expected 200, got %d", status)
	}

	status = postJSONMetrics(t, baseURL+"/api/v0/session/disconnect", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("disconnect status: expected 200, got %d", status)
	}

	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics response: %v", err)
	}
	metrics := string(body)

	if !strings.Contains(metrics, "gateway_queue_send_latency_seconds_bucket") {
		t.Fatalf("expected latency histogram bucket metric")
	}
	if !strings.Contains(metrics, "gateway_queue_send_failed_total") {
		t.Fatalf("expected queue send failed counter metric")
	}
	if !strings.Contains(metrics, `gateway_queue_dropped_unsent_total{session_id="s-1"} 1`) {
		t.Fatalf("expected dropped-unsent metric with session label, got: %s", metrics)
	}
}

func postJSONMetrics(t *testing.T, url string, payload map[string]any) int {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	res, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post request: %v", err)
	}
	defer res.Body.Close()
	return res.StatusCode
}

type metricsFakeUpstream struct {
	listener net.Listener
}

func newMetricsFakeUpstream(t *testing.T) *metricsFakeUpstream {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake upstream: %v", err)
	}

	f := &metricsFakeUpstream{listener: listener}
	go f.run()
	return f
}

func (f *metricsFakeUpstream) run() {
	conn, err := f.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	buffer := make([]byte, 256)
	for {
		if _, err := conn.Read(buffer); err != nil {
			return
		}
	}
}

func (f *metricsFakeUpstream) Port() int {
	_, portStr, _ := net.SplitHostPort(f.listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return port
}

func (f *metricsFakeUpstream) Close() {
	_ = f.listener.Close()
}
