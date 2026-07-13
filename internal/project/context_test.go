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
	// Markers under cmd/: two whose slug an Implemented ADR declares (Tier 1, one
	// ADR declaring both → the dedup skip on the 2nd), and one orphan slug no
	// Implemented ADR declares (the Tier-1 !ok skip).
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "x.go"),
		"package x\n// invariant: gov-slug\n// invariant: gov-slug2\n// invariant: gov-slug3\n// invariant: orphan-slug\n")
	// 0001 Implemented, declares BOTH present gov slugs; tag `precise` plus the
	// domain-mirror `alpha` (excluded from the precise set); related: [3, 5] → Tier 1.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-a.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"),
			testsupport.WithTags("precise", "alpha"), testsupport.WithRelated(3, 5),
			testsupport.WithTitle("0001: Alpha decision"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- `invariant: gov-slug` — a.\n- `invariant: gov-slug2` — b.\n## Consequences\nc\n")))
	// 0002 Proposed, tag `other`, domain beta (owns cmd) → Tier 3 background only.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0002-b.md"),
		testsupport.ADR("Proposed", testsupport.WithDate("2026-06-25"), testsupport.WithTags("other"),
			testsupport.WithTitle("0002: Unrelated"), testsupport.WithDomains("beta"),
			testsupport.WithBody("## Invariants\n- textual only.\n## Consequences\nc\n")))
	// 0003 Accepted, tag `precise` → Tier 2 (shared precise tag; also related-linked).
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0003-c.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("precise"),
			testsupport.WithTitle("0003: Later decision"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- textual only.\n## Consequences\nc\n")))
	// 0004 Superseded, tag `precise` → excluded from Tier 2 despite the shared tag;
	// domain alpha owns cmd → Tier 3 background.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0004-d.md"),
		testsupport.ADR("Superseded by ADR-0001", testsupport.WithDate("2026-06-25"), testsupport.WithTags("precise"),
			testsupport.WithTitle("0004: Retired"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- textual only.\n## Consequences\nc\n")))
	// 0005 Accepted, tag `nomatch` (no shared tag) but related-linked from 0001 →
	// Tier 2 via the related: graph, exercising the relatedNum branch alone.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0005-e.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("nomatch"),
			testsupport.WithTitle("0005: Related only"), testsupport.WithDomains("other"),
			testsupport.WithBody("## Invariants\n- textual only.\n## Consequences\nc\n")))
	// 0006 Implemented, declares the third present gov slug; tag `alpha` is a
	// domain-mirror (excluded from the precise set) → a second Tier-1 ADR, so the
	// Governing sort comparator runs.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0006-f.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("alpha"),
			testsupport.WithTitle("0006: Second governor"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- `invariant: gov-slug3` — c.\n## Consequences\nc\n")))
	// Plans: link Tier-1/2 ADRs (0001, 0003) → surfaced, sorted by filename; the
	// plan linking the Tier-3 ADR 0002 and the frontmatter-less legacy plan do not.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-linked.md"),
		"---\ndate: 2026-07-12\nadrs: [1]\nstatus: Proposed\n---\n# Plan: Linked\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-also-linked.md"),
		"---\ndate: 2026-07-12\nadrs: [3]\nstatus: Implemented\n---\n# Plan: Also Linked\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-unlinked.md"),
		"---\ndate: 2026-07-12\nadrs: [2]\nstatus: Proposed\n---\n# Plan: Unlinked\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-06-24-legacy.md"),
		"# Plan: Legacy\n\nNo frontmatter.\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, p
}

