package gateway

import (
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestTextDecoderDecodeCP1251(t *testing.T) {
	t.Parallel()

	encoder := charmap.Windows1251.NewEncoder()
	source := "\x1b[32mПривет, Арда!\x1b[0m\n"
	encoded, err := encoder.Bytes([]byte(source))
	if err != nil {
		t.Fatalf("encode cp1251: %v", err)
	}

	decoder := NewTextDecoder()
	got := decoder.Decode(encoded)
	if got != source {
		t.Fatalf("decoded text mismatch:\nwant=%q\ngot =%q", source, got)
	}
}

func TestFixSmaugYaArtifact(t *testing.T) {
	t.Parallel()

	input := "землЯ и каплЯ, но Ярость и Я."
	want := "земля и капля, но Ярость и Я."

	got := fixSmaugYaArtifact(input)
	if got != want {
		t.Fatalf("artifact fix mismatch:\nwant=%q\ngot =%q", want, got)
	}
}
