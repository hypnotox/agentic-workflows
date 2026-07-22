package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// checkDrift opens, syncs, and checks a scaffolded root, returning the drift.
func checkDrift(t *testing.T, root string) []manifest.Drift {
	t.Helper()
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	return drift
}

// driftOfKind filters drift entries to one Kind.
func driftOfKind(drift []manifest.Drift, kind string) []manifest.Drift {
	var out []manifest.Drift
	for _, d := range drift {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

// invariant: rendering/inplace-and-placeholders:unused-var-drift
func TestCheckFlagsUnusedVar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars:\n  bogusVar: set-but-dead\nskills:\n  - tdd\nagents: []\n", nil)
	hits := driftOfKind(checkDrift(t, root), "unused-var")
	if len(hits) != 1 || hits[0].Path != ".awf/config.yaml" || !strings.Contains(hits[0].Detail, `"bogusVar"`) {
		t.Fatalf("want one unused-var entry for bogusVar at .awf/config.yaml, got %#v", hits)
	}
}

func TestCheckIgnoresEmptyVar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars:\n  bogusVar: \"\"\nskills:\n  - tdd\nagents: []\n", nil)
	if hits := driftOfKind(checkDrift(t, root), "unused-var"); len(hits) != 0 {
		t.Fatalf("an empty var is the seeded open-to-do state (ADR-0022; unset-var-note territory per ADR-0087) and must not flag, got %#v", hits)
	}
}

// The {{=awf:checkCmd}} placeholder channel: no assembled source references
// .vars.checkCmd in this fixture (refactor-coupling-audit's template does not,
// agents-doc and workflow are local, hooks/bootstrap are off), so only the
// PartVarRefs plumbing can mark the var consumed. (gateCmd can no longer serve
// this role: the always-on plans-template singleton now references .vars.gateCmd
// - ADR-0108 - so it is consumed in every project.)
func TestPartPlaceholderConsumesVar(t *testing.T) {
	cfg := "prefix: example\nvars:\n  checkCmd: awf check\nskills:\n  - exploring\n  - refactor-coupling-audit\nagents: []\n"
	locals := map[string]string{
		"agents-doc.yaml": "local: true\n",
		"workflow.yaml":   "local: true\n",
	}

	with := map[string]string{"skills/parts/refactor-coupling-audit/notes.md": "Run {{=awf:checkCmd}} before committing.\n"}
	for k, v := range locals {
		with[k] = v
	}
	if hits := driftOfKind(checkDrift(t, scaffoldFiles(t, cfg, with)), "unused-var"); len(hits) != 0 {
		t.Fatalf("checkCmd is consumed via the part placeholder and must not flag, got %#v", hits)
	}

	// Negative control: the same fixture without the part proves the channel.
	if hits := driftOfKind(checkDrift(t, scaffoldFiles(t, cfg, locals)), "unused-var"); len(hits) != 1 || !strings.Contains(hits[0].Detail, `"checkCmd"`) {
		t.Fatalf("without the part, checkCmd must flag as unused, got %#v", hits)
	}
}

// The bare-reference escapes cannot fire through shipped templates (none uses
// a selector-free .vars/.data form), so the producers are exercised directly.
func TestBareReferenceEscapes(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars:\n  bogusVar: set-but-dead\nskills:\n  - tdd\nagents: []\n", map[string]string{
		"skills/tdd.yaml": "data:\n  dead: v\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if hits := p.unusedVarDrift([]RenderedFile{{assembled: "{{ range .vars }}{{ . }}{{ end }}"}}); len(hits) != 0 {
		t.Fatalf("a bare .vars reference must consume every var, got %#v", hits)
	}
	bare := []RenderedFile{{kind: "skills", artifact: "tdd", assembled: "{{ range .data }}{{ . }}{{ end }}"}}
	hits, err := p.unusedDataDrift(bare)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("a bare .data reference must consume every data key, got %#v", hits)
	}
}

// invariant: rendering/inplace-and-placeholders:unused-data-drift
func TestCheckFlagsUnusedDataKey(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - tdd\nagents: []\n", map[string]string{
		"skills/tdd.yaml": "data:\n  testSurfaces:\n    - {name: One, location: a, kind: b}\n  dead: v\n",
	})
	hits := driftOfKind(checkDrift(t, root), "unused-data")
	if len(hits) != 1 || hits[0].Path != ".awf/skills/tdd.yaml" {
		t.Fatalf("want one unused-data entry at .awf/skills/tdd.yaml, got %#v", hits)
	}
	if !strings.Contains(hits[0].Detail, "dead") || strings.Contains(hits[0].Detail, "testSurfaces") {
		t.Fatalf("detail must name only the dead key, got %q", hits[0].Detail)
	}
}

