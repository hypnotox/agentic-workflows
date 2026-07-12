package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// ctxYAML configures three domains: alpha and beta both own cmd/** (a path can
// be owned by two domains), gamma declares no paths (unreachable by path query).
const ctxYAML = `prefix: example
vars:
  gateCmd: make gate
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
  - beta
  - gamma
invariants:
  sources:
    - globs:
        - '**/*.go'
      marker: '//'
`

func ctxProject(t *testing.T, configYAML string) (string, *Project) {
	t.Helper()
	root := scaffoldFiles(t, configYAML, map[string]string{
		"domains/alpha.yaml": "paths:\n  - cmd/**\n",
		"domains/beta.yaml":  "paths:\n  - cmd/**\n  - lib/**\n",
		"domains/gamma.yaml": "paths: []\n",
	})
	// A source file backing an invariant marker under cmd/.
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "x.go"), "package x\n// invariant: backed-here\n")
	// An ADR tagged alpha, declaring an inv slug NOT backed under cmd/ (the
	// ADR-side half of the invariants join must still surface it).
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-a.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
			testsupport.WithTitle("0001: Alpha decision"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- `inv: declared-slug` — a contract.\n## Consequences\nc\n")))
	// An ADR tagged only an unowned domain — excluded.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0002-b.md"),
		testsupport.ADR("Proposed", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
			testsupport.WithTitle("0002: Unrelated"), testsupport.WithDomains("other"),
			testsupport.WithBody("## Invariants\n- textual only.\n## Consequences\nc\n")))
	// A second ADR tagged alpha, so the related-ADR set has more than one (the
	// result is sorted by number).
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0003-c.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
			testsupport.WithTitle("0003: Later decision"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- textual only.\n## Consequences\nc\n")))
	// Two plans linking alpha-owned ADRs (0001, 0003) → both surfaced, sorted by
	// filename (also-linked before linked).
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-linked.md"),
		"---\ndate: 2026-07-12\nadrs: [1]\nstatus: Proposed\n---\n# Plan: Linked\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-also-linked.md"),
		"---\ndate: 2026-07-12\nadrs: [3]\nstatus: Implemented\n---\n# Plan: Also Linked\n")
	// A plan linking only ADR 0002 (unowned domain → never surfaced).
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-unlinked.md"),
		"---\ndate: 2026-07-12\nadrs: [2]\nstatus: Proposed\n---\n# Plan: Unlinked\n")
	// A grandfathered frontmatter-less plan — skipped even though it exists.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-06-24-legacy.md"),
		"# Plan: Legacy\n\nNo frontmatter.\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, p
}

