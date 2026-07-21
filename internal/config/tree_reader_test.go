package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type memoryTreeReader map[string][]byte

func (r memoryTreeReader) ReadFile(path string) ([]byte, bool) {
	b, ok := r[path]
	return append([]byte(nil), b...), ok
}
func (r memoryTreeReader) Paths(prefix string) []string { return []string{"x"} }

func TestParseTreeReaderSidecarsAndParts(t *testing.T) {
	r := memoryTreeReader{"skills/s.yaml": []byte("local: true\n"), "skills/parts/s/content.md": []byte("part"), "parts/agents-doc/commands.md": []byte("commands")}
	cfg, err := ParseTree(".awf", []byte("prefix: x\nskills: [s]\n"), r)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := cfg.Sidecar("skills", "s")
	if err != nil || !sc.Local {
		t.Fatalf("sidecar=%#v err=%v", sc, err)
	}
	if _, err := cfg.Sidecar("skills", "missing"); err != nil {
		t.Fatal(err)
	}
	b, ok := cfg.ReadSidecar("skills/s.yaml")
	if !ok || string(b) != "local: true\n" {
		t.Fatal("read sidecar")
	}
	b[0] = 'X'
	again, _ := cfg.ReadSidecar("skills/s.yaml")
	if string(again) != "local: true\n" {
		t.Fatal("aliased")
	}
	part, ok, err := cfg.ReadPart("skills", "s", "content")
	if err != nil || !ok || string(part) != "part" {
		t.Fatalf("part=%q %v", part, ok)
	}
	if part, ok, err = cfg.ReadPart("agents-doc", "", "commands"); err != nil || !ok || string(part) != "commands" {
		t.Fatalf("singleton part=%q %v", part, ok)
	}
	if _, ok, err := cfg.ReadPart("skills", "s", "missing"); err != nil || ok {
		t.Fatal("missing part")
	}
	if b, err := cfg.ReadPartPath(".awf/skills/parts/s/content.md"); err != nil || string(b) != "part" {
		t.Fatalf("part path=%q %v", b, err)
	}
	if _, err := cfg.ReadPartPath(".awf/missing"); err == nil {
		t.Fatal("missing part path")
	}
	if got := cfg.read.Paths(""); !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("paths=%v", got)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fr := filesystemTreeReader{root: root}
	if _, ok := fr.ReadFile("x"); !ok {
		t.Fatal("filesystem reader")
	}
	if _, ok := fr.ReadFile("missing"); ok {
		t.Fatal("missing filesystem reader")
	}
	if got := fr.Paths(""); len(got) != 0 {
		t.Fatalf("filesystem config paths=%v", got)
	}
	var zero Config
	if _, ok := zero.ReadSidecar("x"); ok {
		t.Fatal("nil reader")
	}
	if _, ok, err := zero.ReadPart("x", "x", "x"); err != nil || ok {
		t.Fatal("nil part reader")
	}
}
