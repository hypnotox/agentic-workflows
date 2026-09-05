package effortfs

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLifecycle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	gotEmpty, err := List(root)
	if err != nil {
		t.Fatalf("List() before New() error = %v", err)
	}
	if len(gotEmpty) != 0 {
		t.Fatalf("List() before New() = %v, want empty", gotEmpty)
	}

	memoryPath, err := New(root, "ship-it")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	wantMemoryPath := filepath.Join(".awf", "efforts", "ship-it", "memory.md")
	if memoryPath != wantMemoryPath {
		t.Fatalf("New() path = %q, want %q", memoryPath, wantMemoryPath)
	}

	showPath, body, err := Show(root, "ship-it")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if showPath != wantMemoryPath {
		t.Fatalf("Show() path = %q, want %q", showPath, wantMemoryPath)
	}
	wantBody := "# Effort: ship-it\n\n" +
		"## Outcome and success criteria\n\n" +
		"Define the outcome and criteria here, or point to the detailed criteria in `plan.md`.\n\n" +
		"## Current state\n\n" +
		"## Decisions and evidence\n\n" +
		"Record selected decision-relevant excerpts with attribution. Label quotations, summaries, proposals, and agreed decisions accurately. Reference an ADR instead of duplicating its content.\n\n" +
		"## Artifacts\n\n" +
		"Reference the plan, ADRs, and other effort artifacts here.\n\n" +
		"## Next actions\n\n" +
		"## Completion evidence\n\n" +
		"Compare the actual result with the success criteria. Record verification, unmet criteria, deviations, and required topic updates before finishing.\n"
	if string(body) != wantBody {
		t.Fatalf("Show() body = %q, want %q", body, wantBody)
	}

	slugs, err := List(root)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !reflect.DeepEqual(slugs, []string{"ship-it"}) {
		t.Fatalf("List() = %v, want [ship-it]", slugs)
	}

	archivePath, err := Finish(root, "ship-it")
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	wantArchivePath := filepath.Join(".awf", "effort-archive", "ship-it")
	if archivePath != wantArchivePath {
		t.Fatalf("Finish() path = %q, want %q", archivePath, wantArchivePath)
	}
	if _, err := os.Stat(filepath.Join(root, wantMemoryPath)); !os.IsNotExist(err) {
		t.Fatalf("active memory after Finish() Stat error = %v, want not exist", err)
	}
	archivedBody, err := os.ReadFile(filepath.Join(root, wantArchivePath, "memory.md"))
	if err != nil {
		t.Fatalf("read archived memory: %v", err)
	}
	if string(archivedBody) != wantBody {
		t.Fatalf("archived body = %q, want %q", archivedBody, wantBody)
	}
}

func TestListReturnsSortedEffortsAndIgnoresUnrelatedEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, slug := range []string{"zeta", "alpha", "middle"} {
		if _, err := New(root, slug); err != nil {
			t.Fatalf("New(%q) error = %v", slug, err)
		}
	}

	activeRoot := filepath.Join(root, ".awf", "efforts")
	if err := os.WriteFile(filepath.Join(activeRoot, "plain-file"), []byte("unrelated"), 0o644); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(activeRoot, "no-memory"), 0o755); err != nil {
		t.Fatalf("create unrelated directory: %v", err)
	}
	memoryDirectory := filepath.Join(activeRoot, "non-regular-memory", "memory.md")
	if err := os.MkdirAll(memoryDirectory, 0o755); err != nil {
		t.Fatalf("create non-regular memory entry: %v", err)
	}

	got, err := List(root)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []string{"alpha", "middle", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %v, want %v", got, want)
	}
}

func TestUnsafeSlug(t *testing.T) {
	t.Parallel()

	for _, slug := range []string{"", ".", "..", "-leading", "bad slug", "bad\nslug", "nested/effort", `nested\effort`} {
		slug := slug
		t.Run(slug, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if _, err := New(root, slug); err == nil || !strings.Contains(err.Error(), "invalid effort slug") {
				t.Fatalf("New(%q) error = %v, want unsafe slug error", slug, err)
			}
			if _, _, err := Show(root, slug); err == nil || !strings.Contains(err.Error(), "invalid effort slug") {
				t.Fatalf("Show(%q) error = %v, want unsafe slug error", slug, err)
			}
			if _, err := Finish(root, slug); err == nil || !strings.Contains(err.Error(), "invalid effort slug") {
				t.Fatalf("Finish(%q) error = %v, want unsafe slug error", slug, err)
			}
			if _, err := NewPlan(root, slug); err == nil || !strings.Contains(err.Error(), "invalid effort slug") {
				t.Fatalf("NewPlan(%q) error = %v, want unsafe slug error", slug, err)
			}
		})
	}
}

