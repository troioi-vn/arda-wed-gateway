package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParserPromptTupleFromFixture(t *testing.T) {
	t.Parallel()

	text := loadFixture(t, "prompt_hp_ma_mv_exp_variants.txt")
	parser := NewParser()
	delta := parser.Parse(text)

	want := &PromptTuple{
		HPCurrent: 120,
		HPMax:     150,
		MACurrent: 88,
		MAMax:     90,
		MVCurrent: 60,
		MVMax:     77,
		EXP:       4512,
	}
	if !reflect.DeepEqual(delta.Prompt, want) {
		t.Fatalf("prompt tuple mismatch\nwant: %#v\ngot:  %#v", want, delta.Prompt)
	}
}

func TestParserStatusTagsFromFixture(t *testing.T) {
	t.Parallel()

	text := loadFixture(t, "aura_state_prefix_variants_ru.txt")
	parser := NewParser()
	delta := parser.Parse(text)

	want := []string{"Белая Аура", "В полете", "Светится"}
	if !reflect.DeepEqual(delta.StatusTags, want) {
		t.Fatalf("status tags mismatch\nwant: %#v\ngot:  %#v", want, delta.StatusTags)
	}
}

func TestParserEquipmentFromFixture(t *testing.T) {
	t.Parallel()

	text := loadFixture(t, "equipment_slots_variants_ru.txt")
	parser := NewParser()
	delta := parser.Parse(text)

	want := map[string]string{
		"head":      "рогатый шлем",
		"neck":      "амулет следопыта",
		"body":      "кольчуга странника",
		"shoulders": "плащ рейнджера",
		"wield":     "эльфийский клинок",
		"shield":    "круглый щит",
		"held":      "гномская лампа",
	}
	if !reflect.DeepEqual(delta.Equipment, want) {
		t.Fatalf("equipment mismatch\nwant: %#v\ngot:  %#v", want, delta.Equipment)
	}
}

func TestParserLocationFromFixture(t *testing.T) {
	t.Parallel()

	text := loadFixture(t, "cp1251_plain_room.txt")
	parser := NewParser()
	delta := parser.Parse(text)

	want := "Таверна \"Гарцующий пони\""
	if delta.Location != want {
		t.Fatalf("location mismatch\nwant: %q\ngot:  %q", want, delta.Location)
	}
}

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "parser", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(content)
}
