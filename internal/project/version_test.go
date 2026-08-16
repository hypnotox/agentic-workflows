package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// invariant: config/migrations-and-locks:schema-min-version (TestSchemaMinimumVersionAuthority)
func TestSchemaMinimumVersionAuthority(t *testing.T) {
	for schema, minimum := range minVersionBySchema {
		if err := ValidateSchemaMinimumVersion(schema, minimum); err != nil {
			t.Errorf("schema %d at minimum %s: %v", schema, minimum, err)
		}
	}
	// Derived from the registry rather than a literal generation: the loop above
	// compares each minimum against itself and so can never fail, and a
	// hard-coded generation stops naming the current one the moment a migration
	// registers. Pinning migrate.Current() keeps this assertion pointed at the
	// generation the claim is about, on every future bump.
	if err := ValidateSchemaMinimumVersion(migrate.Current(), Version); err != nil {
		t.Fatalf("current schema minimum: %v", err)
	}
	if got := minVersionBySchema[38]; got != "0.31.0" {
		t.Fatalf("generation-38 minimum version = %q, want 0.31.0", got)
	}
	if got := minVersionBySchema[20]; got != "0.24.0" {
		t.Fatalf("generation-20 minimum version = %q, want 0.24.0", got)
	}
	if err := ValidateSchemaMinimumVersion(20, "0.23.0"); err == nil || !strings.Contains(err.Error(), "requires awf 0.24.0") {
		t.Fatalf("generation-20 older binary error = %v", err)
	}
	// A binary that predates the unified-effort-resident generation must refuse
	// a tree that has already advanced to it.
	if err := ValidateSchemaMinimumVersion(22, "0.25.0"); err == nil || !strings.Contains(err.Error(), "requires awf 0.26.0") {
		t.Fatalf("generation-22 older binary error = %v", err)
	}
	if err := ValidateSchemaMinimumVersion(migrate.Current()+1, Version); err == nil || !strings.Contains(err.Error(), "no minimum") {
		t.Fatalf("unmapped schema error = %v", err)
	}
}

func TestVersionAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, raw, exposed, want string
		schema                   int
	}{
		{"valid", "0.39.0\n", "0.39.0", "", migrate.Current()},
		{"missing newline", "0.39.0", "0.39.0", "canonical version file", migrate.Current()},
		{"extra newline", "0.39.0\n\n", "0.39.0", "canonical version file", migrate.Current()},
		{"prefixed version", "v0.39.0\n", "v0.39.0", "canonical semantic version", migrate.Current()},
		{"leading zero", "0.039.0\n", "0.039.0", "canonical semantic version", migrate.Current()},
		{"divergent exposed value", "0.39.0\n", "0.38.0", "embedded version", migrate.Current()},
		{"missing schema mapping", "0.39.0\n", "0.39.0", "no minimum", migrate.Current() + 1},
		{"below schema floor", "0.23.0\n", "0.23.0", "requires awf 0.24.0", 20},
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

// invariant: config/migrations-and-locks:archive-root-upgrade-boundary (TestArchiveRootUpgradeBoundary)
func TestArchiveRootUpgradeBoundary(t *testing.T) {
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
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: 41, Files: map[string]manifest.Entry{}}
	if err := lock.Save(lockPath); err != nil {
		t.Fatal(err)
	}
	if output, err := run(root, "effort", "list"); err == nil || !strings.Contains(output, "awf upgrade") {
		t.Fatalf("generation-41 effort command = %v\n%s; want upgrade gate", err, output)
	}

	if output, err := run(root, "upgrade"); err != nil {
		t.Fatalf("awf upgrade: %v\n%s", err, output)
	}
	lock, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	const markerRel = ".awf/effort-archive/.gitignore"
	if lock.SchemaVersion != migrate.Current() {
		t.Fatalf("upgraded lock schema = %d, want current %d", lock.SchemaVersion, migrate.Current())
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var wantEntry manifest.Entry
	for _, file := range planned {
		if file.Path == markerRel {
			wantEntry = manifest.Entry{
				TemplateID: file.TemplateID, TemplateHash: file.TemplateHash,
				ConfigHash: file.ConfigHash, OutputHash: manifest.Hash([]byte(file.Content)),
				RegenChecked: file.RegenChecked,
			}
			break
		}
	}
	if got, ok := lock.Files[markerRel]; !ok || got != wantEntry {
		t.Fatalf("upgraded lock marker entry = %#v, present %v; want %#v", got, ok, wantEntry)
	}
	marker := filepath.Join(root, filepath.FromSlash(markerRel))
	want := "# " + bannerText + "\n*\n!.gitignore\n"
	assertMarker := func(state string) {
		t.Helper()
		got, err := os.ReadFile(marker)
		if err != nil || string(got) != want {
			t.Fatalf("%s marker = %q, %v; want %q", state, got, err, want)
		}
	}
	assertMarker("upgraded")
	archiveDescendant := filepath.Join(root, ".awf", "effort-archive", "id-slug", "nested", "adversarial.go")
	const archiveBytes = "not valid Go and never interpreted\n"
	if err := os.MkdirAll(filepath.Dir(archiveDescendant), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archiveDescendant, []byte(archiveBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	assertArchiveDescendant := func(state string) {
		t.Helper()
		got, err := os.ReadFile(archiveDescendant)
		if err != nil || string(got) != archiveBytes {
			t.Fatalf("%s archive descendant = %q, %v; want byte-identical", state, got, err)
		}
	}

	if output, err := run(root, "render"); err != nil || strings.Contains(output, markerRel) {
		t.Fatalf("correct marker render = %v\n%s; want unchanged marker", err, output)
	}
	assertMarker("unchanged")
	assertArchiveDescendant("unchanged marker render")
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if output, err := run(root, "render"); err != nil || !strings.Contains(output, markerRel) {
		t.Fatalf("missing marker repair = %v\n%s", err, output)
	}
	assertMarker("missing repair")
	assertArchiveDescendant("missing marker repair")
	if err := os.WriteFile(marker, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := run(root, "render"); err != nil || !strings.Contains(output, markerRel) {
		t.Fatalf("stale marker repair = %v\n%s", err, output)
	}
	assertMarker("stale repair")
	assertArchiveDescendant("stale marker repair")
}
