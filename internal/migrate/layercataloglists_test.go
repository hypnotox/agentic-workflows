package migrate

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// invariant: config/migrations-and-locks:list-replacement-fixed-snapshot (TestLayerCatalogListsMigration)
func TestLayerCatalogListsMigration(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{
		"skills/tdd.yaml":           "# retained\ndata:\n    testSurfaces:\n        - {name: Local, kind: unit, location: here}\n    laterCatalogList:\n        - retained\nsections:\n    notes:\n        drop: true\n",
		"skills/proposing-adr.yaml": "data:\n    adrTriggers:\n        - Local trigger\n    adrSections:\n",
		"agents/code-reviewer.yaml": "data:\n    focusItems: []\n",
		"agents/adr-reviewer.yaml":  "# before data\ndata: # data line\n    # before null\n    focusItems: # key line\n        # null foot\n",
		"skills/future.yaml":        "data:\n    laterCatalogList:\n        - retained\n",
	})
	tddPath := filepath.Join(root, ".awf", "skills", "tdd.yaml")
	if err := os.Chmod(tddPath, 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := applyLayerCatalogLists(root, &out); err != nil {
		t.Fatal(err)
	}
	for rel, keys := range map[string][]string{
		"skills/tdd.yaml":           {"testSurfaces"},
		"skills/proposing-adr.yaml": {"adrTriggers", "adrSections"},
		"agents/code-reviewer.yaml": {"focusItems"},
		"agents/adr-reviewer.yaml":  {"focusItems"},
	} {
		got, err := os.ReadFile(filepath.Join(root, ".awf", rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			if !strings.Contains(string(got), key+": false") {
				t.Errorf("%s missing dataDefaults.%s suppression:\n%s", rel, key, got)
			}
		}
		if rel == "skills/proposing-adr.yaml" && strings.Count(string(got), "adrSections:") != 1 {
			t.Errorf("null custom key was retained alongside suppression:\n%s", got)
		}
	}
	tdd, _ := os.ReadFile(tddPath)
	for _, want := range []string{"# retained", "Local", "laterCatalogList:", "retained", "sections:", "drop: true"} {
		if !strings.Contains(string(tdd), want) {
			t.Errorf("tdd sidecar lost %q:\n%s", want, tdd)
		}
	}
	if strings.Contains(string(tdd), "laterCatalogList: false") {
		t.Fatalf("post-cutover key was preemptively suppressed:\n%s", tdd)
	}
	info, err := os.Stat(tddPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	commentAt := strings.Index(string(tdd), "# retained")
	dataAt := strings.Index(string(tdd), "data:")
	sectionsAt := strings.Index(string(tdd), "sections:")
	defaultsAt := strings.Index(string(tdd), "dataDefaults:")
	if commentAt < 0 || dataAt < 0 || sectionsAt < 0 || defaultsAt < 0 || commentAt >= dataAt || dataAt >= sectionsAt || sectionsAt >= defaultsAt {
		t.Fatalf("comment or existing top-level order changed:\n%s", tdd)
	}
	future, _ := os.ReadFile(filepath.Join(root, ".awf", "skills/future.yaml"))
	if strings.Contains(string(future), "dataDefaults") {
		t.Fatalf("future artifact was preemptively suppressed:\n%s", future)
	}
	commentedNull, _ := os.ReadFile(filepath.Join(root, ".awf", "agents/adr-reviewer.yaml"))
	for _, want := range []string{"# before data", "# data line", "# before null", "# key line", "# null foot", "dataDefaults:", "focusItems: false"} {
		if !strings.Contains(string(commentedNull), want) {
			t.Errorf("null-only sidecar lost %q:\n%s", want, commentedNull)
		}
	}
	if strings.Count(string(commentedNull), "focusItems:") != 1 {
		t.Errorf("null custom key remained alongside suppression:\n%s", commentedNull)
	}
	for _, rel := range []string{"skills/tdd.yaml", "skills/proposing-adr.yaml", "agents/code-reviewer.yaml", "agents/adr-reviewer.yaml"} {
		want := "layer-catalog-lists: updated .awf/" + rel
		if strings.Count(out.String(), want) != 1 {
			t.Errorf("mutation output contains %q %d times, want once: %q", want, strings.Count(out.String(), want), out.String())
		}
	}

	before := snapshotTree(t, root)
	out.Reset()
	if err := applyLayerCatalogLists(root, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || !sameSnapshot(before, snapshotTree(t, root)) {
		t.Fatalf("rerun was not a silent byte-identical no-op: %q", out.String())
	}
}

// invariant: config/migrations-and-locks:list-replacement-fixed-snapshot (TestLayerCatalogListSnapshotExact)
func TestLayerCatalogListSnapshotExact(t *testing.T) {
	want := []struct {
		kind, artifact string
		keys           []string
	}{
		{"agents", "adr-reviewer", []string{"focusItems"}},
		{"agents", "code-reviewer", []string{"correctnessTraps", "focusItems", "docCurrencyItems"}},
		{"agents", "plan-reviewer", []string{"focusItems", "docCurrencyItems"}},
		{"agents", "implementer", []string{"prohibitedShortcuts"}},
		{"skills", "adr-lifecycle", []string{"adrStates"}},
		{"skills", "proposing-adr", []string{"adrSections", "adrTriggers"}},
		{"skills", "tdd", []string{"testSurfaces"}},
	}
	if !reflect.DeepEqual(layerCatalogListSnapshot, want) {
		t.Fatalf("frozen snapshot = %#v, want %#v", layerCatalogListSnapshot, want)
	}
	files := map[string]string{}
	for _, artifact := range want {
		var body strings.Builder
		body.WriteString("data:\n")
		for _, key := range artifact.keys {
			body.WriteString("  " + key + ": []\n")
		}
		files[artifact.kind+"/"+artifact.artifact+".yaml"] = body.String()
	}
	root := closeFixture(t, "prefix: ex\n", files)
	var out bytes.Buffer
	if err := applyLayerCatalogLists(root, &out); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range want {
		rel := artifact.kind + "/" + artifact.artifact + ".yaml"
		got, err := os.ReadFile(filepath.Join(root, ".awf", rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range artifact.keys {
			if !strings.Contains(string(got), key+": false") {
				t.Errorf("%s missing exact snapshot key %s:\n%s", rel, key, got)
			}
		}
		announcement := "layer-catalog-lists: updated .awf/" + rel
		if strings.Count(out.String(), announcement) != 1 {
			t.Errorf("announcement %q count = %d, want 1: %q", announcement, strings.Count(out.String(), announcement), out.String())
		}
	}
}

type migrationFaultFile struct {
	reader io.Reader
	info   fs.FileInfo
	stat   error
	close  error
}

func (f migrationFaultFile) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f migrationFaultFile) Stat() (fs.FileInfo, error) { return f.info, f.stat }
func (f migrationFaultFile) Close() error               { return f.close }

type migrationFailedReader struct{ err error }

func (r migrationFailedReader) Read([]byte) (int, error) { return 0, r.err }

func TestLayerCatalogListsReadAndParseErrors(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		root := closeFixture(t, "prefix: ex\n", nil)
		path := filepath.Join(root, ".awf", "skills", "tdd.yaml")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := applyLayerCatalogLists(root, io.Discard); err == nil {
			t.Fatal("expected sidecar read error")
		}
	})
	t.Run("injected read", func(t *testing.T) {
		root := closeFixture(t, "prefix: ex\n", nil)
		injected := errors.New("read failed")
		err := applyLayerCatalogListsWithWriterAndOpen(root, io.Discard, manifest.WriteFileAtomicMode, func(string) (sidecarFile, error) {
			return migrationFaultFile{reader: migrationFailedReader{injected}}, nil
		})
		if !errors.Is(err, injected) || !strings.Contains(err.Error(), "read sidecar") {
			t.Fatalf("read error = %v", err)
		}
	})
	t.Run("open", func(t *testing.T) {
		root := closeFixture(t, "prefix: ex\n", nil)
		injected := errors.New("open failed")
		err := applyLayerCatalogListsWithWriterAndOpen(root, io.Discard, manifest.WriteFileAtomicMode, func(string) (sidecarFile, error) {
			return nil, injected
		})
		if !errors.Is(err, injected) || !strings.Contains(err.Error(), "open sidecar") {
			t.Fatalf("open error = %v", err)
		}
	})
	t.Run("stat", func(t *testing.T) {
		root := closeFixture(t, "prefix: ex\n", nil)
		injected := errors.New("stat failed")
		err := applyLayerCatalogListsWithWriterAndOpen(root, io.Discard, manifest.WriteFileAtomicMode, func(string) (sidecarFile, error) {
			return migrationFaultFile{reader: strings.NewReader(""), stat: injected}, nil
		})
		if !errors.Is(err, injected) || !strings.Contains(err.Error(), "stat sidecar") {
			t.Fatalf("stat error = %v", err)
		}
	})
	t.Run("close", func(t *testing.T) {
		root := closeFixture(t, "prefix: ex\n", nil)
		injected := errors.New("close failed")
		err := applyLayerCatalogListsWithWriterAndOpen(root, io.Discard, manifest.WriteFileAtomicMode, func(string) (sidecarFile, error) {
			return migrationFaultFile{reader: strings.NewReader(""), close: injected}, nil
		})
		if !errors.Is(err, injected) || !strings.Contains(err.Error(), "close sidecar") {
			t.Fatalf("close error = %v", err)
		}
	})
	t.Run("parse", func(t *testing.T) {
		root := closeFixture(t, "prefix: ex\n", map[string]string{"skills/tdd.yaml": "data: [\n"})
		err := applyLayerCatalogLists(root, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "parse sidecar") {
			t.Fatalf("error = %v", err)
		}
	})
}

// invariant: config/migrations-and-locks:list-replacement-fixed-snapshot (TestLayerCatalogListsRefusalPreflightsEverySidecar)
func TestLayerCatalogListsRefusalPreflightsEverySidecar(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{
		"agents/adr-reviewer.yaml":  "data:\n  focusItems:\n    - valid\n",
		"agents/code-reviewer.yaml": "data:\n  focusItems: wrong\n",
	})
	before := snapshotTree(t, root)
	err := applyLayerCatalogLists(root, io.Discard)
	if err == nil {
		t.Fatal("expected refusal")
	}
	for _, want := range []string{"operation:", ".awf/agents/code-reviewer.yaml", "focusItems", "changed bytes: no", "changed index: no", "changed message: no", "changed merge state: no", "set data.focusItems to a list or null", "run `awf upgrade`"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal missing %q: %v", want, err)
		}
	}
	if !sameSnapshot(before, snapshotTree(t, root)) {
		t.Fatal("preflight refusal changed fixture bytes")
	}
}

