package project

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

func snapshotVersionFixture(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = contents
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func assertVersionFixtureUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := snapshotVersionFixture(t, root)
	if len(after) != len(before) {
		t.Fatalf("fixture file count after refusal = %d, want %d: %#v", len(after), len(before), after)
	}
	for path, want := range before {
		if got, ok := after[path]; !ok || !bytes.Equal(got, want) {
			t.Fatalf("fixture file %s after refusal = %q, want byte-identical %q", path, got, want)
		}
	}
}

// invariant: config/migrations-and-locks:schema-min-version (TestSchemaMinimumVersionAuthority)
// invariant: tooling/cli:single-version-authority (TestSchemaMinimumVersionAuthority)
func TestSchemaMinimumVersionAuthority(t *testing.T) {
	if got, want := minVersionBySchema, map[int]string{50: "0.44.0", 51: "0.47.0", 52: "0.48.0", 53: "0.49.0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("live schema minimums = %#v, want %#v", got, want)
	}
	if err := ValidateSchemaMinimumVersion(migrate.Current(), Version); err != nil {
		t.Fatalf("current schema minimum: %v", err)
	}
	if err := ValidateSchemaMinimumVersion(migrate.Current(), "0.46.1"); err == nil || !strings.Contains(err.Error(), "requires awf 0.49.0") {
		t.Fatalf("old binary minimum error = %v", err)
	}
	if err := ValidateSchemaMinimumVersion(migrate.LiveSchemaFloor-1, Version); err == nil || !strings.Contains(err.Error(), "no minimum") {
		t.Fatalf("retired schema minimum error = %v", err)
	}
	if err := ValidateSchemaMinimumVersion(migrate.Current()+1, Version); err == nil || !strings.Contains(err.Error(), "no minimum") {
		t.Fatalf("future schema minimum error = %v", err)
	}
}

func TestVersionAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, raw, exposed, want string
		schema                   int
	}{
		{"valid", "0.49.0\n", "0.49.0", "", migrate.Current()},
		{"missing newline", "0.49.0", "0.49.0", "canonical version file", migrate.Current()},
		{"extra newline", "0.49.0\n\n", "0.49.0", "canonical version file", migrate.Current()},
		{"prefixed version", "v0.49.0\n", "v0.49.0", "canonical semantic version", migrate.Current()},
		{"leading zero", "0.047.0\n", "0.047.0", "canonical semantic version", migrate.Current()},
		{"divergent exposed value", "0.49.0\n", "0.48.0", "embedded version", migrate.Current()},
		{"missing schema mapping", "0.49.0\n", "0.49.0", "no minimum", migrate.Current() + 1},
		{"retired schema", "0.23.0\n", "0.23.0", "no minimum", 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateVersionAuthority(tc.raw, tc.exposed, tc.schema)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if err := CheckVersionAuthority(); err != nil {
		t.Fatalf("repository version authority: %v", err)
	}
}

func TestSchema49RefusesBeforeEffortOrUpgrade(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "awf")
	build := exec.CommandContext(testContext(t), "go", "build", "-o", binary, "./cmd/awf")
	build.Dir = repoRootDir(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build awf: %v\n%s", err, output)
	}
	run := func(root string, args ...string) (string, error) {
		t.Helper()
		cmd := exec.CommandContext(testContext(t), binary, args...)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		return string(output), err
	}

	root := scaffold(t, "prefix: archive\nintegrationBranch: main\n")
	lockPath := filepath.Join(root, ".awf", "awf.lock")
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: 49, Files: map[string]manifest.Entry{"prior": {}}}
	if err := lock.Save(lockPath); err != nil {
		t.Fatal(err)
	}
	if output, err := run(root, "effort", "list"); err == nil || !strings.Contains(output, "below live floor 50") || strings.Contains(output, "run awf upgrade") {
		t.Fatalf("generation-49 effort command = %v\n%s; want unsupported live-source refusal", err, output)
	}

	before := snapshotVersionFixture(t, root)
	if output, err := run(root, "upgrade"); err == nil || !strings.Contains(output, "below live floor 50") {
		t.Fatalf("awf upgrade = %v\n%s; want below-floor refusal", err, output)
	}
	assertVersionFixtureUnchanged(t, root, before)
}
