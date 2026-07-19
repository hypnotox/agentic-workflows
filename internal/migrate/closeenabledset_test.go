package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// closeFixture writes an .awf/config.yaml (plus optional sidecars keyed by
// .awf-relative path) and returns the root.
func closeFixture(t *testing.T, cfg string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		p := filepath.Join(root, ".awf", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCloseEnabledSetAddsExploringFromShippedCatalog(t *testing.T) {
	for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
		t.Run(consumer, func(t *testing.T) {
			root := closeFixture(t, "prefix: ex\nskills: ["+consumer+"]\nagents: []\n", nil)
			var out bytes.Buffer
			if err := applyCloseEnabledSet(root, &out); err != nil {
				t.Fatalf("applyCloseEnabledSet: %v", err)
			}
			want := `close-enabled-set: enabled skill "exploring" (required by "` + consumer + `")`
			if !strings.Contains(out.String(), want) {
				t.Errorf("diagnostic missing %q:\n%s", want, out.String())
			}
			before, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(before), "- exploring") {
				t.Errorf("upgraded config missing exploring:\n%s", before)
			}
			var second bytes.Buffer
			if err := applyCloseEnabledSet(root, &second); err != nil {
				t.Fatalf("second applyCloseEnabledSet: %v", err)
			}
			after, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if second.Len() != 0 || !bytes.Equal(before, after) {
				t.Errorf("second apply changed output=%q config=\n%s", second.String(), after)
			}
		})
	}
}

// Dormant doc-gated skills are dropped (printed), missing requirements are
// added to a fixed point, and a re-run is a byte-identical no-op (ADR-0081
// Decision 8).
// invariant: close-enabled-set-migration
func TestCloseEnabledSetDropsDormantAndCloses(t *testing.T) {
	root := closeFixture(t, "prefix: ex\nskills: [brainstorming, roadmap-graduation, tdd]\nagents: []\n", nil)
	var out bytes.Buffer
	if err := applyCloseEnabledSet(root, &out); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		`close-enabled-set: dropped dormant skill "roadmap-graduation" (its "roadmap" doc is disabled)`,
		`close-enabled-set: enabled skill "proposing-adr" (required by "brainstorming")`,
		`close-enabled-set: enabled agent "plan-reviewer" (required by "reviewing-plan")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	cfg, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- reviewing-plan-resync", "- retrospective", "- adr-lifecycle", "- code-reviewer", "- adr-reviewer"} {
		if !strings.Contains(string(cfg), want) {
			t.Errorf("closed config missing %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(string(cfg), "- roadmap-graduation") {
		t.Errorf("dormant skill not dropped:\n%s", cfg)
	}
	// Idempotence: a second run changes nothing.
	if err := applyCloseEnabledSet(root, &out); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	again, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cfg, again) {
		t.Errorf("re-run must be a byte-identical no-op:\n%s", again)
	}
}

// A local:-owned doc-gated skill renders today even without its doc, so the
// dormancy drop skips it - symmetric with the validator's local skip.
func TestCloseEnabledSetKeepsLocalDormantSkill(t *testing.T) {
	root := closeFixture(t, "prefix: ex\nskills: [roadmap-graduation]\nagents: []\n",
		map[string]string{"skills/roadmap-graduation.yaml": "local: true\n"})
	if err := applyCloseEnabledSet(root, io.Discard); err != nil {
		t.Fatalf("apply: %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "- roadmap-graduation") && !strings.Contains(string(cfg), "[roadmap-graduation]") {
		t.Errorf("local dormant skill must be kept:\n%s", cfg)
	}
}

// An absent config is a no-op (idempotent re-run safe, the editConfig skeleton).
func TestCloseEnabledSetNoConfigNoop(t *testing.T) {
	if err := applyCloseEnabledSet(t.TempDir(), io.Discard); err != nil {
		t.Fatalf("absent config must be a no-op, got %v", err)
	}
}

// A malformed config surfaces the load error rather than mutating anything.
func TestCloseEnabledSetMalformedConfig(t *testing.T) {
	root := closeFixture(t, ": : not valid : :\n", nil)
	if err := applyCloseEnabledSet(root, io.Discard); err == nil {
		t.Fatal("expected a parse error for a malformed config")
	}
}

// A dormant doc-gated skill something enabled still requires re-enters WITH
// its doc - the closure demand outranks the dormancy drop (ADR-0081 Decision
// 8). Unreachable in the shipped catalog (nothing requires a doc-gated
// skill), so a synthetic catalog exercises the interplay via the seam.
func TestCloseEnabledSetReAddsDemandedDormantSkillWithDoc(t *testing.T) {
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{
		"keeper": {RequiresSkills: []string{"gated"}},
		"gated":  {RequiresDoc: "roadmap"},
	}}
	root := closeFixture(t, "prefix: ex\nskills: [gated, keeper]\nagents: []\n", nil)
	var out bytes.Buffer
	if err := closeEnabledSet(root, cat, &out); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		`close-enabled-set: dropped dormant skill "gated" (its "roadmap" doc is disabled)`,
		`close-enabled-set: enabled skill "gated" (required by "keeper")`,
		`close-enabled-set: enabled doc "roadmap" (required by "gated")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	cfg, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "- gated") || !strings.Contains(string(cfg), "- roadmap") {
		t.Errorf("demanded dormant skill must re-enter with its doc:\n%s", cfg)
	}
}