// invariant: config/migrations-and-locks:list-replacement-fixed-snapshot (TestLayerCatalogListsRetryAfterLaterWriteFailure)
func TestLayerCatalogListsRetryAfterLaterWriteFailure(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{
		"agents/adr-reviewer.yaml":  "data:\n  focusItems: []\n",
		"agents/code-reviewer.yaml": "data:\n  focusItems: []\n",
	})
	writes := 0
	errInjected := errors.New("injected later write failure")
	err := applyLayerCatalogListsWithWriter(root, io.Discard, func(path string, content []byte, mode os.FileMode) error {
		writes++
		if writes == 2 {
			return errInjected
		}
		return manifest.WriteFileAtomicMode(path, content, mode)
	})
	if !errors.Is(err, errInjected) {
		t.Fatalf("first apply error = %v", err)
	}
	if err := applyLayerCatalogLists(root, io.Discard); err != nil {
		t.Fatalf("retry: %v", err)
	}
	for _, rel := range []string{"agents/adr-reviewer.yaml", "agents/code-reviewer.yaml"} {
		got, _ := os.ReadFile(filepath.Join(root, ".awf", rel))
		if !strings.Contains(string(got), "focusItems: false") {
			t.Errorf("retry left %s incomplete:\n%s", rel, got)
		}
	}
}

