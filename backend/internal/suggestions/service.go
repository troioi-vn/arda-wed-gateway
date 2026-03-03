package suggestions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/state"
)

const defaultRecentLines = 80

type Client interface {
	RequestSuggestion(ctx context.Context, prompt string) (string, error)
}

type SnapshotProvider interface {
	Snapshot() (state.Snapshot, bool, error)
}

type Suggestion struct {
	Commands        []string `json:"commands"`
	Reason          string   `json:"reason"`
	ExpectedOutcome string   `json:"expected_outcome"`
	GeneratedAt     string   `json:"generated_at,omitempty"`
}

type Service struct {
	logger           *slog.Logger
	client           Client
	snapshotProvider SnapshotProvider
	debounce         time.Duration
	recentLinesMax   int

	mu           sync.RWMutex
	recentBySess map[string][]string
	timerBySess  map[string]*time.Timer
	jobSeqBySess map[string]uint64
	latest       Suggestion
	latestFound  bool
}

func NewService(logger *slog.Logger, client Client, snapshots SnapshotProvider, debounce time.Duration, recentLinesMax int) *Service {
	if debounce <= 0 {
		debounce = 700 * time.Millisecond
	}
	if recentLinesMax <= 0 {
		recentLinesMax = defaultRecentLines
	}

	return &Service{
		logger:           logger,
		client:           client,
		snapshotProvider: snapshots,
		debounce:         debounce,
		recentLinesMax:   recentLinesMax,
		recentBySess:     make(map[string][]string),
		timerBySess:      make(map[string]*time.Timer),
		jobSeqBySess:     make(map[string]uint64),
	}
}

func (s *Service) IngestTerminal(sessionID, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	s.mu.Lock()
	lines := appendLogLines(s.recentBySess[sessionID], text)
	s.recentBySess[sessionID] = trimRecent(lines, s.recentLinesMax)

	seq := s.jobSeqBySess[sessionID] + 1
	s.jobSeqBySess[sessionID] = seq

	if timer := s.timerBySess[sessionID]; timer != nil {
		timer.Stop()
	}
	s.timerBySess[sessionID] = time.AfterFunc(s.debounce, func() {
		s.runJob(sessionID, seq)
	})
	s.mu.Unlock()
}

func (s *Service) Latest() (Suggestion, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.latestFound {
		return Suggestion{}, false
	}
	copySuggestion := s.latest
	copySuggestion.Commands = append([]string(nil), s.latest.Commands...)
	return copySuggestion, true
}

func (s *Service) runJob(sessionID string, seq uint64) {
	prompt, err := s.buildPrompt(sessionID)
	if err != nil {
		s.logger.Warn("suggestion prompt build failed", "session_id", sessionID, "job_seq", seq, "error", err.Error())
		return
	}

	response, err := s.client.RequestSuggestion(context.Background(), prompt)
	if err != nil {
		s.logger.Warn("suggestion request failed", "session_id", sessionID, "job_seq", seq, "error", err.Error())
		return
	}

	parsed, err := parseSuggestionJSON(response)
	if err != nil {
		s.logger.Warn("suggestion json parse failed", "session_id", sessionID, "job_seq", seq, "error", err.Error())
		return
	}
	parsed.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobSeqBySess[sessionID] != seq {
		s.logger.Debug("dropping stale suggestion response", "session_id", sessionID, "job_seq", seq, "latest_job_seq", s.jobSeqBySess[sessionID])
		return
	}

	s.latest = parsed
	s.latestFound = true
}

func (s *Service) buildPrompt(sessionID string) (string, error) {
	s.mu.RLock()
	recent := append([]string(nil), s.recentBySess[sessionID]...)
	s.mu.RUnlock()

	snapshot, found, err := s.snapshotProvider.Snapshot()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("You are assisting a MUD player. Return strict JSON only with keys commands, reason, expected_outcome.\n")
	b.WriteString("commands must be an array of short executable in-game commands.\n")
	b.WriteString("Do not use markdown or extra keys.\n\n")

	if found {
		b.WriteString("Current parser state:\n")
		if snapshot.Location != "" {
			b.WriteString("- location: ")
			b.WriteString(snapshot.Location)
			b.WriteByte('\n')
		}
		if snapshot.Prompt != nil {
			b.WriteString(fmt.Sprintf("- prompt: HP %d/%d, MA %d/%d, MV %d/%d, EXP %d\n",
				snapshot.Prompt.HPCurrent,
				snapshot.Prompt.HPMax,
				snapshot.Prompt.MACurrent,
				snapshot.Prompt.MAMax,
				snapshot.Prompt.MVCurrent,
				snapshot.Prompt.MVMax,
				snapshot.Prompt.EXP,
			))
		}
		if len(snapshot.StatusTags) > 0 {
			b.WriteString("- status_tags: ")
			b.WriteString(strings.Join(snapshot.StatusTags, ", "))
			b.WriteByte('\n')
		}
		if len(snapshot.Equipment) > 0 {
			slots := make([]string, 0, len(snapshot.Equipment))
			for slot := range snapshot.Equipment {
				slots = append(slots, slot)
			}
			sort.Strings(slots)
			b.WriteString("- equipment:\n")
			for _, slot := range slots {
				b.WriteString("  - ")
				b.WriteString(slot)
				b.WriteString(": ")
				b.WriteString(snapshot.Equipment[slot])
				b.WriteByte('\n')
			}
		}
		if snapshot.UpdatedAt != "" {
			b.WriteString("- snapshot_updated_at: ")
			b.WriteString(snapshot.UpdatedAt)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	b.WriteString("Recent terminal log (newest at bottom):\n")
	if len(recent) == 0 {
		b.WriteString("(no recent terminal lines)\n")
	} else {
		for _, line := range recent {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	return b.String(), nil
}

func parseSuggestionJSON(raw string) (Suggestion, error) {
	var parsed Suggestion
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return Suggestion{}, fmt.Errorf("decode strict suggestion json: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Suggestion{}, fmt.Errorf("suggestion json must contain a single object")
	}
	if len(parsed.Commands) == 0 {
		return Suggestion{}, fmt.Errorf("commands must contain at least one command")
	}
	if strings.TrimSpace(parsed.Reason) == "" {
		return Suggestion{}, fmt.Errorf("reason is required")
	}
	if strings.TrimSpace(parsed.ExpectedOutcome) == "" {
		return Suggestion{}, fmt.Errorf("expected_outcome is required")
	}
	for i, command := range parsed.Commands {
		parsed.Commands[i] = strings.TrimSpace(command)
		if parsed.Commands[i] == "" {
			return Suggestion{}, fmt.Errorf("commands[%d] is empty", i)
		}
	}
	return parsed, nil
}

func appendLogLines(existing []string, text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	parts := strings.Split(text, "\n")
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line == "" {
			continue
		}
		existing = append(existing, line)
	}
	return existing
}

func trimRecent(lines []string, max int) []string {
	if max <= 0 || len(lines) <= max {
		return lines
	}
	return append([]string(nil), lines[len(lines)-max:]...)
}
