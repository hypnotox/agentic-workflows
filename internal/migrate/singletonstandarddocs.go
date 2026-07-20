package migrate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"gopkg.in/yaml.v3"
)

// singletonStandardDocNames are the three docs ADR-0043 promotes from
// toggleable `docs:` catalog entries to always-on singletons.
// touches-state: config/migrations-and-locks:singleton-doc-migration-relocates-parts - the doc set whose parts/sidecars migrate; proof in singletonstandarddocs_test.go
var singletonStandardDocNames = []string{"workflow", "doc-standard", "agents-md-standard"}

// applySingletonStandardDocs ports each of singletonStandardDocNames from the
// plural docs shape to the singleton shape (config.IsSingletonKind), mirroring
// portAgentsDoc's relocation when agents-doc first became a singleton: its
// sidecar moves from <awfDir>/docs/<name>.yaml to <awfDir>/<name>.yaml, its
// convention-part dir from <awfDir>/docs/parts/<name>/ to <awfDir>/parts/<name>/,
// then <name> is stripped from the docs: array - each step a no-op if its
// source is already absent, so a repeated run is idempotent. Each performed
// operation prints one provenance line to out.
func applySingletonStandardDocs(root string, out io.Writer) error {
	awfDir := config.RootDir(root)
	for _, name := range singletonStandardDocNames {
		relocations := []struct{ src, dst []string }{
			{src: []string{"docs", name + ".yaml"}, dst: []string{name + ".yaml"}},
			{src: []string{"docs", "parts", name}, dst: []string{"parts", name}},
		}
		for _, r := range relocations {
			moved, err := relocate(filepath.Join(awfDir, filepath.Join(r.src...)), filepath.Join(awfDir, filepath.Join(r.dst...)))
			if err != nil {
				return err
			}
			if moved {
				fmt.Fprintf(out, "singleton-standard-docs: moved %s → %s\n",
					path.Join(config.DirName, path.Join(r.src...)), path.Join(config.DirName, path.Join(r.dst...)))
			}
		}
		removed, err := removeFromDocsArray(filepath.Join(awfDir, "config.yaml"), name)
		if err != nil {
			return err
		}
		if removed {
			fmt.Fprintf(out, "singleton-standard-docs: removed doc %q from docs: (now always-on)\n", name)
		}
	}
	return nil
}

// relocate renames src to dst if src exists (file or directory), reporting
// whether a rename happened; a no-op when src is absent. It refuses rather than
// clobber an existing destination (mirroring applyAwfRelocation), so a partial
// prior migration cannot be silently overwritten.
func relocate(src, dst string) (bool, error) {
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil { // e.g. ENOTDIR: a path component of src is a regular file
		return false, err
	}
	if _, err := os.Stat(dst); err == nil {
		return false, fmt.Errorf("cannot relocate: %s already exists", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { // e.g. dst's parent exists as a regular file
		return false, err
	}
	if err := os.Rename(src, dst); err != nil { // coverage-ignore: Rename between two just-Stat'd paths under the same .awf dir fails only on a permission/IO fault a test cannot trigger
		return false, err
	}
	return true, nil
}

// removeFromDocsArray strips name from the docs: array in the config.yaml at
// path, if both the config and the array member are present, reporting whether
// the member was removed. SetArrayMember errors on removing an absent member or
// an absent key, so membership is checked first via a plain decode.
func removeFromDocsArray(path, name string) (bool, error) {
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil { // e.g. EISDIR: the config path exists as a directory
		return false, err
	}
	var probe struct {
		Docs []string `yaml:"docs"`
	}
	if err := yaml.Unmarshal(src, &probe); err != nil {
		return false, err
	}
	present := false
	for _, d := range probe.Docs {
		if d == name {
			present = true
			break
		}
	}
	if !present {
		return false, nil
	}
	updated, err := config.SetArrayMember(src, "docs", name, false)
	if err != nil { // coverage-ignore: the membership check above guarantees name is present under docs:, and yaml.Unmarshal above already validated src parses, so SetArrayMember cannot error here
		return false, err
	}
	// touches-state: config/migrations-and-locks:lock-atomic-save - atomic temp-file+rename write site; proof in manifest_test.go
	if err := manifest.WriteFileAtomic(path, updated); err != nil { // coverage-ignore: the atomic write faults only on a permission/IO error that the test root bypasses
		return false, err
	}
	return true, nil
}
