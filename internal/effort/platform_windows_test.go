//go:build windows

package effort

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

type fakeWindowsInfo struct{ id string }

func (f fakeWindowsInfo) Name() string     { return f.id }
func (fakeWindowsInfo) Size() int64        { return 0 }
func (fakeWindowsInfo) Mode() fs.FileMode  { return 0 }
func (fakeWindowsInfo) ModTime() time.Time { return time.Time{} }
func (fakeWindowsInfo) IsDir() bool        { return false }
func (fakeWindowsInfo) Sys() any           { return nil }

type fakeWindowsFile struct {
	id    string
	bytes string
}

type fakeWindowsPublication struct {
	files     map[string]fakeWindowsFile
	events    []string
	flushFail error
}

func (f *fakeWindowsPublication) api(t *testing.T) windowsPublicationAPI {
	t.Helper()
	return windowsPublicationAPI{
		move: func(from, to string, flags uint32) error {
			f.events = append(f.events, "move:"+from+":"+to)
			if flags != windows.MOVEFILE_WRITE_THROUGH {
				t.Fatalf("MoveFileEx flags = %#x, want MOVEFILE_WRITE_THROUGH", flags)
			}
			value, ok := f.files[from]
			if !ok {
				return os.ErrNotExist
			}
			if _, exists := f.files[to]; exists {
				return os.ErrExist
			}
			f.files[to] = value
			delete(f.files, from)
			return nil
		},
		replace: func(path, replacement, backup string, flags uint32) error {
			f.events = append(f.events, "replace:"+path+":"+replacement+":"+backup)
			if flags != 0 {
				t.Fatalf("ReplaceFileW flags = %#x, want 0", flags)
			}
			old, oldOK := f.files[path]
			newValue, newOK := f.files[replacement]
			if !oldOK || !newOK {
				return os.ErrNotExist
			}
			f.files[backup] = old
			f.files[path] = newValue
			delete(f.files, replacement)
			return nil
		},
		inspect: func(path string) (fileIdentity, error) {
			value, ok := f.files[path]
			if !ok {
				return fileIdentity{}, os.ErrNotExist
			}
			return fileIdentity{info: fakeWindowsInfo{id: value.id}}, nil
		},
		same: func(left, right fileIdentity) bool { return left.info.Name() == right.info.Name() },
		flush: func(path string, expected fileIdentity) error {
			f.events = append(f.events, "flush:"+path)
			value, ok := f.files[path]
			if !ok || value.id != expected.info.Name() {
				return errors.New("flush identity mismatch")
			}
			return f.flushFail
		},
	}
}

func TestWindowsPublicationAPIFaultMatrix(t *testing.T) {
	t.Run("expected replacement", func(t *testing.T) {
		fake := &fakeWindowsPublication{files: map[string]fakeWindowsFile{
			"temp": {id: "new", bytes: "new"}, "path": {id: "old", bytes: "old"},
		}}
		expected := fileIdentity{info: fakeWindowsInfo{id: "old"}}
		if err := publishAtomicWindows("temp", "path", &expected, fake.api(t)); err != nil {
			t.Fatal(err)
		}
		if got := fake.files["path"].bytes; got != "new" {
			t.Fatalf("replacement bytes = %q", got)
		}
	})

	t.Run("raced replacement restores unexpected bytes", func(t *testing.T) {
		fake := &fakeWindowsPublication{files: map[string]fakeWindowsFile{
			"temp": {id: "new", bytes: "new"}, "path": {id: "raced", bytes: "raced"},
		}}
		expected := fileIdentity{info: fakeWindowsInfo{id: "old"}}
		err := publishAtomicWindows("temp", "path", &expected, fake.api(t))
		if err == nil || !strings.Contains(err.Error(), "identity") {
			t.Fatalf("raced replacement error = %v", err)
		}
		if got := fake.files["path"].bytes; got != "raced" {
			t.Fatalf("restored raced bytes = %q", got)
		}
	})

	t.Run("flush failure reports published bytes", func(t *testing.T) {
		flushErr := errors.New("injected FlushFileBuffers failure")
		fake := &fakeWindowsPublication{files: map[string]fakeWindowsFile{
			"temp": {id: "new", bytes: "new"}, "path": {id: "old", bytes: "old"},
		}, flushFail: flushErr}
		expected := fileIdentity{info: fakeWindowsInfo{id: "old"}}
		err := publishAtomicWindows("temp", "path", &expected, fake.api(t))
		if !errors.Is(err, flushErr) {
			t.Fatalf("flush failure = %v", err)
		}
		if got := fake.files["path"].bytes; got != "new" {
			t.Fatalf("bytes after flush failure = %q", got)
		}
	})
}

func TestWindowsFinishReportsUnavailableParentSync(t *testing.T) {
	if directorySyncAvailable() {
		t.Fatal("Windows directory sync reported available")
	}
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.Fault = func(stage string) error {
			if strings.HasSuffix(stage, "fsync") {
				return errors.New("unavailable sync stage was invoked")
			}
			return nil
		}
	})
	record, err := service.New(testContext(t), NewInput{Slug: "windows-sync", Title: "Windows sync"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Finish(testContext(t), record.Slug)
	if err != nil || !result.Archived || result.DestinationSyncAvailable || result.SourceSyncAvailable || result.DestinationSynced || result.SourceSynced {
		t.Fatalf("finish result=%#v err=%v", result, err)
	}
	mutation, err := result.FinishMutation(record.Slug)
	if err != nil || len(mutation.Changes) != 2 || mutation.Changes[1].Label != "platform limits" || len(mutation.Changes[1].Values) != 2 {
		t.Fatalf("finish mutation=%#v err=%v", mutation, err)
	}
}

func TestWindowsOwnershipAPIRejectsForeignOwnerAndFaults(t *testing.T) {
	api := windowsSecurityAPI{
		ownerSID:   func(windows.Handle) (string, error) { return "S-1-owner", nil },
		currentSID: func() (string, error) { return "S-1-current", nil },
	}
	var hard interface{ Forceable() bool }
	err := validateWindowsOwner("resident", 1, api)
	if !errors.As(err, &hard) || hard.Forceable() || !strings.Contains(err.Error(), "foreign-owner") {
		t.Fatalf("foreign owner refusal = %v", err)
	}
	api.ownerSID = func(windows.Handle) (string, error) { return "", errors.New("descriptor fault") }
	if err := validateWindowsOwner("resident", 1, api); err == nil || !strings.Contains(err.Error(), "descriptor fault") {
		t.Fatalf("descriptor fault = %v", err)
	}
}
