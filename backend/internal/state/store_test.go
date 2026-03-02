package state

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSQLiteStorePersistsMergedStateAcrossDeltas(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	parser := NewParser()
	sessionID := "s-1"
	fixtures := []string{
		"cp1251_plain_room.txt",
		"prompt_hp_ma_mv_exp_variants.txt",
		"aura_state_prefix_variants_ru.txt",
		"equipment_slots_variants_ru.txt",
	}

	for _, fixture := range fixtures {
		delta := parser.Parse(loadFixture(t, fixture))
		if err := store.UpsertDelta(sessionID, delta); err != nil {
			t.Fatalf("upsert delta from %s: %v", fixture, err)
		}
	}

	snapshot, found, err := store.GetLatest()
	if err != nil {
		t.Fatalf("get latest snapshot: %v", err)
	}
	if !found {
		t.Fatalf("expected snapshot to be found")
	}

	if snapshot.SessionID != sessionID {
		t.Fatalf("session id mismatch: want=%q got=%q", sessionID, snapshot.SessionID)
	}
	if snapshot.Location != "Таверна \"Гарцующий пони\"" {
		t.Fatalf("location mismatch: %q", snapshot.Location)
	}

	wantPrompt := &PromptTuple{HPCurrent: 120, HPMax: 150, MACurrent: 88, MAMax: 90, MVCurrent: 60, MVMax: 77, EXP: 4512}
	if !reflect.DeepEqual(snapshot.Prompt, wantPrompt) {
		t.Fatalf("prompt mismatch\nwant: %#v\ngot:  %#v", wantPrompt, snapshot.Prompt)
	}

	wantTags := []string{"Белая Аура", "В полете", "Светится"}
	if !reflect.DeepEqual(snapshot.StatusTags, wantTags) {
		t.Fatalf("status tags mismatch\nwant: %#v\ngot:  %#v", wantTags, snapshot.StatusTags)
	}

	if snapshot.Equipment["head"] != "рогатый шлем" {
		t.Fatalf("equipment head mismatch: %q", snapshot.Equipment["head"])
	}
	if snapshot.Equipment["wield"] != "эльфийский клинок" {
		t.Fatalf("equipment wield mismatch: %q", snapshot.Equipment["wield"])
	}
}