func TestContextForAssembles(t *testing.T) {
	_, p := ctxProject(t, ctxYAML)

	res, err := p.ContextFor([]string{"cmd/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	// A path under two domains → both, sorted; gamma (no paths) never appears.
	if names := domainNames(res); names != "alpha,beta" {
		t.Errorf("domains: got %q want %q", names, "alpha,beta")
	}
	if res.Domains[0].CurrentState != "docs/domains/alpha.md" {
		t.Errorf("current-state pointer: got %q", res.Domains[0].CurrentState)
	}
	// The marker under cmd/ is surfaced; the ADR-declared slug is NOT in the
	// path-backed set (it is backed nowhere under cmd/).
	if strings.Join(res.Invariants, ",") != "backed-here" {
		t.Errorf("invariants: got %v want [backed-here]", res.Invariants)
	}
	if len(res.ADRs) != 2 || res.ADRs[0].Number != "0001" || res.ADRs[1].Number != "0003" {
		t.Fatalf("adrs: got %+v, want [0001 0003] sorted (0002 excluded)", res.ADRs)
	}
	a := res.ADRs[0]
	if a.Title != "Alpha decision" { // "ADR-0001: " prefix stripped
		t.Errorf("adr title: got %q want %q", a.Title, "Alpha decision")
	}
	if strings.Join(a.Invariants, ",") != "declared-slug" { // ADR-side half, surfaced with provenance
		t.Errorf("adr invariants: got %v want [declared-slug]", a.Invariants)
	}
	if len(res.Unowned) != 0 {
		t.Errorf("unowned: got %v want none", res.Unowned)
	}
}

// invariant: context-surfaces-linked-plans
func TestContextForSurfacesLinkedPlans(t *testing.T) {
	_, p := ctxProject(t, ctxYAML)

	res, err := p.ContextFor([]string{"cmd/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	// The two plans linking surfaced ADRs (0001, 0003) appear, sorted by filename;
	// the plan linking the unowned ADR 0002 and the frontmatter-less legacy plan
	// do not.
	if len(res.Plans) != 2 {
		t.Fatalf("plans: got %+v, want the two alpha-linked plans", res.Plans)
	}
	if res.Plans[0].Filename != "2026-07-12-also-linked.md" || res.Plans[1].Filename != "2026-07-12-linked.md" {
		t.Errorf("plans not sorted by filename: got %+v", res.Plans)
	}
	pl := res.Plans[1]
	if pl.Path != "docs/plans/2026-07-12-linked.md" {
		t.Errorf("plan path: got %q", pl.Path)
	}
	if pl.Status != "Proposed" || len(pl.ADRs) != 1 || pl.ADRs[0] != 1 {
		t.Errorf("plan ref fields: got %+v", pl)
	}
}

// context.go's plan.ParseDir error propagates rather than silently dropping plans.
func TestContextForPropagatesPlanParseError(t *testing.T) {
	root, p := ctxProject(t, ctxYAML)
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-linked.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	if _, err := p.ContextFor([]string{"cmd/x.go"}); err == nil {
		t.Fatal("expected ContextFor to propagate the plan parse error, got nil")
	}
}

func TestContextForUnownedAndDedup(t *testing.T) {
	_, p := ctxProject(t, ctxYAML)

	// Unclean + duplicate paths collapse; an unowned path lands in Unowned.
	res, err := p.ContextFor([]string{"./cmd/", "cmd", ".", "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Paths, ",") != "README.md,cmd" {
		t.Errorf("paths: got %v want [README.md cmd]", res.Paths)
	}
	if strings.Join(res.Unowned, ",") != "README.md" {
		t.Errorf("unowned: got %v want [README.md]", res.Unowned)
	}
	if domainNames(res) != "alpha,beta" {
		t.Errorf("domains: got %q", domainNames(res))
	}
}

func TestContextForInvariantsDisabled(t *testing.T) {
	const disabledYAML = `prefix: example
vars:
  gateCmd: make gate
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
  - beta
  - gamma
invariants:
  disabled: true
`
	_, p := ctxProject(t, disabledYAML)
	res, err := p.ContextFor([]string{"cmd/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Invariants) != 0 {
		t.Errorf("disabled invariants: got %v want none", res.Invariants)
	}
}

// Each reader's fault propagates out of ContextFor. Faults are induced after
// Open (config.Sidecar and adr.ParseDir re-read from disk on each call).
func TestContextForReaderFaults(t *testing.T) {
	t.Run("sidecar parse error", func(t *testing.T) {
		root, p := ctxProject(t, ctxYAML)
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "alpha.yaml"), "paths: [\n")
		if _, err := p.ContextFor([]string{"cmd/x.go"}); err == nil {
			t.Error("expected a domain-sidecar parse error")
		}
	})
	t.Run("ADR parse error", func(t *testing.T) {
		root, p := ctxProject(t, ctxYAML)
		testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0009-bad.md"), "---\nstatus: \"unterminated\n---\n# ADR-X: T\n")
		if _, err := p.ContextFor([]string{"cmd/x.go"}); err == nil {
			t.Error("expected an ADR parse error")
		}
	})
	t.Run("marker scan error", func(t *testing.T) {
		root, p := ctxProject(t, ctxYAML)
		if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "cmd", "bad.go")); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ContextFor([]string{"cmd"}); err == nil {
			t.Error("expected a marker-scan read error")
		}
	})
}

func domainNames(res ContextResult) string {
	var n []string
	for _, d := range res.Domains {
		n = append(n, d.Name)
	}
	return strings.Join(n, ",")
}
