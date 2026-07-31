package project

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func TestLoadTreeCurrentStateRejectsFutureSchema(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: example\n")}})
	if err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{SchemaVersion: migrate.Current() + 1}
	if _, _, err := loadTreeCurrentState(".", tree, lock, adr.FormatBoundaries{}, nil); err == nil || !strings.Contains(err.Error(), "ahead of current") {
		t.Fatalf("future schema current-state load error = %v", err)
	}
}

func TestSnapshotAuthorityRejectsSymlinkConfigAndLock(t *testing.T) {
	lockTree, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/awf.lock", Mode: snapshot.Symlink, Bytes: []byte("target")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockFromTree(lockTree); err == nil {
		t.Fatal("staged symlink lock accepted")
	}
	if _, found, err := optionalLockFromTree(lockTree); !found || err == nil {
		t.Fatal("optional symlink lock accepted")
	}
	configTree, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/config.yaml", Mode: snapshot.Symlink, Bytes: []byte("target")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadTreeCurrentState(".", configTree, nil, adr.FormatBoundaries{}, nil); err == nil {
		t.Fatal("symlink config accepted")
	}
	if _, err := nextADRIdentityFromTree(configTree); err == nil {
		t.Fatal("symlink cutoff config accepted")
	}
	ordinary, _ := snapshot.NewTree([]snapshot.File{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: x\n")}, {Path: "docs/decisions/0001-link.md", Mode: snapshot.Symlink, Bytes: []byte("bad")}})
	if next, err := nextADRIdentityFromTree(ordinary); err != nil || next != 1 {
		t.Fatalf("next=%d err=%v", next, err)
	}
	if got := eligiblePaths(configTree, nil, nil); len(got) != 0 {
		t.Fatalf("eligible symlink=%v", got)
	}
	reader := configSnapshotReader{tree: ordinary}
	b, ok := reader.ReadFile("config.yaml")
	if !ok || len(b) == 0 {
		t.Fatal("snapshot config read")
	}
	b[0] = 'X'
	again, _ := reader.ReadFile("config.yaml")
	if again[0] == 'X' {
		t.Fatal("snapshot config alias")
	}
	if _, ok := reader.ReadFile("missing"); ok {
		t.Fatal("missing config read")
	}
	if got := reader.Paths(""); len(got) != 1 || got[0] != "config.yaml" {
		t.Fatalf("snapshot config paths=%v", got)
	}
}

func TestResidentPathsAreNeverEligibleOrNested(t *testing.T) {
	const adversarial = ".awf/efforts/e/.awf/config.yaml"
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: adversarial, Mode: snapshot.Regular, Bytes: []byte("prefix: nested\n")},
		{Path: "internal/owned.go", Mode: snapshot.Regular, Bytes: []byte("package internal\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := eligiblePaths(tree, nil, nil)
	if slices.Contains(got, adversarial) {
		t.Fatalf("resident path is eligible: %v", got)
	}
	if !slices.Contains(got, "internal/owned.go") {
		t.Fatalf("ordinary source was filtered: %v", got)
	}
	if !isResidentPath(adversarial) || isResidentPath(".awf/effort/other") {
		t.Fatal("resident path predicate is not closed to resident roots")
	}
}

// TestCurrentStateReportRouting proves the report splits into blocking findings
// (every static handshake message plus each error-severity coverage line) and
// non-failing notes (warn-severity coverage lines), rendering both coverage
// kinds.
// The claim's routing clause is marked here rather than named in prose from
// internal/currentstate, which cannot see CurrentStateReport.
// invariant: invariants/current-state-authority:currentstate-handshake-findings-unranked
func TestCurrentStateReportRouting(t *testing.T) {
	r := CurrentStateReport{
		Static: []currentstate.Finding{{Message: "handshake broke"}},
		Coverage: []topic.CoverageFinding{
			{Path: "internal/a.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error},
			{Path: "internal/b.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 3},
		},
	}
	findings := r.Findings()
	if len(findings) != 2 || findings[0] != "handshake broke" || !strings.Contains(findings[1], "internal/a.go is owned by domain alpha") {
		t.Fatalf("findings = %#v", findings)
	}
	notes := r.Notes()
	if len(notes) != 1 || !strings.Contains(notes[0], "internal/b.go is matched by 3 path-scoped topics") {
		t.Fatalf("notes = %#v", notes)
	}
}

// csNoPolicyYAML declares no currentState block. It is the base rather than a
// subtraction from csYAML on purpose: since ADR-0192 the two shapes produce
// identical coverage and fan-out findings, so a derivation that silently failed
// to strip the block would leave the no-policy tests green while exercising the
// wrong shape, and phase 2 puts a proof marker on exactly those tests. Deriving
// in this direction has no pattern to fall out of sync.
const csNoPolicyYAML = `prefix: example
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
contextIgnore:
  - internal/skip.go
`

// csYAML is csNoPolicyYAML plus the currentState block.
const csYAML = csNoPolicyYAML + "currentState:\n  maxTopicsPerPath: 8\n"

// csRuleTopic is a one-claim current-state part citing an Implemented Origin ADR.
const csRuleTopic = "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-0001\n"

// csRepo builds a git-backed project: a fresh repo, the given config, and the
// given working files (untracked but nonignored, so the working Tree includes
// them). It writes an Implemented ADR-0001 the topic can cite unless the caller
// supplies its own decisions file.
func csRepo(t *testing.T, cfg string, files map[string]string) *Project {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	// A base commit so the working Tree can resolve HEAD; the fixture files below
	// stay untracked-nonignored and are still part of the working universe.
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, cfg)
	if _, ok := files["docs/decisions/0001-first.md"]; !ok {
		files["docs/decisions/0001-first.md"] = testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First"),
			testsupport.WithBody("## Context\nx\n## Consequences\nc\n"))
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(dir, rel), body)
	}
	p, err := Open(testContext(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCheckCurrentState runs the working-tree check end to end: a covered path
// yields nothing, an owned-but-unscoped path yields one uncovered finding, and a
// generated (lock-listed) path and a contextIgnore path are both excluded. A
// sealed bridge attestation supplies the cutoff.
func TestCheckCurrentState(t *testing.T) {
	p := csRepo(t, csYAML, map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": csRuleTopic,
		"internal/foo/x.go":                            "package foo\n",
		"internal/bar.go":                              "package internalx\n",
		"internal/skip.go":                             "package internalx\n",
		"internal/gen.go":                              "package internalx\n",
	})
	lock := &manifest.Lock{
		AWFVersion: "0.18.0", SchemaVersion: 14,
		Files:             map[string]manifest.Entry{"internal/gen.go": {}},
		BridgeAttestation: &manifest.BridgeAttestation{Version: 1, PreparedHead: "x", TreeDigest: "sha256:x", ADRFormatV1From: 2, LegacyADRGaps: []int{}},
	}
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, lockFile(p.Root), string(b))

	report, err := p.CheckCurrentState(testContext(t))
	if err != nil {
		t.Fatalf("CheckCurrentState: %v", err)
	}
	findings := report.Findings()
	if len(findings) != 1 || !strings.Contains(findings[0], "internal/bar.go") {
		t.Fatalf("findings = %#v; want exactly the internal/bar.go uncovered finding", findings)
	}
	for _, excluded := range []string{"internal/foo/x.go", "internal/gen.go", "internal/skip.go"} {
		for _, f := range findings {
			if strings.Contains(f, excluded) {
				t.Errorf("%s should not be reported (covered, generated, or ignored)", excluded)
			}
		}
	}
	if len(report.Notes()) != 0 {
		t.Errorf("notes = %#v; want none", report.Notes())
	}
}

func TestCheckCurrentStateNoPolicy(t *testing.T) {
	cfg := "prefix: example\nskills: [tdd]\nagents: [code-reviewer]\ndomains: [alpha]\n"
	files := map[string]string{
		".awf/domains/alpha.yaml": "paths:\n  - internal/**\n",
		"internal/bar.go":         "package internalx\n",
		"internal/foo/x.go":       "package foo\n",
	}
	for i := 1; i <= 9; i++ {
		name := fmt.Sprintf("fan%d", i)
		files[".awf/topics/metadata/alpha/"+name+".yaml"] = fmt.Sprintf("title: Fan %d\nsummary: Fan-out fixture topic %d.\npaths:\n  - internal/foo/**\n", i, i)
		part := "Intro.\n\n## Claims\n"
		if i == 1 {
			part = csRuleTopic
		}
		files[".awf/topics/parts/alpha/"+name+"/current-state.md"] = part
	}
	p := csRepo(t, cfg, files)
	report, err := p.CheckCurrentState(testContext(t))
	if err != nil {
		t.Fatalf("CheckCurrentState: %v", err)
	}
	if report.Coverage == nil {
		t.Fatal("coverage = nil; want evaluation without a currentState policy")
	}
	want := []topic.CoverageFinding{
		{Path: "internal/bar.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error},
		{Path: "internal/foo/x.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 9},
	}
	if !reflect.DeepEqual(report.Coverage, want) {
		t.Fatalf("coverage:\n got %#v\nwant %#v", report.Coverage, want)
	}
}

// TestCheckCurrentStateOutsideRepo covers the working-Tree open failure: a
// scaffolded project that is not a git repository.
func TestCheckStagedRootOutsideRepo(t *testing.T) {
	if _, err := CheckStagedRoot(testContext(t), t.TempDir()); err == nil {
		t.Fatal("CheckStagedRoot accepted a non-repository")
	}
}

func TestCheckCurrentStateOutsideRepo(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [tdd]\nagents: [code-reviewer]\n", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.CheckCurrentState(testContext(t)); err == nil {
		t.Fatal("expected a working-tree error outside a git repository")
	}
}

// TestCheckCurrentStateCorruptLock covers the lock-read failure: a malformed
// awf.lock is not gated before this project method.
func TestCheckCurrentStateCorruptLock(t *testing.T) {
	p := csRepo(t, csYAML, map[string]string{".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"})
	testsupport.WriteFile(t, lockFile(p.Root), "{not json")
	if _, err := p.CheckCurrentState(testContext(t)); err == nil {
		t.Fatal("expected a lock parse error")
	}
}

// TestCheckCurrentStateLoadError propagates a corpus load failure: a decisions
// file that is not parseable.
func TestCheckCurrentStateLoadError(t *testing.T) {
	p := csRepo(t, csYAML, map[string]string{
		".awf/domains/alpha.yaml":      "paths:\n  - internal/**\n",
		"docs/decisions/0001-first.md": "---\nstatus: [unterminated\n---\n# X\n",
	})
	if _, err := p.CheckCurrentState(testContext(t)); err == nil {
		t.Fatal("expected a corpus load error from the malformed ADR")
	}
}

// invYAML declares a marker source over internal/** and a test-backing glob so a
// test-backed invariant claim can carry its required proof marker.
const invYAML = `prefix: example
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
currentState:
  sources:
    - globs: ["internal/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`

// TestCurrentStateInvariants reports the invariant claims from the working-tree
// topic corpus: a test-backed invariant with its proof-marker site and an
// unbacked one with its Verify guidance, while a rule claim never appears.
func TestCurrentStateInvariants(t *testing.T) {
	p := csRepo(t, invYAML, map[string]string{
		".awf/domains/alpha.yaml":             "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml": "title: One\nsummary: O.\npaths:\n  - internal/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n" +
			"### `rule: r`\nA rule.\nOrigin: ADR-0001\n\n" +
			"### `invariant: backed`\nBacked one.\nOrigin: ADR-0001\nBacking: test\n\n" +
			"### `invariant: reasoned`\nReasoned one.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: inspect by hand.\n",
		"internal/foo.go":      "package foo\n",
		"internal/foo_test.go": "package foo\n// invariant: alpha/one:backed\n",
	})
	invs, err := p.CurrentStateInvariants(testContext(t))
	if err != nil {
		t.Fatalf("CurrentStateInvariants: %v", err)
	}
	if len(invs) != 2 {
		t.Fatalf("invariants = %#v; want the two invariant claims (rule excluded)", invs)
	}
	// Sorted by ID: alpha/one:backed then alpha/one:reasoned.
	if invs[0].ID != "alpha/one:backed" || invs[0].Backing != "test" || len(invs[0].Proofs) != 1 ||
		!strings.HasPrefix(invs[0].Proofs[0], "internal/foo_test.go:") {
		t.Errorf("backed invariant = %#v", invs[0])
	}
	if invs[1].ID != "alpha/one:reasoned" || invs[1].Backing != "unbacked" || invs[1].Verify != "inspect by hand." || len(invs[1].Proofs) != 0 {
		t.Errorf("reasoned invariant = %#v", invs[1])
	}
}

// TestCurrentStateInvariantsEmpty proves a project with no invariant claims
// reports none without error.
// invariant: invariants/current-state-authority:invariants-zero-slugs-clean
func TestCurrentStateInvariantsEmpty(t *testing.T) {
	p := csRepo(t, "prefix: example\nskills: [tdd]\nagents: [code-reviewer]\n", map[string]string{})
	invs, err := p.CurrentStateInvariants(testContext(t))
	if err != nil {
		t.Fatalf("CurrentStateInvariants: %v", err)
	}
	if len(invs) != 0 {
		t.Fatalf("invariants = %#v; want none", invs)
	}
}

// TestCurrentStateInvariantsError propagates a corpus load failure (a
// backing-contract violation is a load error, not a reported entry): a
// test-backed invariant with no proof marker.
func TestCurrentStateInvariantsError(t *testing.T) {
	p := csRepo(t, invYAML, map[string]string{
		".awf/domains/alpha.yaml":             "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml": "title: One\nsummary: O.\npaths:\n  - internal/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n" +
			"### `invariant: backed`\nBacked one.\nOrigin: ADR-0001\nBacking: test\n",
		"internal/foo.go": "package foo\n",
	})
	if _, err := p.CurrentStateInvariants(testContext(t)); err == nil {
		t.Fatal("expected a load error for the test-backed invariant with no proof marker")
	}
}
