package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const rtCfg = "prefix: example\nskills: []\nagents: []\n"

// rtTarget declares fixture-gone; its related: is the back-pointer surface.
const rtTarget = `---
status: Superseded by ADR-0002
superseded_by: "0002"
related: []
---
# ADR-0001: Target

## Decision

1. Old thing.

## Invariants

- ` + "`invariant: fixture-gone`" + ` - x.
`

// rtCarrier retires fixture-gone via the legacy frontmatter key.
const rtCarrier = `---
status: Implemented
retires_invariants: [fixture-gone]
related: []
---
# ADR-0002: Carrier

## Context

x

## Decision

1. Something.

## Consequences

c
`

// rtFixture writes a config plus the given decisions files and returns the root.
func rtFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, rtCfg)
	for name, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", name), body)
	}
	return root
}

// TestRetirementTokensMigratesCorpus covers the happy path end to end: the key
// stripped, the bookkeeping Decision item appended without renumbering, the
// target's related: back-pointer inserted, and the provenance lines printed
// exactly.
// invariant: upgrade-migrates-retirements
func TestRetirementTokensMigratesCorpus(t *testing.T) {
	root := rtFixture(t, map[string]string{
		"0001-target.md":  rtTarget,
		"0002-carrier.md": rtCarrier,
	})
	var buf bytes.Buffer
	if err := applyRetirementTokens(root, &buf); err != nil {
		t.Fatalf("applyRetirementTokens: %v", err)
	}
	wantOut := "retirement-tokens: 0002-carrier.md: stripped retires_invariants\n" +
		"retirement-tokens: 0002-carrier.md: appended Decision item 2 (fixture-gone)\n" +
		"retirement-tokens: 0001-target.md: related: gains 2 (back-pointer for ADR-0002)\n"
	if buf.String() != wantOut {
		t.Errorf("output:\n%s\nwant:\n%s", buf.String(), wantOut)
	}
	carrier, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0002-carrier.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(carrier), "\nretires_invariants:") {
		t.Errorf("key not stripped:\n%s", carrier)
	}
	wantItem := "1. Something.\n\n2. **Retirement bookkeeping (migrated from retires_invariants by awf upgrade,\n" +
		"   ADR-0120).** This ADR retires `supersedes-invariant: ADR-0001#fixture-gone`.\n\n## Consequences"
	if !strings.Contains(string(carrier), wantItem) {
		t.Errorf("bookkeeping item missing or misplaced:\n%s", carrier)
	}
	target, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-target.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(target), "related: [2]") {
		t.Errorf("back-pointer not inserted:\n%s", target)
	}
}