func TestCheckFlagsLocalSidecarData(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - tdd\nagents: []\n", map[string]string{
		"skills/tdd.yaml": "local: true\ndata:\n  k: v\n",
	})
	// A local: true artifact's file is hand-maintained and must exist on disk.
	testsupport.WriteFile(t, filepath.Join(root, ".claude/skills/example-tdd/SKILL.md"),
		"---\nname: example-tdd\ndescription: hand-maintained\n---\nbody\n")
	hits := driftOfKind(checkDrift(t, root), "unused-data")
	if len(hits) != 1 || !strings.Contains(hits[0].Detail, "local: true renders nothing") {
		t.Fatalf("want the local-sidecar detail, got %#v", hits)
	}
}

func TestCheckFlagsDropShadowedDataKey(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - tdd\nagents: []\n", map[string]string{
		"skills/tdd.yaml": "data:\n  testSurfaces:\n    - {name: One, location: a, kind: b}\nsections:\n  surfaces:\n    drop: true\n",
	})
	hits := driftOfKind(checkDrift(t, root), "unused-data")
	if len(hits) != 1 || !strings.Contains(hits[0].Detail, "testSurfaces") || !strings.Contains(hits[0].Detail, "dropped section") {
		t.Fatalf("a key referenced only in a dropped section counts as unused, got %#v", hits)
	}
}

func TestCheckFlagsUnusedSingletonDataKey(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - tdd\nagents: []\n", map[string]string{
		"agents-doc.yaml": "data:\n  deadKey: v\n",
	})
	hits := driftOfKind(checkDrift(t, root), "unused-data")
	if len(hits) != 1 || hits[0].Path != ".awf/agents-doc.yaml" || !strings.Contains(hits[0].Detail, "deadKey") {
		t.Fatalf("want one unused-data entry at .awf/agents-doc.yaml naming deadKey, got %#v", hits)
	}
}

// The domain-doc placeholder channel: domain docs render outside RenderAll and
// their RenderedFile copy is hand-built (generateDomainDocs), so this pins the
// partVarRefs field preservation - re-stripping it would silently reintroduce
// false unused-var drift with every other test green. (assembled is retained
// for the var-consumption union too, but is unexercisable here: the domain
// template reads no vars.)
func TestDomainPartPlaceholderConsumesVar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars:\n  gateCmd: ./x gate\nskills: []\nagents: []\ndomains:\n  - config\n", map[string]string{
		"domains/parts/config/current-state.md": "Run {{=awf:gateCmd}} before committing.\n",
		"agents-doc.yaml":                       "local: true\n",
		"workflow.yaml":                         "local: true\n",
	})
	if hits := driftOfKind(checkDrift(t, root), "unused-var"); len(hits) != 0 {
		t.Fatalf("gateCmd is consumed via the domain part placeholder and must not flag, got %#v", hits)
	}
}

// A var-reading placeholder appearing only inside an authoring comment never
// renders, so PartVarRefs must not mark the var consumed: it flags unused
// (ADR-0121 Decision 2; the stripped counterpart of
// TestPartPlaceholderConsumesVar, same fixture).
func TestCommentWrappedPlaceholderDoesNotConsumeVar(t *testing.T) {
	cfg := "prefix: example\nvars:\n  checkCmd: awf check\nskills:\n  - exploring\n  - refactor-coupling-audit\nagents: []\n"
	with := map[string]string{
		"agents-doc.yaml": "local: true\n",
		"workflow.yaml":   "local: true\n",
		"skills/parts/refactor-coupling-audit/notes.md": "<!-- awf:comment run {{=awf:checkCmd}} first -->\nplain notes.\n",
	}
	hits := driftOfKind(checkDrift(t, scaffoldFiles(t, cfg, with)), "unused-var")
	if len(hits) != 1 || !strings.Contains(hits[0].Detail, `"checkCmd"`) {
		t.Fatalf("a comment-wrapped placeholder must not consume checkCmd, want one unused-var hit, got %#v", hits)
	}
}
