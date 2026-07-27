package effort

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// invariant: tooling/effort-management:effort-record-authority
func TestEffortPersistedValidationMatrix(t *testing.T) {
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	valid := persistedRecord{SchemaVersion: 1, ID: idA, Title: "Title", State: StateActive, CreatedAt: now, UpdatedAt: now, Integration: IntegrationNone}
	mutations := map[string]func(*persistedRecord){
		"schema":          func(r *persistedRecord) { r.SchemaVersion = 2 },
		"id-format":       func(r *persistedRecord) { r.ID = "bad" },
		"id-path":         func(r *persistedRecord) { r.ID = idB },
		"blank-title":     func(r *persistedRecord) { r.Title = " " },
		"untrimmed-title": func(r *persistedRecord) { r.Title = " Title" },
		"state":           func(r *persistedRecord) { r.State = "unknown" },
		"created-zero":    func(r *persistedRecord) { r.CreatedAt = time.Time{} },
		"created-non-UTC": func(r *persistedRecord) { r.CreatedAt = now.In(time.FixedZone("local", 3600)) },
		"updated-zero":    func(r *persistedRecord) { r.UpdatedAt = time.Time{} },
		"updated-non-UTC": func(r *persistedRecord) { r.UpdatedAt = now.In(time.FixedZone("local", 3600)) },
		"updated-before":  func(r *persistedRecord) { r.UpdatedAt = now.Add(-time.Second) },
		"integration":     func(r *persistedRecord) { r.Integration = "unknown" },
		"worktree-branch": func(r *persistedRecord) {
			r.Worktree = &Worktree{Branch: "wrong", Base: strings.Repeat("a", 40), AttachedAt: now}
			r.Integration = IntegrationPending
		},
		"worktree-base": func(r *persistedRecord) {
			r.Worktree = &Worktree{Branch: "awf/" + idA, Base: "bad", AttachedAt: now}
			r.Integration = IntegrationPending
		},
		"worktree-time": func(r *persistedRecord) {
			r.Worktree = &Worktree{Branch: "awf/" + idA, Base: strings.Repeat("a", 40)}
			r.Integration = IntegrationPending
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			record := valid
			mutate(&record)
			if err := validatePersisted(record, idA); err == nil {
				t.Fatal("invalid record accepted")
			}
		})
	}
	if got := logical(valid); got.ID != idA || persisted(got).ID != idA {
		t.Fatal("logical/persisted projection changed")
	}
}

func TestEffortStoreRejectsUnsafeEntriesAndJSONTails(t *testing.T) {
	corrupt := &CorruptError{Path: "p", Err: os.ErrInvalid}
	if !strings.Contains(corrupt.Error(), "p") || !errors.Is(corrupt, os.ErrInvalid) {
		t.Fatalf("corrupt error contract = %v", corrupt)
	}
	root := initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if _, err := service.New("List", false); err != nil {
		t.Fatal(err)
	}
	efforts := filepath.Join(root, ".awf", "efforts")
	if err := os.Mkdir(filepath.Join(efforts, "ignored-dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	if records, err := service.List(); err != nil || len(records) != 1 {
		t.Fatalf("directory entry affected list: %#v, %v", records, err)
	}
	writeEffortFile(t, filepath.Join(efforts, "bad.json"), `{}`)
	if _, err := service.List(); err == nil {
		t.Fatal("invalid filename accepted")
	}
	if err := os.Remove(filepath.Join(efforts, "bad.json")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	writeEffortFile(t, outside, `{}`)
	if err := os.Symlink(outside, filepath.Join(efforts, "bad-link")); err == nil {
		if _, err := service.List(); err == nil {
			t.Fatal("symlinked effort entry accepted")
		}
		if err := os.Remove(filepath.Join(efforts, "bad-link")); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(efforts, idA+".json")
	raw, _ := os.ReadFile(path)
	writeEffortFile(t, path, string(raw)+` {}`)
	if _, err := service.Show(idA); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
	if _, err := service.Show("bad"); err == nil {
		t.Fatal("invalid requested ID accepted")
	}
	writeEffortFile(t, path, schemaRecordJSON(time.Now().UTC(), "null", "none"))
	if err := service.store.replace(Record{ID: idA}, false); !errors.Is(err, os.ErrExist) {
		t.Fatalf("exclusive replace = %v", err)
	}
	if err := service.store.replace(Record{ID: idB}, false); err == nil {
		t.Fatal("invalid replacement accepted")
	}
}