func TestLayerCatalogListsReportsWriteAndAnnouncementFailures(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{"skills/tdd.yaml": "data:\n  testSurfaces: []\n"})
	writeFailure := errors.New("write failed")
	err := applyLayerCatalogListsWithWriter(root, io.Discard, func(string, []byte, os.FileMode) error { return writeFailure })
	path := filepath.Join(root, ".awf", "skills", "tdd.yaml")
	if !errors.Is(err, writeFailure) || !strings.Contains(err.Error(), "write sidecar "+filepath.ToSlash(path)) {
		t.Fatalf("write failure = %v", err)
	}

	root = closeFixture(t, "prefix: ex\n", map[string]string{"skills/tdd.yaml": "data:\n  testSurfaces: []\n"})
	announcementFailure := errors.New("announcement failed")
	err = applyLayerCatalogLists(root, structuralHeadingFailWriter{err: announcementFailure})
	if !errors.Is(err, announcementFailure) || !strings.Contains(err.Error(), "announce layer-catalog-lists update") {
		t.Fatalf("announcement failure = %v", err)
	}
}

func TestLayerCatalogListsRetainsPrimaryAndCleanupFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reader  io.Reader
		statErr error
		context string
	}{
		{"read", migrationFailedReader{errors.New("read failed")}, nil, "read sidecar"},
		{"stat", strings.NewReader(""), errors.New("stat failed"), "stat sidecar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			primary := tc.statErr
			if primary == nil {
				primary = tc.reader.(migrationFailedReader).err
			}
			closeErr := errors.New("close failed")
			root := closeFixture(t, "prefix: ex\n", nil)
			err := applyLayerCatalogListsWithWriterAndOpen(root, io.Discard, manifest.WriteFileAtomicMode, func(string) (sidecarFile, error) {
				return migrationFaultFile{reader: tc.reader, stat: tc.statErr, close: closeErr}, nil
			})
			if !errors.Is(err, primary) || !errors.Is(err, closeErr) || !strings.Contains(err.Error(), tc.context) || !strings.Contains(err.Error(), "close sidecar") {
				t.Fatalf("joined failures = %v", err)
			}
		})
	}
}

func snapshotTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().IsRegular() {
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			rel, _ := filepath.Rel(root, path)
			out[rel] = b
		}
		return nil
	})
	return out
}

func sameSnapshot(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for path, content := range a {
		if !bytes.Equal(content, b[path]) {
			return false
		}
	}
	return true
}
