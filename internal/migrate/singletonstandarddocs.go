package migrate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"gopkg.in/yaml.v3"
)

// singletonStandardDocNames are the three docs ADR-0043 promotes from
// toggleable `docs:` catalog entries to always-on singletons.
// invariant: singleton-doc-migration-relocates-parts
var singletonStandardDocNames = []string{"workflow", "doc-standard", "agents-md-standard"}

// applySingletonStandardDocs ports each of singletonStandardDocNames from the
// plural docs shape to the singleton shape (config.IsSingletonKind), mirroring
// portAgentsDoc's relocation when agents-doc first became a singleton: its
// sidecar moves from <awfDir>/docs/<name>.yaml to <awfDir>/<name>.yaml, its
// convention-part dir from <awfDir>/docs/parts/<name>/ to <awfDir>/parts/<name>/,
// then <name> is stripped from the docs: array — each step a no-op if its
// source is already absent, so a repeated run is idempotent.
func applySingletonStandardDocs(root string, _ io.Writer) error {
	awfDir := config.RootDir(root)
	for _, name := range singletonStandardDocNames {
		if err := relocate(filepath.Join(awfDir, "docs", name+".yaml"), filepath.Join(awfDir, name+".yaml")); err != nil { // coverage-ignore: relocate errors here only on the existing-destination guard or a permission fault, neither of which occurs over the fresh trees this migration runs on
			return err
		}
		if err := relocate(filepath.Join(awfDir, "docs", "parts", name), filepath.Join(awfDir, "parts", name)); err != nil { // coverage-ignore: relocate errors here only on the existing-destination guard or a permission fault, neither of which occurs over the fresh trees this migration runs on
			return err
		}
		if err := removeFromDocsArray(filepath.Join(awfDir, "config.yaml"), name); err != nil {
			return err
		}
	}
	return nil
}

// relocate renames src to dst if src exists (file or directory); a no-op when
// src is absent. It refuses rather than clobber an existing destination (mirroring
// applyAwfRelocation), so a partial prior migration cannot be silently overwritten.
func relocate(src, dst string) error {
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil { // coverage-ignore: Stat fails here only on a permission fault a test cannot trigger
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("cannot relocate: %s already exists", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { // coverage-ignore: dst's parent is under the just-Stat'd .awf dir; fails only on a permission fault a test cannot trigger
		return err
	}
	return os.Rename(src, dst)
}

// removeFromDocsArray strips name from the docs: array in the config.yaml at
// path, if both the config and the array member are present. SetArrayMember
// errors on removing an absent member or an absent key, so membership is
// checked first via a plain decode.
func removeFromDocsArray(path, name string) error {
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil { // coverage-ignore: ReadFile faults only on a permission error that the test root bypasses
		return err
	}
	var probe struct {
		Docs []string `yaml:"docs"`
	}
	if err := yaml.Unmarshal(src, &probe); err != nil {
		return err
	}
	present := false
	for _, d := range probe.Docs {
		if d == name {
			present = true
			break
		}
	}
	if !present {
		return nil
	}
	updated, err := config.SetArrayMember(src, "docs", name, false)
	if err != nil { // coverage-ignore: the membership check above guarantees name is present under docs:, and yaml.Unmarshal above already validated src parses, so SetArrayMember cannot error here
		return err
	}
	// invariant: lock-atomic-save
	return manifest.WriteFileAtomic(path, updated)
}
