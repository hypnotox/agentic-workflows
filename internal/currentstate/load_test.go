package currentstate_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// treeFrom builds a snapshot Tree from an in-memory path->content map so a load
// case can shape an exact universe without touching the filesystem.
func treeFrom(t *testing.T, files map[string]string) *snapshot.Tree {
	t.Helper()
	var fl []snapshot.File
	for p, c := range files {
		fl = append(fl, snapshot.File{Path: p, Mode: snapshot.Regular, Bytes: []byte(c)})
	}
	tree, err := snapshot.NewTree(fl)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func loadCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Parse("/nonexistent", []byte(loadCfgBody))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// legacyADR is a minimal markerless Implemented ADR: frontmatter status/date
// and a title.
func legacyADR() string {
	return "---\nstatus: Implemented\ndate: 2026-07-20\n---\n# Legacy decision\n"
}

// v1Scaffold is a valid Proposed current-state-v1 ADR with a None state change.
func v1Scaffold() string {
	return "---\nformat: current-state-v1\nstatus: Proposed\ndate: 2026-07-20\n---\n" +
		"# A decision\n\n" +
		"## Context\n\nBackground prose.\n\n" +
		"## Decision\n\n1. The only decision.\n\n" +
		"## State changes\n\nNone.\n\n" +
		"## Consequences\n\nConsequence prose.\n\n" +
		"## Alternatives Considered\n\nNone considered.\n\n" +
		"## Status history\n\n- 2026-07-20: Proposed\n"
}

// ruleTopicPart is a one-claim current-state part whose rule cites an Implemented
// Origin ADR, so the provenance graph accepts it.
func ruleTopicPart(origin string) string {
	return "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-" + origin + "\n"
}

const loadCfgBody = "prefix: test\ndomains: [alpha]\n"

// TestLoadFromTreeAssembles loads a mixed legacy/v1 decisions set plus one topic
// from a single tree, proving the ADR walk, the v1 route, and topic assembly all
// read the same universe. It also proves non-ADR and nested decision files are
// skipped.
func TestLoadFromTreeSkipsSymlinkADR(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: "docs/decisions/0001-first.md", Mode: snapshot.Regular, Bytes: []byte(legacyADR())}, {Path: "docs/decisions/0002-link.md", Mode: snapshot.Symlink, Bytes: []byte("../bad")}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := currentstate.LoadFromTree(tree, loadCfg(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ADRs) != 1 || loaded.ADRs[0].Number != "0001" {
		t.Fatalf("ADRs=%#v", loaded.ADRs)
	}
}

func TestLoadFromTreeAssembles(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		"docs/decisions/0001-first.md":                 legacyADR(),
		"docs/decisions/0002-second.md":                v1Scaffold(),
		"docs/decisions/0003-third.md":                 strings.Replace(v1Scaffold(), adr.V1FormatMarker, adr.V2FormatMarker, 1),
		"docs/decisions/README.md":                     "# Index\n",
		"docs/decisions/nested/0009-ignored.md":        legacyADR(),
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart("0001"),
	})
	got, err := currentstate.LoadFromTree(tree, loadCfg(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ADRs) != 3 {
		t.Fatalf("ADRs = %d, want 3 (README and nested skipped)", len(got.ADRs))
	}
	if !got.ADRs[1].IsV1() || !got.ADRs[2].IsV2() {
		t.Errorf("mixed boundaries routed formats as %v and %v", got.ADRs[1].Format, got.ADRs[2].Format)
	}
	if len(got.Topics.All()) != 1 {
		t.Fatalf("topics = %d, want 1", len(got.Topics.All()))
	}
	if _, ok := got.Topics.ByClaimID("alpha/one:r"); !ok {
		t.Error("claim alpha/one:r missing from assembled corpus")
	}
}

// TestLoadFromTreeEmpty proves an empty decisions set and empty topic set yield a
// clean empty view rather than a contiguity failure.
func TestLoadFromTreeEmpty(t *testing.T) {
	tree := treeFrom(t, map[string]string{"docs/decisions/README.md": "# Index\n"})
	got, err := currentstate.LoadFromTree(tree, loadCfg(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ADRs) != 0 || len(got.Topics.All()) != 0 {
		t.Fatalf("view = {adrs:%d topics:%d}, want empty", len(got.ADRs), len(got.Topics.All()))
	}
}

// TestLoadFromTreeDoesNotRequireContiguousNumbers proves format routing no
// longer needs a lock-recorded gap set.
func TestLoadFromTreeDoesNotRequireContiguousNumbers(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		"docs/decisions/0001-first.md": legacyADR(),
		"docs/decisions/0003-third.md": legacyADR(),
	})
	loaded, err := currentstate.LoadFromTree(tree, loadCfg(t))
	if err != nil {
		t.Fatalf("LoadFromTree: %v", err)
	}
	if len(loaded.ADRs) != 2 {
		t.Fatalf("ADRs = %d, want 2", len(loaded.ADRs))
	}
}

