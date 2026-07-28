package migrate

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"
)

// invariant: config/migrations-and-locks:workflow-telemetry-config-migration
func TestRemoveWorkflowResidentsMigration(t *testing.T) {
	const primary = "/repo"
	t.Run("deterministic order and retry after partial removal", func(t *testing.T) {
		removed := []string{}
		calls := 0
		lstat := func(path string) (fs.FileInfo, error) { return fakeDirInfo{}, nil }
		remove := func(path string) error {
			calls++
			if calls == 2 {
				return errors.New("injected removal failure")
			}
			removed = append(removed, path)
			return nil
		}
		var out strings.Builder
		if err := removeWorkflowResidents(primary, &out, lstat, remove); err == nil {
			t.Fatal("partial removal failure was hidden")
		}
		if got, want := removed, []string{"/repo/.awf/metrics"}; !equalStrings(got, want) {
			t.Fatalf("removed=%v want=%v", got, want)
		}
		if err := removeWorkflowResidents(primary, &out, lstat, func(path string) error { removed = append(removed, path); return nil }); err != nil {
			t.Fatal(err)
		}
		if got, want := removed, []string{"/repo/.awf/metrics", "/repo/.awf/metrics", "/repo/.awf/assignments"}; !equalStrings(got, want) {
			t.Fatalf("retry order=%v want=%v", got, want)
		}
	})
	t.Run("absent roots are reported in order", func(t *testing.T) {
		var out strings.Builder
		err := removeWorkflowResidents(primary, &out, func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }, func(string) error { t.Fatal("remove called"); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if got, want := out.String(), "remove-workflow-residents: metrics already absent\nremove-workflow-residents: assignments already absent\n"; got != want {
			t.Fatalf("output=%q want=%q", got, want)
		}
	})
	if err := removeWorkflowResidents(primary, &strings.Builder{}, func(string) (fs.FileInfo, error) { return nil, errors.New("inspect") }, func(string) error { return nil }); err == nil {
		t.Fatal("inspection error accepted")
	}
	for _, tc := range []struct {
		name string
		info fs.FileInfo
	}{{"symlink", fakeLinkInfo{}}, {"file", fakeFileInfo{}}} {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			if err := removeWorkflowResidents(primary, &out, func(string) (fs.FileInfo, error) { return tc.info, nil }, func(string) error { t.Fatal("unsafe root removed"); return nil }); err == nil {
				t.Fatal("unsafe root accepted")
			}
		})
	}
}

type fakeDirInfo struct{}

func (fakeDirInfo) Name() string           { return "root" }
func (fakeDirInfo) Size() int64            { return 0 }
func (fakeDirInfo) Mode() fs.FileMode      { return fs.ModeDir }
func (fakeDirInfo) ModTime() (t time.Time) { return }
func (fakeDirInfo) IsDir() bool            { return true }
func (fakeDirInfo) Sys() any               { return nil }

type fakeLinkInfo struct{ fakeDirInfo }

func (fakeLinkInfo) Mode() fs.FileMode { return fs.ModeSymlink }

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string           { return "root" }
func (fakeFileInfo) Size() int64            { return 0 }
func (fakeFileInfo) Mode() fs.FileMode      { return 0 }
func (fakeFileInfo) ModTime() (t time.Time) { return }
func (fakeFileInfo) IsDir() bool            { return false }
func (fakeFileInfo) Sys() any               { return nil }
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
