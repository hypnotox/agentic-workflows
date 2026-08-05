package migrate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"gopkg.in/yaml.v3"
)

const layerCatalogListsGeneration = 35

// layerCatalogListSnapshot is the exact same-key catalog list population at
// the schema-35 cutover. It is deliberately frozen here rather than derived
// from catalog.Standard, so later catalog additions are never preemptively
// suppressed by this migration.
var layerCatalogListSnapshot = []struct {
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

type sidecarListEdit struct {
	path    string
	source  []byte
	mode    os.FileMode
	present map[string]bool
	nulls   map[string]bool
}

type atomicSidecarWriter func(string, []byte, os.FileMode) error

func applyLayerCatalogLists(root string, out io.Writer) error {
	return applyLayerCatalogListsWithWriter(root, out, manifest.WriteFileAtomicMode)
}

// applyLayerCatalogListsWithWriter separates the operation's atomic write
// dependency so interruption tests can fail a later replacement without a
// mutable package-global seam.
func applyLayerCatalogListsWithWriter(root string, out io.Writer, write atomicSidecarWriter) error {
	edits := make([]sidecarListEdit, 0, len(layerCatalogListSnapshot))
	for _, artifact := range layerCatalogListSnapshot {
		path := filepath.Join(root, config.DirName, artifact.kind, artifact.artifact+".yaml")
		source, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil { // coverage-ignore: the file was read immediately above
			return err
		}
		var decoded struct {
			Data map[string]any `yaml:"data"`
		}
		if err := yaml.Unmarshal(source, &decoded); err != nil {
			return fmt.Errorf("parse sidecar %s: %w", filepath.ToSlash(path), err)
		}
		edit := sidecarListEdit{path: path, source: source, mode: info.Mode().Perm(), present: map[string]bool{}, nulls: map[string]bool{}}
		for _, key := range artifact.keys {
			value, present := decoded.Data[key]
			if !present {
				continue
			}
			edit.present[key] = true
			if value == nil {
				edit.nulls[key] = true
				continue
			}
			if _, ok := value.([]any); !ok {
				rel, _ := filepath.Rel(root, path)
				return fmt.Errorf("operation: list-replacement migration refused because %s data.%s is non-null and not a list; changed bytes: no; changed index: no; changed message: no; changed merge state: no; next actions: 1. set data.%s to a list or null in %s 2. run `awf upgrade`", filepath.ToSlash(rel), key, key, filepath.ToSlash(rel))
			}
		}
		if len(edit.present) > 0 {
			edits = append(edits, edit)
		}
	}

	for _, edit := range edits {
		updated := edit.source
		for _, artifact := range layerCatalogListSnapshot {
			want := filepath.Join(root, config.DirName, artifact.kind, artifact.artifact+".yaml")
			if want != edit.path {
				continue
			}
			for _, key := range artifact.keys {
				if !edit.present[key] {
					continue
				}
				var err error
				if edit.nulls[key] {
					updated, err = config.RemoveMappingKey(updated, "data", key)
					if err != nil { // coverage-ignore: successful typed preflight guarantees data is an editable mapping
						return err
					}
				}
				updated, err = config.SetMappingScalar(updated, "dataDefaults", key, false)
				if err != nil { // coverage-ignore: successful YAML preflight guarantees an editable document root
					return err
				}
			}
			break
		}
		if bytes.Equal(updated, edit.source) {
			continue
		}
		if err := write(edit.path, updated, edit.mode); err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, edit.path)
		fmt.Fprintf(out, "layer-catalog-lists: updated %s\n", filepath.ToSlash(rel))
	}
	return nil
}