func TestNewPlanRequiresEffortAndPreservesExistingPlan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := NewPlan(root, "missing"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("NewPlan() missing effort error = %v", err)
	}
	if _, err := New(root, "planned"); err != nil {
		t.Fatal(err)
	}
	planRelative, err := NewPlan(root, "planned")
	if err != nil {
		t.Fatal(err)
	}
	wantRelative := filepath.Join(".awf", "efforts", "planned", "plan.md")
	if planRelative != wantRelative {
		t.Fatalf("NewPlan() path = %q, want %q", planRelative, wantRelative)
	}
	body, err := os.ReadFile(filepath.Join(root, planRelative))
	if err != nil {
		t.Fatal(err)
	}
	for _, heading := range []string{"# Plan: planned", "## Outcome and success criteria", "## Work sequence", "## Verification"} {
		if !strings.Contains(string(body), heading) {
			t.Errorf("plan missing %q", heading)
		}
	}

	edited := []byte("author plan\n")
	if err := os.WriteFile(filepath.Join(root, planRelative), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPlan(root, "planned"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("repeat NewPlan() error = %v", err)
	}
	preserved, err := os.ReadFile(filepath.Join(root, planRelative))
	if err != nil || !bytes.Equal(preserved, edited) {
		t.Fatalf("existing plan = %q, %v", preserved, err)
	}

	archiveRelative, err := Finish(root, "planned")
	if err != nil {
		t.Fatal(err)
	}
	archived, err := os.ReadFile(filepath.Join(root, archiveRelative, "plan.md"))
	if err != nil || !bytes.Equal(archived, edited) {
		t.Fatalf("archived plan = %q, %v", archived, err)
	}
}

func TestArchiveCollision(t *testing.T) {
	t.Parallel()

	t.Run("finish preserves active and archive", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		if _, err := New(root, "collision"); err != nil {
			t.Fatalf("New() error = %v", err)
		}
		archiveDirectory := filepath.Join(root, ".awf", "effort-archive", "collision")
		if err := os.MkdirAll(archiveDirectory, 0o755); err != nil {
			t.Fatalf("create archive collision: %v", err)
		}
		marker := filepath.Join(archiveDirectory, "existing.txt")
		if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
			t.Fatalf("write archive marker: %v", err)
		}

		if _, err := Finish(root, "collision"); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("Finish() error = %v, want archive collision", err)
		}
		if _, err := os.Stat(filepath.Join(root, ".awf", "efforts", "collision", "memory.md")); err != nil {
			t.Fatalf("active effort changed after collision: %v", err)
		}
		body, err := os.ReadFile(marker)
		if err != nil {
			t.Fatalf("read archive marker: %v", err)
		}
		if string(body) != "keep" {
			t.Fatalf("archive marker = %q, want keep", body)
		}
	})

	t.Run("new refuses archived slug", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		archiveDirectory := filepath.Join(root, ".awf", "effort-archive", "old")
		if err := os.MkdirAll(archiveDirectory, 0o755); err != nil {
			t.Fatalf("create archived effort: %v", err)
		}
		if _, err := New(root, "old"); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("New() error = %v, want archive collision", err)
		}
		if _, err := os.Stat(filepath.Join(root, ".awf", "efforts", "old")); !os.IsNotExist(err) {
			t.Fatalf("active path after refused New() Stat error = %v, want not exist", err)
		}
	})
}

func TestRawMemoryAndExtraFilesSurviveFinish(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	memoryRelative, err := New(root, "raw")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rawBody := []byte{'n', 'o', 't', ' ', 'm', 'a', 'r', 'k', 'd', 'o', 'w', 'n', 0, 0xff, '\n'}
	if err := os.WriteFile(filepath.Join(root, memoryRelative), rawBody, 0o644); err != nil {
		t.Fatalf("replace memory body: %v", err)
	}
	extraRelative := filepath.Join("scratch", "notes.bin")
	extraBody := []byte{0x00, 0x01, 0xfe, 0xff}
	extraPath := filepath.Join(root, ".awf", "efforts", "raw", extraRelative)
	if err := os.MkdirAll(filepath.Dir(extraPath), 0o755); err != nil {
		t.Fatalf("create extra-file directory: %v", err)
	}
	if err := os.WriteFile(extraPath, extraBody, 0o644); err != nil {
		t.Fatalf("write extra file: %v", err)
	}

	showPath, shownBody, err := Show(root, "raw")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if showPath != memoryRelative {
		t.Fatalf("Show() path = %q, want %q", showPath, memoryRelative)
	}
	if !bytes.Equal(shownBody, rawBody) {
		t.Fatalf("Show() body = %v, want %v", shownBody, rawBody)
	}

	archiveRelative, err := Finish(root, "raw")
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	archivedMemory, err := os.ReadFile(filepath.Join(root, archiveRelative, "memory.md"))
	if err != nil {
		t.Fatalf("read archived memory: %v", err)
	}
	if !bytes.Equal(archivedMemory, rawBody) {
		t.Fatalf("archived memory = %v, want %v", archivedMemory, rawBody)
	}
	archivedExtra, err := os.ReadFile(filepath.Join(root, archiveRelative, extraRelative))
	if err != nil {
		t.Fatalf("read archived extra file: %v", err)
	}
	if !bytes.Equal(archivedExtra, extraBody) {
		t.Fatalf("archived extra = %v, want %v", archivedExtra, extraBody)
	}
}
