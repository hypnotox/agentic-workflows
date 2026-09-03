package catalog

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
	"gopkg.in/yaml.v3"
)

// invariant: rendering/catalog-and-targets:catalog-go-single-source (TestCatalogIsCompileTimeSingleSource)
func TestCatalogIsCompileTimeSingleSource(t *testing.T) {
	if _, err := fs.Stat(templates.FS, "catalog.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("catalog.yaml must not be embedded; got stat err = %v", err)
	}
	if len(Standard.Skills) == 0 || len(Standard.Docs) == 0 ||
		len(SingletonKinds()) == 0 || len(Standard.Vars) == 0 || len(Standard.DomainDoc.Sections) == 0 {
		t.Fatalf("catalog.Standard is not populated across all kinds")
	}
}

func TestNewViewRejectsNilCatalog(t *testing.T) {
	defer func() {
		if got := recover(); got != "catalog view: missing catalog" {
			t.Fatalf("panic = %v", got)
		}
	}()
	NewView(nil)
}

// TestCompleteViewPreservesStandard verifies the preparatory view selects every
// complete-catalog entry and preserves the ordered descriptor population.
func TestCompleteViewPreservesStandard(t *testing.T) {
	view := CompleteView()
	if view.Catalog() == Standard {
		t.Fatal("complete view retained a mutable alias to Standard")
	}
	if !reflect.DeepEqual(view.Catalog(), Standard) {
		t.Fatal("complete view does not preserve the complete Standard catalog")
	}
}

func TestViewOwnsDeepCatalogSnapshot(t *testing.T) {
	injected := cloneCatalog(Standard)
	injectedSkill := injected.Skills["awf-effort"]
	injectedSkill.Data = map[string]any{}
	injectedSkill.Data["strings"] = []string{"original"}
	injectedSkill.Data["numbers"] = []int{1}
	injectedSkill.Data["labels"] = map[string]string{"value": "original"}
	injectedSkill.Data["records"] = []map[string]any{{"value": "original"}}
	injectedSkill.Data["array"] = [1]string{"original"}
	injectedSkill.Data["direct-nil"] = nil
	injectedSkill.Data["nil-list"] = []any{nil}
	var nilMap map[string]string
	injectedSkill.Data["nil-map"] = nilMap
	var nilSlice []int
	injectedSkill.Data["nil-slice"] = nilSlice
	pointed := []string{"original"}
	injectedSkill.Data["pointer"] = &pointed
	var nilPointer *[]string
	injectedSkill.Data["nil-pointer"] = nilPointer
	injected.Skills["awf-effort"] = injectedSkill
	view := NewView(injected)

	injectedSkill.Sections[0] = "changed input"
	injectedSkill.Data["strings"].([]string)[0] = "changed input"
	injectedSkill.Data["numbers"].([]int)[0] = 2
	injectedSkill.Data["labels"].(map[string]string)["value"] = "changed input"
	injectedSkill.Data["records"].([]map[string]any)[0]["value"] = "changed input"
	pointed[0] = "changed input"
	injected.Skills["awf-effort"] = injectedSkill

	got := view.Catalog().Skills["awf-effort"]
	gotNilMap, mapOK := got.Data["nil-map"].(map[string]string)
	gotNilSlice, sliceOK := got.Data["nil-slice"].([]int)
	if got.Sections[0] == "changed input" || got.Data["strings"].([]string)[0] != "original" ||
		got.Data["numbers"].([]int)[0] != 1 || got.Data["labels"].(map[string]string)["value"] != "original" ||
		got.Data["records"].([]map[string]any)[0]["value"] != "original" || got.Data["array"].([1]string)[0] != "original" ||
		got.Data["direct-nil"] != nil || got.Data["nil-list"].([]any)[0] != nil || !mapOK || gotNilMap != nil ||
		!sliceOK || gotNilSlice != nil || (*got.Data["pointer"].(*[]string))[0] != "original" || got.Data["nil-pointer"] != (*[]string)(nil) {
		t.Fatalf("view changed through injected reference alias: %#v", got)
	}

	standardSection := Standard.Skills["awf-effort"].Sections[0]
	_, standardHadProbe := Standard.Skills["awf-effort"].Data["view-probe"]
	complete := CompleteView().Catalog()
	completeSkill := complete.Skills["awf-effort"]
	completeSkill.Sections[0] = "changed view"
	completeSkill.Data = map[string]any{}
	completeSkill.Data["view-probe"] = "changed view"
	complete.Skills["awf-effort"] = completeSkill
	if Standard.Skills["awf-effort"].Sections[0] != standardSection {
		t.Fatal("Standard sections changed through complete view alias")
	}
	_, standardHasProbe := Standard.Skills["awf-effort"].Data["view-probe"]
	if standardHasProbe != standardHadProbe {
		t.Fatal("Standard data changed through complete view alias")
	}

	returned := view.Catalog()
	returnedSkill := returned.Skills["awf-effort"]
	returnedSkill.Sections[0] = "changed returned snapshot"
	returned.Skills["awf-effort"] = returnedSkill
	if view.Catalog().Skills["awf-effort"].Sections[0] == "changed returned snapshot" {
		t.Fatal("View changed through a returned catalog snapshot")
	}
}

func TestAgentsDocSectionsNonEmpty(t *testing.T) {
	cat := Standard
	sections := cat.Docs["agents-doc"].Sections
	if len(sections) == 0 {
		t.Error("expected agents-doc Sections to be non-empty")
	}
	expected := []string{"you-and-this-project", "identity", "invariants", "workflow", "commands", "document-map"}
	for _, s := range expected {
		found := false
		for _, sec := range sections {
			if sec == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected section %q in agents-doc Sections, got: %v", s, sections)
		}
	}
}

// invariant: rendering/catalog-and-targets:no-single-marker-init-descriptor (TestNoSingleMarkerInitDescriptor)
//
// The catalog exposes no single marker/globs var descriptor; qualified markers
// reach config only through currentState.sources.
func TestNoSingleMarkerInitDescriptor(t *testing.T) {
	for _, d := range Standard.Vars {
		if d.Key == "invariantsMarker" || d.Key == "invariantsGlobs" {
			t.Errorf("catalog still declares removed descriptor key %q", d.Key)
		}
		if d.Target == "invariants-marker" || d.Target == "invariants-globs" {
			t.Errorf("catalog still declares removed descriptor target %q", d.Target)
		}
	}

	var live struct {
		CurrentState struct {
			Sources []struct {
				Globs  []string `yaml:"globs"`
				Marker string   `yaml:"marker"`
			} `yaml:"sources"`
		} `yaml:"currentState"`
	}
	configPath := filepath.Join(testsupport.RepoRoot(t), ".awf", "config.yaml")
	body, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(body, &live); err != nil {
		t.Fatal(err)
	}
	const testPath = "internal/catalog/catalog_test.go"
	const qualified = "invariant: rendering/catalog-and-targets:no-single-marker-init-descriptor"
	for _, source := range live.CurrentState.Sources {
		for _, glob := range source.Globs {
			if pathglob.Match(glob, testPath) && source.Marker+" "+qualified == "// "+qualified {
				return
			}
		}
	}
	t.Fatalf("currentState.sources has no configuration route from %s to qualified marker %q", testPath, "// "+qualified)
}
