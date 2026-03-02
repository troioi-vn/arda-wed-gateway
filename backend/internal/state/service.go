package state

import (
	"fmt"
	"log/slog"
	"sync"
)

type store interface {
	UpsertDelta(sessionID string, delta Delta) error
	GetLatest() (Snapshot, bool, error)
}

type Service struct {
	logger *slog.Logger
	parser *Parser
	store  store
}

func NewService(sqlitePath string, logger *slog.Logger) *Service {
	svc := &Service{
		logger: logger,
		parser: NewParser(),
	}

	sqliteStore, err := NewSQLiteStore(sqlitePath)
	if err != nil {
		logger.Warn("state sqlite init failed, using in-memory state store", "error", err.Error())
		svc.store = newMemoryStore()
		return svc
	}
	svc.store = sqliteStore
	return svc
}

func (s *Service) Ingest(sessionID, text string) error {
	delta := s.parser.Parse(text)
	if delta.Empty() {
		return nil
	}
	if err := s.store.UpsertDelta(sessionID, delta); err != nil {
		return fmt.Errorf("persist parsed state: %w", err)
	}
	return nil
}

func (s *Service) Snapshot() (Snapshot, bool, error) {
	snapshot, found, err := s.store.GetLatest()
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("load latest state snapshot: %w", err)
	}
	return snapshot, found, nil
}

type memoryStore struct {
	mu       sync.RWMutex
	snapshot Snapshot
	found    bool
}

func newMemoryStore() *memoryStore {
	return &memoryStore{}
}

func (m *memoryStore) UpsertDelta(sessionID string, delta Delta) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.found {
		m.snapshot = Snapshot{SessionID: sessionID, Equipment: map[string]string{}}
		m.found = true
	}

	mergeSnapshot(&m.snapshot, delta)
	if sessionID != "" {
		m.snapshot.SessionID = sessionID
	}
	return nil
}

func (m *memoryStore) GetLatest() (Snapshot, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.found {
		return Snapshot{}, false, nil
	}
	copySnapshot := m.snapshot
	if len(m.snapshot.Equipment) > 0 {
		copySnapshot.Equipment = map[string]string{}
		for k, v := range m.snapshot.Equipment {
			copySnapshot.Equipment[k] = v
		}
	}
	if len(m.snapshot.StatusTags) > 0 {
		copySnapshot.StatusTags = append([]string(nil), m.snapshot.StatusTags...)
	}
	if m.snapshot.Prompt != nil {
		prompt := *m.snapshot.Prompt
		copySnapshot.Prompt = &prompt
	}
	return copySnapshot, true, nil
}
