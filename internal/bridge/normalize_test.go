package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func TestDeriveMappings(t *testing.T) {
	inv := Inventory{Entries: []LegacyInvariant{{Key: "ADR-0001#stable", Declarer: "0001", Slug: "stable", Backing: "test", Active: true}, {Key: "ADR-0002#manual", Declarer: "0002", Slug: "manual", Backing: "unbacked", Active: true}, {Key: "ADR-0003#gone", Active: false}}}
	claim := topic.Claim{ID: "core/contracts:stable", Slug: "stable", Type: topic.Invariant, Origin: "0001", Backing: topic.TestBacking}
	manual := topic.Claim{ID: "core/contracts:manual", Slug: "manual", Type: topic.Invariant, Origin: "0002", Backing: topic.Unbacked}
	unrelated := topic.Claim{ID: "core/contracts:rule", Slug: "stable", Type: topic.Rule, Origin: "0001"}
	got, err := DeriveMappings(inv, []topic.Topic{{Claims: []topic.Claim{manual, unrelated, claim}}})
	if err != nil || len(got) != 2 || got[0].Destination != claim.ID {
		t.Fatalf("%#v %v", got, err)
	}
	one := Inventory{Entries: inv.Entries[:1]}
	for _, topics := range [][]topic.Topic{nil, {{Claims: []topic.Claim{claim, claim}}}, {{Claims: []topic.Claim{{ID: "core/x:stable", Slug: "stable", Type: topic.Invariant, Origin: "0001", Backing: topic.Unbacked}}}}} {
		if _, err := DeriveMappings(one, topics); err == nil {
			t.Errorf("expected mapping refusal for %#v", topics)
		}
	}
}

func TestPlanNormalization(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	configBytes := []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  sources:\n    - globs: ['src/**']\n      marker: //\n  testGlobs: ['src/**']\n")
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), configBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "- `invariant: stable` - x.\n- `invariant: alpha` - x.", "1. x.", "")
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0002-two.md", "Superseded", "", "1. retire `supersedes-invariant: ADR-0001#stable`.\n2. retire `supersedes-invariant: ADR-0001#alpha`.", "")
	carrierPath := filepath.Join(root, "docs", "decisions", "0002-two.md")
	carrier, _ := os.ReadFile(carrierPath)
	if err := os.WriteFile(carrierPath, []byte(strings.TrimRight(string(carrier), "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "x.go"), []byte("package x\n// invariant: stable\n// touches-invariant: stable - relevant\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, ".awf"))
	if err != nil {
		t.Fatal(err)
	}
	if err = cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	corpus := bridgeCorpus(t, filepath.Join(root, "docs", "decisions"))
	inventory, err := BuildInventory(corpus)
	if err != nil {
		t.Fatal(err)
	}
	mappings := []Mapping{{Key: "ADR-0001#stable", Destination: "core/contracts:stable", Approved: true}}
	mutations, err := PlanNormalization(root, cfg, corpus, inventory, mappings)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 3 {
		t.Fatalf("mutations: %#v", mutations)
	}
	for _, m := range mutations {
		switch m.Path {
		case ".awf/config.yaml":
			if !strings.Contains(string(m.After), "currentState:") {
				t.Error("config not converted")
			}
		case "docs/decisions/0002-two.md":
			if !strings.Contains(string(m.After), "status: Implemented") || strings.Count(string(m.After), "basis: encoded") != 2 {
				t.Errorf("ADR not normalized:\n%s", m.After)
			}
		case "src/x.go":
			if !strings.Contains(string(m.After), "invariant: core/contracts:stable") || !strings.Contains(string(m.After), "touches-state: core/contracts:stable - relevant") {
				t.Errorf("markers not rewritten:\n%s", m.After)
			}
		}
	}
	for _, m := range mutations {
		if err := applyMutation(root, m); err != nil {
			t.Fatal(err)
		}
	}
	cfg2, _ := config.Load(filepath.Join(root, ".awf"))
	corpus2 := bridgeCorpus(t, filepath.Join(root, "docs", "decisions"))
	inventory2, err := BuildInventory(corpus2)
	if err != nil {
		t.Fatal(err)
	}
	again, err := PlanNormalization(root, cfg2, corpus2, inventory2, mappings)
	if err != nil || len(again) != 0 {
		t.Fatalf("idempotence: %#v %v", again, err)
	}
}

func TestPlanNormalizationRefusesADRMetadata(t *testing.T) {
	badCfg, _ := config.Parse("", []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  disabled: true\n"))
	if _, err := PlanNormalization(t.TempDir(), badCfg, adr.NewCorpus(nil), Inventory{}, nil); err == nil {
		t.Fatal("disabled config converted")
	}
	for _, tc := range []struct{ date, status, want string }{{"bad", "Implemented", "valid frontmatter date"}, {"2026-07-19", "\"Superseded\"", "not canonical"}} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
			t.Fatal(err)
		}
		dec := filepath.Join(root, "docs", "decisions")
		if err := os.MkdirAll(dec, 0o755); err != nil {
			t.Fatal(err)
		}
		src := []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  sources: []\n")
		if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), src, 0o644); err != nil {
			t.Fatal(err)
		}
		body := "---\nstatus: " + tc.status + "\ndate: " + tc.date + "\n---\n# ADR-0001: X\n\n## Decision\n\n1. retire `supersedes-invariant: ADR-0002#x`.\n"
		if err := os.WriteFile(filepath.Join(dec, "0001-x.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		writeBridgeADR(t, dec, "0002-two.md", "Implemented", "- `invariant: x` - x.", "1. x.", "")
		cfg, _ := config.Load(filepath.Join(root, ".awf"))
		if err := cfg.Validate(); err != nil {
			t.Fatal(err)
		}
		corpus := bridgeCorpus(t, dec)
		inv, err := BuildInventory(corpus)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := PlanNormalization(root, cfg, corpus, inv, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", tc.want, err)
		}
	}
}

func TestMarkerRewriteCloseAndNested(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "nested", ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "x.md"), []byte("<!-- ordinary text\n<!-- invariant: x -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "x.md"), []byte("<!-- invariant: x -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"**/*.md"}, Marker: "<!--", Close: "-->"}}}
	got, err := planMarkerRewrites(root, cfg, []Mapping{{Key: "ADR-0001#x", Destination: "core/x:x"}})
	if err != nil || len(got) != 1 || !strings.Contains(string(got[0].After), "core/x:x -->") {
		t.Fatalf("%#v %v", got, err)
	}
	if _, err := planMarkerRewrites(root, cfg, []Mapping{{Key: "ADR-0001#x", Destination: "a/x:x"}, {Key: "ADR-0002#x", Destination: "b/x:x"}}); err == nil {
		t.Fatal("ambiguous slug accepted")
	}
}

func TestMarkerRewriteRefusals(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"src/**"}, Marker: "//"}}}
	mapping := []Mapping{{Key: "ADR-0001#x", Destination: "core/x:x"}}
	for _, body := range []string{"// touches-invariant: x\n", "// touches-invariant: unknown - note\n", "// invariant: unknown\n"} {
		if err := os.WriteFile(filepath.Join(root, "src", "x.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := planMarkerRewrites(root, cfg, mapping); err == nil {
			t.Errorf("accepted %q", body)
		}
	}
	if _, err := planMarkerRewrites(root, nil, mapping); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "x.go"), []byte("<!-- invariant: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := planMarkerRewrites(root, &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"src/**"}, Marker: "<!--", Close: "-->"}}}, mapping); err == nil {
		t.Fatal("missing close accepted")
	}
}