// TestContextForAssembles exercises the three-tier assembly: Tier 1 (an ADR
// declaring a present invariant slug, deduped when it declares two, an orphan
// present slug skipped), Tier 2 (shared precise tag and related-linked, with the
// domain-mirror tag excluded and a Superseded ADR dropped), and the Tier-3
// collapsed background count.
// invariant: context-tier1-governs
// invariant: context-tier2-topical
// invariant: context-tier3-collapsed
func TestContextForAssembles(t *testing.T) {
	_, p := ctxProject(t, ctxYAML)

	res, err := p.ContextFor([]string{"cmd/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	if names := domainNames(res); names != "alpha,beta" {
		t.Errorf("domains: got %q want %q", names, "alpha,beta")
	}
	if res.Domains[0].CurrentState != "docs/domains/alpha.md" {
		t.Errorf("current-state pointer: got %q", res.Domains[0].CurrentState)
	}
	// Tier 1: 0001 (declares gov-slug + gov-slug2, deduped) and 0006 (gov-slug3),
	// sorted; the orphan slug maps to no ADR.
	if len(res.Governing) != 2 || res.Governing[0].Number != "0001" || res.Governing[1].Number != "0006" {
		t.Fatalf("governing: got %+v, want [0001 0006]", res.Governing)
	}
	if res.Governing[0].Title != "Alpha decision" { // "ADR-0001: " prefix stripped
		t.Errorf("governing title: got %q", res.Governing[0].Title)
	}
	// Tier 2: 0003 (shared precise tag) + 0005 (related-linked, no shared tag),
	// sorted; 0004 (Superseded) and 0002 (no precise tag, not related) excluded.
	if len(res.Related) != 2 || res.Related[0].Number != "0003" || res.Related[1].Number != "0005" {
		t.Fatalf("related: got %+v, want [0003 0005]", res.Related)
	}
	// Tier 3: 0002 (beta owns cmd) + 0004 (alpha owns cmd), collapsed to a count.
	if res.Background != 2 {
		t.Errorf("background: got %d want 2 (0002, 0004)", res.Background)
	}
	if len(res.Unowned) != 0 {
		t.Errorf("unowned: got %v want none", res.Unowned)
	}
}

// invariant: context-surfaces-tiered-plans
func TestContextForSurfacesTieredPlans(t *testing.T) {
	_, p := ctxProject(t, ctxYAML)

	res, err := p.ContextFor([]string{"cmd/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	// The two plans linking a Tier-1/Tier-2 ADR (0001 Governing, 0003 Related)
	// appear, sorted by filename; the plan linking the Tier-3 ADR 0002 and the
	// frontmatter-less legacy plan do not.
	if len(res.Plans) != 2 {
		t.Fatalf("plans: got %+v, want the two tier-1/2-linked plans", res.Plans)
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

// A duplicate inv slug across two Implemented ADRs makes DeclaringADRs error,
// which ContextFor propagates (the Tier-1 join is the shared one-to-one map).
func TestContextForDuplicateSlugError(t *testing.T) {
	root, p := ctxProject(t, ctxYAML)
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0007-dup.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
			testsupport.WithTitle("0007: Dup"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- `invariant: gov-slug` — clash.\n## Consequences\nc\n")))
	if _, err := p.ContextFor([]string{"cmd/x.go"}); err == nil {
		t.Fatal("expected a duplicate inv slug error from DeclaringADRs")
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

// TestContextForLabelsAndNotes exercises ADR-0106: each governing invariant
// surfaced under a queried production path is labelled backed/unbacked from its
// declaring ADR's class; an unbacked invariant surfaces its Verify: guidance and
// a touches marker its site note; and a proof marker in a test file under the
// queried production directory surfaces via the union scan.
func TestContextForLabelsAndNotes(t *testing.T) {
	const yaml = "prefix: example\nvars: {}\nskills: []\nagents: []\ndomains: [foo]\n" +
		"invariants:\n  sources:\n    - globs: ['**/*.go']\n      marker: '//'\n"
	root := scaffoldFiles(t, yaml, map[string]string{"domains/foo.yaml": "paths:\n  - internal/foo/**\n"})
	// Production file: an unbacked slug's touches note and a slug marked by both a
	// proof and a touches marker.
	testsupport.WriteFile(t, filepath.Join(root, "internal", "foo", "x.go"), "package x\n"+
		"// touches-invariant: unbacked-slug — the reasoned site.\n"+
		"// invariant: dual-slug\n"+
		"// touches-invariant: dual-slug — dual note.\n")
	// Test file under the same production directory: a proof marker only. The union
	// scan surfaces it when the production directory is queried (ADR-0106).
	testsupport.WriteFile(t, filepath.Join(root, "internal", "foo", "x_test.go"), "package x\n// invariant: backed-slug\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-a.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
			testsupport.WithTitle("0001: Labels"), testsupport.WithDomains("foo"),
			testsupport.WithBody("## Invariants\n"+
				"- `invariant: backed-slug` — b.\n"+
				"- `invariant: dual-slug` — d.\n"+
				"- `unbacked-invariant: unbacked-slug` — a reasoned contract. **Verify:** inspect by hand.\n"+
				"## Consequences\nc\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.ContextFor([]string{"internal/foo"})
	if err != nil {
		t.Fatal(err)
	}
	byslug := map[string]InvariantRef{}
	var order []string
	for _, iv := range res.Invariants {
		byslug[iv.Slug] = iv
		order = append(order, iv.Slug)
	}
	if strings.Join(order, ",") != "backed-slug,dual-slug,unbacked-slug" {
		t.Fatalf("invariants (slug-sorted): got %v", order)
	}
	// backed-slug: surfaced via a proof marker in a test file (union scan), backed.
	if b := byslug["backed-slug"]; b.Class != "backed" || b.Verify != "" || len(b.Touches) != 0 {
		t.Errorf("backed-slug: %+v", b)
	}
	// dual-slug: proof + touches, backed, carries the touches note.
	if d := byslug["dual-slug"]; d.Class != "backed" || len(d.Touches) != 1 || !strings.Contains(d.Touches[0], "dual note") {
		t.Errorf("dual-slug: %+v", d)
	}
	// unbacked-slug: touches-only, labelled unbacked, surfaces Verify + touches note.
	u := byslug["unbacked-slug"]
	if u.Class != "unbacked" || u.Verify != "inspect by hand." {
		t.Errorf("unbacked-slug label: %+v", u)
	}
	if len(u.Touches) != 1 || !strings.Contains(u.Touches[0], "reasoned site") {
		t.Errorf("unbacked-slug touches: %+v", u.Touches)
	}
	// All three are declared by 0001 → a single deduped Governing entry.
	if len(res.Governing) != 1 || res.Governing[0].Number != "0001" {
		t.Errorf("governing: %+v", res.Governing)
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

const ctxPitfallsYAML = "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: [alpha, beta]\n" +
	"invariants:\n  sources:\n    - globs: ['**/*.go']\n      marker: '//'\n"

// ctxPitfallsProject scaffolds a tree where alpha owns cmd/**, beta owns lib/**,
// a marker + Implemented ADR under cmd/ produce the precise tag `ptag`, and the
// pitfalls sidecar carries the given data.
func ctxPitfallsProject(t *testing.T, sidecar string) (string, *Project) {
	t.Helper()
	root := scaffoldFiles(t, ctxPitfallsYAML, map[string]string{
		"domains/alpha.yaml": "paths:\n  - cmd/**\n",
		"domains/beta.yaml":  "paths:\n  - lib/**\n",
		"docs/pitfalls.yaml": sidecar,
	})
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "x.go"), "package x\n// invariant: p-slug\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-a.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("ptag"),
			testsupport.WithTitle("0001: P"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- `invariant: p-slug` — x.\n## Consequences\nc\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, p
}

// A pitfall sharing the query's precise tag (ptag) surfaces (sorted by title); a
// pitfall with only a non-matching tag, and an untagged pitfall, do not.
// invariant: context-surfaces-tiered-pitfalls
func TestContextForSurfacesTieredPitfalls(t *testing.T) {
	_, p := ctxPitfallsProject(t, "data:\n  pitfalls:\n"+
		"    - title: Bravo\n      tags: [ptag]\n      body: b\n"+
		"    - title: Alfa\n      tags: [ptag]\n      body: b\n"+
		"    - title: NoMatch\n      tags: [zzz]\n      body: b\n"+
		"    - title: Untagged\n      body: b\n")
	res, err := p.ContextFor([]string{"cmd/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, pf := range res.Pitfalls {
		got = append(got, pf.Title)
	}
	if strings.Join(got, ",") != "Alfa,Bravo" {
		t.Fatalf("pitfalls: got %v want [Alfa Bravo] sorted (NoMatch + Untagged excluded)", got)
	}
	if res.Pitfalls[0].Path != "docs/pitfalls.md" || strings.Join(res.Pitfalls[0].Tags, ",") != "ptag" {
		t.Errorf("pitfall ref fields wrong: %+v", res.Pitfalls[0])
	}
}

// A disabled pitfalls doc surfaces no pitfalls.
func TestContextForPitfallsDisabled(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: [alpha]\n",
		map[string]string{"domains/alpha.yaml": "paths:\n  - cmd/**\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.ContextFor([]string{"cmd/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Pitfalls != nil {
		t.Errorf("disabled pitfalls must surface none, got %v", res.Pitfalls)
	}
}

// Both pitfalls reader faults propagate: a structural error (valid YAML, bad
// shape) and a post-Open sidecar parse error (the sidecar re-reads on each call).
func TestContextForPitfallsFaults(t *testing.T) {
	t.Run("structural error", func(t *testing.T) {
		_, p := ctxPitfallsProject(t, "data:\n  pitfalls: not-a-list\n")
		if _, err := p.ContextFor([]string{"cmd/x.go"}); err == nil {
			t.Error("expected a pitfalls structural error")
		}
	})
	t.Run("sidecar parse error", func(t *testing.T) {
		root, p := ctxPitfallsProject(t, "data:\n  pitfalls: []\n")
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "docs", "pitfalls.yaml"), "data:\n  pitfalls: [\n")
		if _, err := p.ContextFor([]string{"cmd/x.go"}); err == nil {
			t.Error("expected a pitfalls sidecar parse error")
		}
	})
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

// uncoveredBase is ctxYAML without a domains block, so each Uncovered case injects
// its own domain ownership.
const uncoveredBase = `prefix: example
vars:
  gateCmd: make gate
skills:
  - tdd
agents:
  - code-reviewer
invariants:
  sources:
    - globs:
        - '**/*.go'
      marker: '//'
`

func TestUncovered(t *testing.T) {
	type dom struct{ name, glob string }
	build := func(t *testing.T, doms ...dom) *Project {
		t.Helper()
		var cfg strings.Builder
		cfg.WriteString(uncoveredBase)
		side := map[string]string{}
		if len(doms) > 0 {
			cfg.WriteString("domains:\n")
			for _, d := range doms {
				cfg.WriteString("  - " + d.name + "\n")
				side["domains/"+d.name+".yaml"] = "paths:\n  - " + d.glob + "\n"
			}
		}
		root := scaffoldFiles(t, cfg.String(), side)
		p, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	join := func(e []string) string { return strings.Join(e, ",") }

	t.Run("collapse fully-uncovered subtree", func(t *testing.T) {
		p := build(t, dom{"render", "internal/render/**"})
		res, err := p.Uncovered([]string{"internal/render/r.go", "internal/plan/p.go", "internal/plan/q.go"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		// invariant: uncovered-collapses-directories
		if join(res.Entries) != "internal/plan/" {
			t.Errorf("got %v want [internal/plan/]", res.Entries)
		}
	})
	t.Run("stray top-level file reported individually", func(t *testing.T) {
		p := build(t, dom{"runner", "x"})
		res, err := p.Uncovered([]string{"x", "README.md"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if join(res.Entries) != "README.md" {
			t.Errorf("got %v want [README.md]", res.Entries)
		}
	})
	t.Run("mixed dir reports files individually", func(t *testing.T) {
		p := build(t, dom{"awf", "cmd/awf/main.go"})
		res, err := p.Uncovered([]string{"cmd/awf/main.go", "cmd/awf/other.go"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		// invariant: uncovered-lists-unowned-only
		if join(res.Entries) != "cmd/awf/other.go" {
			t.Errorf("got %v want [cmd/awf/other.go]", res.Entries)
		}
	})
	t.Run("zero covered collapses to root", func(t *testing.T) {
		p := build(t)
		res, err := p.Uncovered([]string{"a/b.go", "c.go"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if join(res.Entries) != "." {
			t.Errorf("got %v want [.]", res.Entries)
		}
	})
	t.Run("scan-root segment boundary", func(t *testing.T) {
		p := build(t)
		res, err := p.Uncovered([]string{"internal/git/g.go", "internal/gitlab/h.go"}, []string{"internal/git"})
		if err != nil {
			t.Fatal(err)
		}
		if join(res.Entries) != "internal/git/" {
			t.Errorf("got %v want [internal/git/] (gitlab sibling out of scope)", res.Entries)
		}
		if join(res.ScanRoots) != "internal/git" {
			t.Errorf("scanRoots: got %v want [internal/git]", res.ScanRoots)
		}
	})
	t.Run("sidecar fault", func(t *testing.T) {
		root, p := ctxProject(t, ctxYAML)
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "alpha.yaml"), "paths: [\n")
		if _, err := p.Uncovered([]string{"cmd/x.go"}, nil); err == nil {
			t.Error("expected a domain-sidecar parse error")
		}
	})
}