// TestLoadFromTreeADRParseError propagates a malformed authored format.
func TestLoadFromTreeADRParseError(t *testing.T) {
	tree := treeFrom(t, map[string]string{"docs/decisions/0001-a.md": "---\nformat: unknown\n---\n# ADR-0001: Invalid\n"})
	_, err := currentstate.LoadFromTree(tree, loadCfg(t))
	if err == nil {
		t.Fatal("expected a parse error for an unknown authored format")
	}
}

// TestLoadFromTreeTopicError propagates a topic-assembly failure while the ADR
// set is well formed, proving the two loaders compose without masking either.
func TestLoadFromTreeTopicError(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		"docs/decisions/0001-a.md":            legacyADR(),
		".awf/topics/metadata/alpha/one.yaml": "title: [unterminated\n",
	})
	_, err := currentstate.LoadFromTree(tree, loadCfg(t))
	if err == nil {
		t.Fatal("expected a topic metadata parse error")
	}
}

// TestLoadUniverseFromSelectionMatchesPolicyProjection proves the sparse
// authority selection yields the same transition universe as the complete-tree
// reduced parser, while malformed marker sources remain outside its boundary.
func TestLoadUniverseFromSelectionMatchesPolicyProjection(t *testing.T) {
	cfg, err := config.Parse("/nonexistent", []byte(loadCfgBody+"currentState:\n  sources:\n    - globs: [\"internal/**/*_test.go\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"docs/decisions/0001-first.md":                 legacyADR(),
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart("0001"),
		"internal/proof_test.go":                       "package internal\n// invariant: alpha/one:missing (TestMissing)\nfunc TestMissing() {}\n",
	}
	complete := treeFrom(t, files)
	want, err := loadUniverseFromTree(complete, cfg)
	if err != nil {
		t.Fatalf("LoadUniverseFromTree: %v", err)
	}
	selection, err := snapshot.NewSelection([]snapshot.File{
		{Path: "docs/decisions/0001-first.md", Mode: snapshot.Regular, Bytes: []byte(files["docs/decisions/0001-first.md"])},
		{Path: ".awf/topics/metadata/alpha/one.yaml", Mode: snapshot.Regular, Bytes: []byte(files[".awf/topics/metadata/alpha/one.yaml"])},
		{Path: ".awf/topics/parts/alpha/one/current-state.md", Mode: snapshot.Regular, Bytes: []byte(files[".awf/topics/parts/alpha/one/current-state.md"])},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := currentstate.LoadUniverseFromSelection(selection, cfg)
	if err != nil {
		t.Fatalf("LoadUniverseFromSelection: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selection universe = %#v, want complete reduced universe %#v", got, want)
	}
	if _, err := currentstate.LoadFromTree(complete, cfg); err == nil {
		t.Fatal("LoadFromTree stopped indexing malformed marker sources")
	}
}

// invariant: tooling/audit-and-snapshots:audit-history-policy-projection (TestLoadUniverseFromTreeMatchesPolicyProjection)
func TestLoadUniverseFromTreeMatchesPolicyProjection(t *testing.T) {
	cfg, err := config.Parse("/nonexistent", []byte(loadCfgBody+"currentState:\n  sources:\n    - globs: [\"internal/**/*_test.go\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	valid := treeFrom(t, map[string]string{
		"docs/decisions/0001-first.md":                 legacyADR(),
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart("0001"),
		".awf/domains/alpha.yaml":                      "paths: [\"internal/**\"]\n",
		"internal/proof_test.go":                       "package internal\n// invariant: alpha/one:missing (TestMissing)\nfunc TestMissing() {}\n",
	})
	_, err = currentstate.LoadFromTree(valid, cfg)
	if err == nil {
		t.Fatal("full loader accepted malformed marker source")
	}
	// The policy projection retains exactly the transition inputs that a valid
	// full view would expose: records, source bytes, and assembled topics.
	clean := treeFrom(t, map[string]string{
		"docs/decisions/0001-first.md":                 legacyADR(),
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart("0001"),
		".awf/domains/alpha.yaml":                      "paths: [\"internal/**\"]\n",
	})
	full, err := currentstate.LoadFromTree(clean, cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := full.Universe()
	got, err := loadUniverseFromTree(clean, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("policy universe = %#v, want %#v", got, want)
	}
	for _, docsDir := range []string{"./docs", "docs/"} {
		spelled := *cfg
		spelled.DocsDir = docsDir
		got, err := loadUniverseFromTree(clean, &spelled)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("docsDir %q policy universe = %#v, %v; want %#v", docsDir, got, err, want)
		}
	}
	for _, changed := range []map[string]string{
		{
			"docs/decisions/0001-first.md":                 legacyADR(),
			".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
			".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart("0001"),
			".awf/domains/alpha.yaml":                      "unknown: [\n",
		},
		{
			"docs/decisions/0001-first.md":                 legacyADR(),
			".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
			".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart("0001"),
			"internal/proof_test.go":                       "package internal\n// invariant: alpha/one:missing (TestMissing)\nfunc TestMissing() {}\n",
		},
	} {
		got, err := loadUniverseFromTree(treeFrom(t, changed), cfg)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("omitted bytes changed policy universe: %#v, %v", got, err)
		}
	}
	for name, broken := range map[string]map[string]string{
		"ADR": {
			"docs/decisions/0001-first.md": "---\nformat: unknown\n---\n# Invalid\n",
		},
		"topic metadata": {
			"docs/decisions/0001-first.md":                 legacyADR(),
			".awf/topics/metadata/alpha/one.yaml":          "title: [\n",
			".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart("0001"),
		},
		"topic part": {
			"docs/decisions/0001-first.md":        legacyADR(),
			".awf/topics/metadata/alpha/one.yaml": "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		},
		"claim provenance": {
			"docs/decisions/0001-first.md":                 legacyADR(),
			".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
			".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart("9999"),
		},
		"claim reference": {
			"docs/decisions/0001-first.md":                 legacyADR(),
			".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
			".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: r`\nRule.\nOrigin: ADR-0001\nReferences: alpha/missing:r\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadUniverseFromTree(treeFrom(t, broken), cfg); err == nil {
				t.Fatal("policy loader accepted malformed required authority")
			}
		})
	}
}

func loadUniverseFromTree(tree *snapshot.Tree, cfg *config.Config) (currentstate.Universe, error) {
	selection, err := snapshot.NewSelection(tree.List())
	if err != nil {
		return currentstate.Universe{}, err
	}
	return currentstate.LoadUniverseFromSelection(selection, cfg)
}

// v3Pending is a valid Proposed pending current-state-v3 record: slug identity,
// no number, and the slug-form heading.
func v3Pending(slug string) string {
	return "---\nformat: current-state-v3\nslug: " + slug + "\nstatus: Proposed\ndate: 2026-07-31\n---\n" +
		"# ADR-" + slug + ": A decision\n\n" +
		"## Context\n\nBackground prose.\n\n" +
		"## Decision\n\n1. The only decision.\n\n" +
		"## State changes\n\nNone.\n\n" +
		"## Consequences\n\nConsequence prose.\n\n" +
		"## Alternatives Considered\n\nNone considered.\n\n" +
		"## Status history\n\n- 2026-07-31: Proposed\n"
}

// A pending record joins the corpus without joining the number contiguity set,
// and a stray file under the decisions directory is a corpus error.
func TestLoadFromTreeCarriesPendingRecordsOutsideContiguity(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		"docs/decisions/0001-first.md": legacyADR(),
		"docs/decisions/pending-x.md":  v3Pending("pending-x"),
		"docs/decisions/README.md":     "# Decisions\n",
		"docs/decisions/INDEX.md":      "# Index\n",
		"docs/decisions/template.md":   "# Template\n",
		"docs/decisions/diagram.png":   "not markdown\n",
	})
	loaded, err := currentstate.LoadFromTree(tree, loadCfg(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ADRs) != 2 {
		t.Fatalf("records = %#v", loaded.ADRs)
	}
	if _, ok := loaded.Corpus.BySlug("pending-x"); !ok || !loaded.Corpus.Has("0001") {
		t.Fatal("loaded corpus indexes")
	}

	stray := treeFrom(t, map[string]string{"docs/decisions/notes.md": "# Notes\n"})
	if _, err := currentstate.LoadFromTree(stray, loadCfg(t)); err == nil ||
		!strings.Contains(err.Error(), "not an ADR record") {
		t.Fatalf("stray decisions file = %v", err)
	}

	duplicate := treeFrom(t, map[string]string{
		"docs/decisions/dupe.md":      v3Pending("dupe"),
		"docs/decisions/0001-dupe.md": strings.Replace(v3Pending("dupe"), "# ADR-dupe:", "# ADR-0001:", 1),
	})
	if _, err := currentstate.LoadFromTree(duplicate, loadCfg(t)); err == nil ||
		!strings.Contains(err.Error(), `ADR slug "dupe" is declared by more than one file`) {
		t.Fatalf("duplicate slug = %v", err)
	}
}
