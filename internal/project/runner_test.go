package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// runnerFile renders a project with the given config and returns the rendered
// command-runner `x` (or nil when none is produced).
func runnerFile(t *testing.T, configYAML string) *RenderedFile {
	t.Helper()
	root := scaffold(t, configYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var found *RenderedFile
	for i := range out {
		if out[i].Path == "x" {
			if found != nil {
				t.Fatalf("more than one runner rendered")
			}
			found = &out[i]
		}
	}
	return found
}

// With the singleton enabled, exactly one runner `x` renders at the repo root;
// absent or disabled, none does.
// invariant: rendering/companion-scripts:runner-singleton-toggle
func TestRunnerToggle(t *testing.T) {
	if runnerFile(t, "prefix: example\nrunner:\n  enabled: true\n") == nil {
		t.Error("expected the runner x to render when enabled")
	}
	for _, cfg := range []string{
		"prefix: example\n",
		"prefix: example\nrunner:\n  enabled: false\n",
	} {
		if rf := runnerFile(t, cfg); rf != nil {
			t.Errorf("expected no runner for config %q, got %q", cfg, rf.Path)
		}
	}
}

// The awf-verb arms are awf-owned (outside any awf:edit-in-place section) and each
// delegates to the bootstrap-resolved pinned binary; the two adopter regions are
// awf:edit-in-place sections rendered as #-comments (the shell comment style).
// invariant: rendering/companion-scripts:runner-awf-verbs-owned
// invariant: rendering/companion-scripts:runner-project-verbs-in-place
func TestRunnerStructure(t *testing.T) {
	rf := runnerFile(t, "prefix: example\nrunner:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("runner did not render")
	}
	if !rf.RegenChecked {
		t.Error("a runner carrying in-place sections must be regeneration-checked")
	}
	c := rf.Content
	if !strings.HasPrefix(c, "#!/usr/bin/env bash\n") {
		t.Errorf("runner must open with the bash shebang:\n%s", c)
	}
	// Each metadata-forwarded awf verb delegates directly to the pinned binary
	// via the bootstrap, with no PATH fallback (ADR-0101 Decision 2/4).
	// invariant: tooling/cli:managed-runner-command-parity
	for _, name := range clispec.ForwardedNames() {
		if strings.Count(c, "\n"+name+")\n") != 1 {
			t.Errorf("forwarded command %q must have exactly one arm:\n%s", name, c)
		}
	}
	for _, name := range []string{"init", "upgrade", "uninstall"} {
		if strings.Contains(c, "\n"+name+")\n") {
			t.Errorf("excluded command %q must not have a runner arm:\n%s", name, c)
		}
	}
	if !strings.Contains(c, "usage: ./x {"+strings.Join(clispec.ForwardedNames(), "|")+"|gate|test}") {
		t.Errorf("runner usage does not derive from forwarded metadata:\n%s", c)
	}
	if !strings.Contains(c, `"$(bash .awf/bootstrap.sh)" "$cmd" "$@" ;;`) {
		t.Errorf("awf arms must delegate directly to the pinned binary:\n%s", c)
	}
	if strings.Contains(c, "command awf") {
		t.Errorf("the runner must not carry a PATH fallback:\n%s", c)
	}
	for _, repositoryCommand := range []string{"dashboard-awf-path", "dashboard-awf-advance"} {
		if strings.Contains(c, repositoryCommand) {
			t.Errorf("generic runner must not publish repository-only command %q:\n%s", repositoryCommand, c)
		}
	}
	// The two adopter regions are #-comment in-place pointers (shell comment
	// style), never HTML.
	for _, name := range []string{"runner-setup", "runner-project-verbs"} {
		want := "# awf:edit-in-place " + name + ": "
		if !strings.Contains(c, want) {
			t.Errorf("missing #-style in-place pointer for %q:\n%s", name, c)
		}
	}
	if strings.Contains(c, "<!-- awf:edit") {
		t.Errorf("shell runner must not carry HTML-comment pointers:\n%s", c)
	}
}

// The runner renders leak-free (no unresolved token, no stray section/marker
// residue) - the publication-safety contract every awf template meets.
// invariant: rendering/companion-scripts:runner-render-publication-safe
func TestRunnerProjectVerbLabelsAndCollisions(t *testing.T) {
	labels, err := runnerProjectVerbLabels("alpha | beta)\n\techo ok ;;\nz9)\n\techo z ;;\n")
	if err != nil || strings.Join(labels, ",") != "alpha,beta,z9" {
		t.Fatalf("labels = %v, err = %v", labels, err)
	}
	for _, body := range []string{")\n", "Bad)\n", "bad_name)\n"} {
		if _, err := runnerProjectVerbLabels(body); err == nil {
			t.Errorf("expected malformed label error for %q", body)
		}
	}
	if err := validateRunnerProjectVerbs("Bad)\n"); err == nil || !strings.Contains(err.Error(), "malformed case label") {
		t.Fatalf("malformed validation error = %v", err)
	}
	if err := validateRunnerProjectVerbs("sync)\n"); err == nil || !strings.Contains(err.Error(), `runner project verb "sync" collides`) {
		t.Fatalf("collision error = %v", err)
	}
}

func TestRunnerCollisionRefusesBeforeWrite(t *testing.T) {
	root := scaffold(t, "prefix: example\nrunner:\n  enabled: true\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	xPath := filepath.Join(root, "x")
	before, err := os.ReadFile(xPath)
	if err != nil {
		t.Fatal(err)
	}
	colliding := setRegion(t, string(before), "runner-project-verbs", "sync)\n\techo collision ;;\n")
	if err := os.WriteFile(xPath, []byte(colliding), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err == nil || !strings.Contains(err.Error(), `runner project verb "sync" collides with metadata-forwarded awf command`) {
		t.Fatalf("Sync collision error = %v", err)
	}
	after, err := os.ReadFile(xPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != colliding {
		t.Fatal("collision refusal rewrote the runner")
	}
}

func TestRunnerPublicationSafe(t *testing.T) {
	rf := runnerFile(t, "prefix: example\nrunner:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("runner did not render")
	}
	if strings.Contains(rf.Content, "<no value>") {
		t.Errorf("runner leaked an unresolved-value token:\n%s", rf.Content)
	}
	for _, marker := range []string{"awf:section", "awf:end"} {
		if strings.Contains(rf.Content, marker) {
			t.Errorf("runner leaked a structural %q marker:\n%s", marker, rf.Content)
		}
	}
}

// The runner is a dedicated config-tree render block, not a catalog DocEntry, so it
// stays out of SingletonKinds() - the unified-doc-model completeness set is
// unchanged by the runner's existence.
// invariant: rendering/singletons-and-payloads:singleton-kinds-complete
func TestRunnerNotASingletonKind(t *testing.T) {
	if slices.Contains(catalog.SingletonKinds(), "runner") {
		t.Error("the runner must not be a catalog SingletonKind (it is a dedicated render block)")
	}
}

// A convention part authored for an awf-owned runner section (as its
// `create ... to override` pointer invites) is claimed by the closed-tree sweep, so
// override renders and `awf check` does not flag `.awf/runner` as unclaimed.
func TestRunnerInPlacePartReadError(t *testing.T) {
	root := scaffold(t, "prefix: example\nrunner:\n  enabled: true\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, ".awf/runner/parts/runner-setup.md")
	if err := os.MkdirAll(part, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderAll(); err == nil {
		t.Fatal("in-place part read error accepted")
	}
}

func TestRunnerPartOverrideClaimed(t *testing.T) {
	root := scaffold(t, "prefix: example\nrunner:\n  enabled: true\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, ".awf/runner/parts/runner-tail.md")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("*)\n\techo custom-tail ;;\nesac\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	x, err := os.ReadFile(filepath.Join(root, "x"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(x), "custom-tail") {
		t.Errorf("runner-tail part override not applied:\n%s", x)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if strings.Contains(d.Path, ".awf/runner") {
			t.Errorf("runner parts must be claimed by the sweep, got drift %v", d)
		}
	}
}
