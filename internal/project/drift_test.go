package project

import (
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// configHashOf re-opens the project and returns the per-target ConfigHash of the
// rendered file at rel.
func configHashOf(t *testing.T, root, rel string) string {
	t.Helper()
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	for _, f := range files {
		if f.Path == rel {
			return f.ConfigHash
		}
	}
	t.Fatalf("no rendered file %s", rel)
	return ""
}

// invariant: drift-source-set
func TestPerTargetDriftProjection(t *testing.T) {
	const (
		A = ".claude/skills/example-tdd/SKILL.md"
		B = ".claude/skills/example-bugfix/SKILL.md"
	)
	cfg := func(pitfalls string) string {
		return "prefix: example\n" + sprintfVars(pitfalls) + "skills:\n  - tdd\n  - bugfix\nagents: []\n"
	}
	root := scaffoldFiles(t, cfg(""), map[string]string{
		"skills/tdd.yaml":           "data:\n  testSurfaces:\n    - {name: One, location: a, kind: b}\n",
		"skills/bugfix.yaml":        "data:\n  k: v\n",
		"skills/parts/tdd/notes.md": "ORIGINAL NOTES\n",
	})

	a0 := configHashOf(t, root, A)
	b0 := configHashOf(t, root, B)

	// (1) Editing target A's sidecar changes A's hash but not B's.
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/tdd.yaml"), "data:\n  testSurfaces:\n    - {name: Changed, location: x, kind: y}\n")
	a1 := configHashOf(t, root, A)
	b1 := configHashOf(t, root, B)
	if a1 == a0 {
		t.Error("editing A's sidecar should change A's ConfigHash")
	}
	if b1 != b0 {
		t.Error("editing A's sidecar must not change B's ConfigHash")
	}

	// (2) Editing a part A consumes changes A.
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/parts/tdd/notes.md"), "NEW NOTES BODY\n")
	a2 := configHashOf(t, root, A)
	if a2 == a1 {
		t.Error("editing a part A consumes should change A's ConfigHash")
	}

	// (3) An unrelated vars edit (a var A does not reference) does not change A's hash.
	testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), cfg("now-set"))
	a3 := configHashOf(t, root, A)
	if a3 != a2 {
		t.Errorf("a var A does not reference (pitfallsDoc) must not change A's ConfigHash:\n%s\n%s", a2, a3)
	}

	// (4) A sidecar/part for a target absent from the enable list is an orphan.
	orphRoot := scaffoldFiles(t, cfg(""), map[string]string{
		"skills/tdd.yaml":                 "data:\n  k: v\n",
		"skills/bugfix.yaml":              "data:\n  k: v\n",
		"skills/debugging.yaml":           "data:\n  k: v\n", // debugging not enabled
		"skills/parts/orphan-target/x.md": "stray\n",         // orphan-target not enabled
	})
	p, err := Open(orphRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	wantOrphans := map[string]bool{
		".awf/skills/debugging.yaml":      false,
		".awf/skills/parts/orphan-target": false,
	}
	for _, d := range drift {
		if d.Kind == "orphaned" {
			if _, ok := wantOrphans[d.Path]; ok {
				wantOrphans[d.Path] = true
			}
		}
	}
	for path, seen := range wantOrphans {
		if !seen {
			t.Errorf("expected orphan drift for %s, got %#v", path, drift)
		}
	}
}

func sprintfVars(pitfalls string) string {
	return "vars:\n  testCmd: \"\"\n  gateCmd: \"\"\n  gateCmdFull: \"\"\n  workflowDoc: \"\"\n  docCurrencyTargets: \"\"\n  pitfallsDoc: \"" + pitfalls + "\"\n"
}

// invariant: schema-version-lock
func TestSyncStampsSchemaVersion(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != migrate.Current() {
		t.Errorf("lock SchemaVersion = %d, want %d (current schema)", lock.SchemaVersion, migrate.Current())
	}
	if lock.AWFVersion != Version {
		t.Errorf("AWFVersion = %q, want %q (independent tool version)", lock.AWFVersion, Version)
	}
}
