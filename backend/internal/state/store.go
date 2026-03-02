package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		path = ":memory:"
	}

	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS state_snapshot (
			session_id TEXT PRIMARY KEY,
			location TEXT,
			hp_current INTEGER,
			hp_max INTEGER,
			ma_current INTEGER,
			ma_max INTEGER,
			mv_current INTEGER,
			mv_max INTEGER,
			exp INTEGER,
			status_tags_json TEXT NOT NULL DEFAULT '[]',
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS state_equipment (
			session_id TEXT NOT NULL,
			slot TEXT NOT NULL,
			item TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (session_id, slot)
		);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) UpsertDelta(sessionID string, delta Delta) error {
	if sessionID == "" || delta.Empty() {
		return nil
	}

	current, found, err := s.GetBySession(sessionID)
	if err != nil {
		return err
	}
	if !found {
		current = Snapshot{SessionID: sessionID, Equipment: map[string]string{}}
	}
	mergeSnapshot(&current, delta)
	current.SessionID = sessionID
	current.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	if err := s.upsertSnapshot(current); err != nil {
		return err
	}
	if err := s.upsertEquipment(current.SessionID, current.Equipment, current.UpdatedAt); err != nil {
		return err
	}
	return nil
}

func mergeSnapshot(snapshot *Snapshot, delta Delta) {
	if delta.Location != "" {
		snapshot.Location = delta.Location
	}
	if delta.Prompt != nil {
		snapshot.Prompt = delta.Prompt
	}
	if len(delta.StatusTags) > 0 {
		snapshot.StatusTags = append([]string(nil), delta.StatusTags...)
	}
	if len(delta.Equipment) > 0 {
		if snapshot.Equipment == nil {
			snapshot.Equipment = map[string]string{}
		}
		for slot, item := range delta.Equipment {
			snapshot.Equipment[slot] = item
		}
	}
}

func (s *SQLiteStore) upsertSnapshot(snapshot Snapshot) error {
	statusJSON, err := json.Marshal(snapshot.StatusTags)
	if err != nil {
		return fmt.Errorf("marshal status tags: %w", err)
	}

	var hpCurrent, hpMax, maCurrent, maMax, mvCurrent, mvMax, exp any
	if snapshot.Prompt != nil {
		hpCurrent = snapshot.Prompt.HPCurrent
		hpMax = snapshot.Prompt.HPMax
		maCurrent = snapshot.Prompt.MACurrent
		maMax = snapshot.Prompt.MAMax
		mvCurrent = snapshot.Prompt.MVCurrent
		mvMax = snapshot.Prompt.MVMax
		exp = snapshot.Prompt.EXP
	}

	_, err = s.db.Exec(`
		INSERT INTO state_snapshot (
			session_id, location, hp_current, hp_max, ma_current, ma_max, mv_current, mv_max, exp, status_tags_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			location = excluded.location,
			hp_current = excluded.hp_current,
			hp_max = excluded.hp_max,
			ma_current = excluded.ma_current,
			ma_max = excluded.ma_max,
			mv_current = excluded.mv_current,
			mv_max = excluded.mv_max,
			exp = excluded.exp,
			status_tags_json = excluded.status_tags_json,
			updated_at = excluded.updated_at;
	`, snapshot.SessionID, snapshot.Location, hpCurrent, hpMax, maCurrent, maMax, mvCurrent, mvMax, exp, string(statusJSON), snapshot.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert snapshot: %w", err)
	}
	return nil
}

func (s *SQLiteStore) upsertEquipment(sessionID string, equipment map[string]string, updatedAt string) error {
	if len(equipment) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin equipment tx: %w", err)
	}
	defer tx.Rollback()

	for slot, item := range equipment {
		if _, err := tx.Exec(`
			INSERT INTO state_equipment (session_id, slot, item, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(session_id, slot) DO UPDATE SET item = excluded.item, updated_at = excluded.updated_at;
		`, sessionID, slot, item, updatedAt); err != nil {
			return fmt.Errorf("upsert equipment: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit equipment tx: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetLatest() (Snapshot, bool, error) {
	row := s.db.QueryRow(`SELECT session_id FROM state_snapshot ORDER BY updated_at DESC LIMIT 1`)
	var sessionID string
	if err := row.Scan(&sessionID); err != nil {
		if err == sql.ErrNoRows {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, fmt.Errorf("query latest state session: %w", err)
	}
	return s.GetBySession(sessionID)
}

func (s *SQLiteStore) GetBySession(sessionID string) (Snapshot, bool, error) {
	row := s.db.QueryRow(`
		SELECT session_id, location, hp_current, hp_max, ma_current, ma_max, mv_current, mv_max, exp, status_tags_json, updated_at
		FROM state_snapshot
		WHERE session_id = ?
	`, sessionID)

	var snap Snapshot
	var statusJSON string
	var hpCurrent, hpMax, maCurrent, maMax, mvCurrent, mvMax, exp sql.NullInt64
	if err := row.Scan(&snap.SessionID, &snap.Location, &hpCurrent, &hpMax, &maCurrent, &maMax, &mvCurrent, &mvMax, &exp, &statusJSON, &snap.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, fmt.Errorf("query snapshot: %w", err)
	}

	if statusJSON != "" {
		if err := json.Unmarshal([]byte(statusJSON), &snap.StatusTags); err != nil {
			return Snapshot{}, false, fmt.Errorf("unmarshal status tags: %w", err)
		}
	}

	if hpCurrent.Valid && hpMax.Valid && maCurrent.Valid && maMax.Valid && mvCurrent.Valid && mvMax.Valid && exp.Valid {
		snap.Prompt = &PromptTuple{
			HPCurrent: int(hpCurrent.Int64),
			HPMax:     int(hpMax.Int64),
			MACurrent: int(maCurrent.Int64),
			MAMax:     int(maMax.Int64),
			MVCurrent: int(mvCurrent.Int64),
			MVMax:     int(mvMax.Int64),
			EXP:       int(exp.Int64),
		}
	}

	equipment, err := s.loadEquipment(snap.SessionID)
	if err != nil {
		return Snapshot{}, false, err
	}
	snap.Equipment = equipment

	return snap, true, nil
}

func (s *SQLiteStore) loadEquipment(sessionID string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT slot, item FROM state_equipment WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query equipment: %w", err)
	}
	defer rows.Close()

	equipment := map[string]string{}
	for rows.Next() {
		var slot, item string
		if err := rows.Scan(&slot, &item); err != nil {
			return nil, fmt.Errorf("scan equipment: %w", err)
		}
		equipment[slot] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate equipment: %w", err)
	}
	if len(equipment) == 0 {
		return nil, nil
	}

	// Keep deterministic map order for test comparisons by rebuilding in slot order.
	slots := make([]string, 0, len(equipment))
	for slot := range equipment {
		slots = append(slots, slot)
	}
	sort.Strings(slots)
	ordered := make(map[string]string, len(equipment))
	for _, slot := range slots {
		ordered[slot] = equipment[slot]
	}
	return ordered, nil
}
