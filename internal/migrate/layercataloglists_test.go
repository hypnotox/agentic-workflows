package migrate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	if !strings.Contains(out.String(), "layer-catalog-lists: updated .awf/skills/tdd.yaml") {
		t.Errorf("mutation output = %q", out.String())
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
	t.Run("parse", func(t *testing.T) {
		root := closeFixture(t, "prefix: ex\n", map[string]string{"skills/tdd.yaml": "data: [\n"})
		err := applyLayerCatalogLists(root, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "parse sidecar") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestLayerCatalogListsRefusalPreflightsEverySidecar(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{
		"skills/tdd.yaml":           "data:\n  testSurfaces:\n    - valid\n",
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