func TestRetirementTokensAppendsAfterFencedSyntax(t *testing.T) {
	carrier := strings.Replace(rtCarrier, "1. Something.", "```\n## Fake heading\n2. Fake item.\n```\n\n1. Something.\n2. Also real.", 1)
	root := rtFixture(t, map[string]string{
		"0001-target.md":  rtTarget,
		"0002-carrier.md": carrier,
	})
	if err := applyRetirementTokens(root, io.Discard); err != nil {
		t.Fatalf("applyRetirementTokens: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0002-carrier.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "2. Also real.\n\n3. **Retirement bookkeeping (migrated from retires_invariants by awf upgrade,\n   ADR-0120).** This ADR retires `supersedes-invariant: ADR-0001#fixture-gone`.\n\n## Consequences"
	if !strings.Contains(string(got), want) {
		t.Errorf("bookkeeping item missing or misplaced:\n%s", got)
	}
}

// An empty key is stripped without appending an item; a target already
// back-pointed gains no duplicate; existing related: order is preserved with
// the carrier appended last.
// invariant: upgrade-migrates-retirements
func TestRetirementTokensEmptyKeyAndExistingBackpointer(t *testing.T) {
	t.Run("empty key stripped, no item", func(t *testing.T) {
		root := rtFixture(t, map[string]string{
			"0001-solo.md": "---\nstatus: Implemented\nretires_invariants: []\nrelated: []\n---\n# ADR-0001: Solo\n\n## Decision\n\n1. x.\n",
		})
		var buf bytes.Buffer
		if err := applyRetirementTokens(root, &buf); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "retirement-tokens: 0001-solo.md: stripped retires_invariants\n" {
			t.Errorf("output: %q", buf.String())
		}
		b, _ := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-solo.md"))
		if strings.Contains(string(b), "retires_invariants") || strings.Contains(string(b), "Retirement bookkeeping") {
			t.Errorf("empty key must strip without an item:\n%s", b)
		}
	})
	t.Run("existing back-pointer preserved, no duplicate", func(t *testing.T) {
		target := strings.Replace(rtTarget, "related: []", "related: [7, 2]", 1)
		root := rtFixture(t, map[string]string{
			"0001-target.md":  target,
			"0002-carrier.md": rtCarrier,
		})
		var buf bytes.Buffer
		if err := applyRetirementTokens(root, &buf); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(buf.String(), "related: gains") {
			t.Errorf("no back-pointer op expected, got:\n%s", buf.String())
		}
		b, _ := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-target.md"))
		if !strings.Contains(string(b), "related: [7, 2]") {
			t.Errorf("existing related: must survive byte-identical:\n%s", b)
		}
	})
	t.Run("carrier appended last to a non-empty list", func(t *testing.T) {
		target := strings.Replace(rtTarget, "related: []", "related: [7]", 1)
		root := rtFixture(t, map[string]string{
			"0001-target.md":  target,
			"0002-carrier.md": rtCarrier,
		})
		if err := applyRetirementTokens(root, io.Discard); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-target.md"))
		if !strings.Contains(string(b), "related: [7, 2]") {
			t.Errorf("carrier must append last:\n%s", b)
		}
	})
}

// A target that is itself a carrier is edited twice - the back-pointer pass
// must read the stripped content, not the on-disk original - and a Decision
// section with no following heading appends at end of file.
// invariant: upgrade-migrates-retirements
func TestRetirementTokensCarrierTargetAndTrailingDecision(t *testing.T) {
	// 0001 declares fixture-gone and itself retires fixture-old (declared by
	// 0003); 0002 retires fixture-gone. 0001's Decision is the last section.
	root := rtFixture(t, map[string]string{
		"0001-target.md": "---\nstatus: Implemented\nretires_invariants: [fixture-old]\nrelated: []\n---\n" +
			"# ADR-0001: Target\n\n## Invariants\n\n- `invariant: fixture-gone` - x.\n\n## Decision\n\n1. Old thing.\n",
		"0002-carrier.md": rtCarrier,
		"0003-elder.md":   "---\nstatus: Superseded by ADR-0001\nsuperseded_by: \"0001\"\nrelated: [1]\n---\n# ADR-0003: Elder\n\n## Decision\n\n1. x.\n\n## Invariants\n\n- `invariant: fixture-old` - x.\n",
	})
	if err := applyRetirementTokens(root, io.Discard); err != nil {
		t.Fatalf("applyRetirementTokens: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-target.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if strings.Contains(got, "\nretires_invariants:") {
		t.Errorf("carrier-target's key must be stripped:\n%s", got)
	}
	if !strings.HasSuffix(got, "This ADR retires `supersedes-invariant: ADR-0003#fixture-old`.\n") {
		t.Errorf("trailing Decision must gain the item at end of file:\n%s", got)
	}
	if !strings.Contains(got, "related: [2]") {
		t.Errorf("back-pointer must land on the already-edited content:\n%s", got)
	}
}

// A re-run after migration prints nothing and leaves the corpus byte-identical.
// invariant: upgrade-migrates-retirements
func TestRetirementTokensIdempotent(t *testing.T) {
	root := rtFixture(t, map[string]string{
		"0001-target.md":  rtTarget,
		"0002-carrier.md": rtCarrier,
	})
	if err := applyRetirementTokens(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, name := range []string{"0001-target.md", "0002-carrier.md"} {
		b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = b
	}
	var buf bytes.Buffer
	if err := applyRetirementTokens(root, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("re-run must print nothing, got:\n%s", buf.String())
	}
	for name, want := range before {
		got, err := os.ReadFile(filepath.Join(root, "docs", "decisions", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed on re-run:\n%s", name, got)
		}
	}
}

// The loud-failure paths: an unresolvable slug, a multi-declared slug, a
// multi-line key, a carrier without a Decision section, and a target without a
// related: line each fail naming the offender.
// invariant: upgrade-migrates-retirements
func TestRetirementTokensLoudFailures(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		wantErr []string
	}{
		{
			name: "unresolvable slug",
			files: map[string]string{
				"0002-carrier.md": strings.Replace(rtCarrier, "fixture-gone", "fixture-ghost", 1),
			},
			wantErr: []string{"0002-carrier.md", `"fixture-ghost"`, "declared by no ADR"},
		},
		{
			name: "multi-declared slug",
			files: map[string]string{
				"0001-target.md":  rtTarget,
				"0002-carrier.md": rtCarrier,
				"0003-dup.md":     "---\nstatus: Proposed\nrelated: []\n---\n# ADR-0003: Dup\n\n## Decision\n\n1. x.\n\n## Invariants\n\n- `invariant: fixture-gone` - x.\n",
			},
			wantErr: []string{"0002-carrier.md", `"fixture-gone"`, "ADR-0001 and ADR-0003"},
		},
		{
			name: "multi-line key",
			files: map[string]string{
				"0002-carrier.md": strings.Replace(rtCarrier, "retires_invariants: [fixture-gone]", "retires_invariants:\n  - fixture-gone", 1),
			},
			wantErr: []string{"0002-carrier.md", "not a single-line inline list"},
		},
		{
			name: "carrier without a Decision section",
			files: map[string]string{
				"0001-target.md":  rtTarget,
				"0002-carrier.md": "---\nstatus: Implemented\nretires_invariants: [fixture-gone]\n---\n# ADR-0002: Carrier\n\n## Context\n\nx\n",
			},
			wantErr: []string{"0002-carrier.md", "no Decision section"},
		},
		{
			name: "target without a related: line",
			files: map[string]string{
				"0001-target.md":  strings.Replace(rtTarget, "related: []\n", "", 1),
				"0002-carrier.md": rtCarrier,
			},
			wantErr: []string{"0001-target.md", "no related: line"},
		},
		{
			name: "a body related: line does not stand in for the frontmatter key",
			files: map[string]string{
				"0001-target.md": strings.Replace(rtTarget, "related: []\n", "", 1) +
					"\nA quoted example:\n\nrelated: [9]\n",
				"0002-carrier.md": rtCarrier,
			},
			wantErr: []string{"0001-target.md", "no related: line"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := rtFixture(t, tc.files)
			err := applyRetirementTokens(root, io.Discard)
			if err == nil {
				t.Fatal("expected a loud failure, got nil")
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q missing %q", err, want)
				}
			}
		})
	}
}

// The no-op and error plumbing paths: no config tree, no decisions dir, a
// malformed config, and a malformed ADR.
func TestRetirementTokensNoOpAndErrorPaths(t *testing.T) {
	t.Run("no config is a no-op", func(t *testing.T) {
		var buf bytes.Buffer
		if err := applyRetirementTokens(t.TempDir(), &buf); err != nil || buf.Len() != 0 {
			t.Fatalf("want silent no-op, got err=%v out=%q", err, buf.String())
		}
	})
	t.Run("no decisions dir is a no-op", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, rtCfg)
		var buf bytes.Buffer
		if err := applyRetirementTokens(root, &buf); err != nil || buf.Len() != 0 {
			t.Fatalf("want silent no-op, got err=%v out=%q", err, buf.String())
		}
	})
	t.Run("malformed config errors", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, "prefix: [unterminated\n")
		if err := applyRetirementTokens(root, io.Discard); err == nil {
			t.Fatal("expected config load error")
		}
	})
	t.Run("malformed ADR errors", func(t *testing.T) {
		root := rtFixture(t, map[string]string{
			"0001-broken.md": "---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n",
		})
		if err := applyRetirementTokens(root, io.Discard); err == nil {
			t.Fatal("expected adr.ParseDir error")
		}
	})
}
