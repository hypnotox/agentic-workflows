package project

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func TestCommitAuthorizationResultDiagnostic(t *testing.T) {
	result := CommitAuthorizationResult{Category: "operation", Condition: "non-merge", ChangedIndex: true, NextActions: []string{"correct the message trailers", "run git commit"}}
	diagnostic, err := result.Diagnostic()
	if err != nil {
		t.Fatal(err)
	}
	document, err := diagnostic.Document()
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := presentation.Render(&got, document); err != nil {
		t.Fatal(err)
	}
	want := "condition: non-merge\nstate: operation\n\ndiagnostic:\n  changed:\n    index: yes\n    message: no\n    merge state: no\n  steps:\n    step 1: correct the message trailers\n    step 2: run git commit\n"
	if got.String() != want {
		t.Fatalf("diagnostic = %q, want %q", got.String(), want)
	}
}

func TestHeadTreeAndLockRejectsMissingRepository(t *testing.T) {
	tree, lock, err := headTreeAndLock(nil, testContext(t))
	if tree != nil || lock != nil || !errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("head tree without repository = (%v, %v, %v)", tree, lock, err)
	}
}

func TestLoadTreeCurrentStateRejectsFutureSchema(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: example\nprofile: full\nintegrationBranch: main\n")}})
	if err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{SchemaVersion: migrate.Current() + 1}
	if _, _, err := loadTreeCurrentState(".", tree, lock); err == nil || !strings.Contains(err.Error(), "ahead of current") {
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
	if _, _, err := loadTreeCurrentState(".", configTree, nil); err == nil {
		t.Fatal("symlink config accepted")
	}
	// A config today's schema cannot parse for a reason the retired-key
	// port-forward does not fix. That pass strips keys whose struct field is
	// gone, so only a key that was never declared still reaches the parser.
	unknownKey, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: x\nintegrationBranch: main\nnoSuchKey: 1\n")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadTreeCurrentState(".", unknownKey, nil); err == nil {
		t.Fatal("unknown config key accepted")
	}
	ordinary, _ := snapshot.NewTree([]snapshot.File{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: x\nintegrationBranch: main\n")}, {Path: "docs/decisions/0001-link.md", Mode: snapshot.Symlink, Bytes: []byte("bad")}})
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

func TestEligiblePathsExcludeNestedAdopter(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: "nested/.awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: nested\nintegrationBranch: main\n")},
		{Path: "nested/internal/x.go", Mode: snapshot.Regular, Bytes: []byte("package internal\n")},
		{Path: "internal/owned.go", Mode: snapshot.Regular, Bytes: []byte("package internal\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := eligiblePaths(tree, nil, nil), []string{"internal/owned.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("eligible paths=%v, want %v", got, want)
	}
}

func TestResidentPathsAreNeverEligibleOrNested(t *testing.T) {
	const adversarial = ".awf/efforts/e/.awf/config.yaml"
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: adversarial, Mode: snapshot.Regular, Bytes: []byte("prefix: nested\nintegrationBranch: main\n")},
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
	if !resident.IsResidentPath(adversarial) || resident.IsResidentPath(".awf/effort/other") {
		t.Fatal("resident path predicate is not closed to resident roots")
	}
}

// TestCurrentStateReportRouting proves the report splits into blocking findings
// (every static handshake message plus each error-severity coverage line) and
// non-failing notes (warn-severity coverage lines), rendering both coverage
// kinds.
// The claim's routing clause is marked here rather than named in prose from
// internal/currentstate, which cannot see CurrentStateReport.
// invariant: invariants/current-state-authority:currentstate-handshake-findings-unranked (TestCurrentStateReportRouting)
func currentStateWarningNotes(report CurrentStateReport) []string {
	var out []string
	for _, finding := range report.Coverage {
		if finding.Severity == severity.Warn {
			out = append(out, finding.Message())
		}
	}
	return append(out, report.PlanNotes...)
}

func TestCurrentStateReportRouting(t *testing.T) {
	r := CurrentStateReport{
		Static:      []currentstate.Finding{{Message: "handshake broke"}},
		Provisional: []currentstate.Introduction{{Identity: "0002", Format: adr.CurrentStateV2}, {Identity: "0003", Format: adr.Legacy}, {Identity: "0004", Format: adr.Format(999)}},
		PlanDrift:   []manifest.Drift{{Path: "docs/plans/v2.md", Kind: "plan-reference", Detail: "missing ADR"}},
		Coverage: []topic.CoverageFinding{
			{Path: "internal/a.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error, CandidateTopics: []string{"alpha/global"}},
			{Path: "internal/b.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 3},
			{Path: "internal/c.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Warn, CandidateTopics: []string{"alpha/a", "alpha/b"}},
		},
	}
	findings := r.Findings()
	wantCoverage := "uncovered: internal/a.go is owned by domain alpha with no claim-bearing topic owner; if global topic alpha/global naturally governs this path, add a matching domain-bounded paths selector; otherwise create/use an appropriate scoped claim-bearing topic"
	if len(findings) != 3 || findings[0] != "handshake broke" || findings[1] != wantCoverage || findings[2] != "plan-reference docs/plans/v2.md: missing ADR" {
		t.Fatalf("findings = %#v", findings)
	}
	notes := append(r.Information(), currentStateWarningNotes(r)...)
	wantNote := "uncovered: internal/c.go is owned by domain alpha with no claim-bearing topic owner; if one of global topics alpha/a, alpha/b naturally governs this path, add a matching domain-bounded paths selector; otherwise create/use an appropriate scoped claim-bearing topic"
	if len(notes) != 5 || !strings.Contains(notes[0], "provisional older-format ADR-0002") || !strings.Contains(notes[1], "ADR-0003 (legacy)") || !strings.Contains(notes[2], "ADR-0004 (legacy)") || !strings.Contains(notes[3], "internal/b.go is matched by 3 owning topics") || notes[4] != wantNote {
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
profile: full
integrationBranch: main
domains:
  - alpha
contextIgnore:
  - internal/skip.go
`

// csYAML is csNoPolicyYAML plus the currentState block.
const csYAML = csNoPolicyYAML + "currentState:\n"

// csRuleTopic is a one-claim current-state part citing an Implemented Origin ADR.
const csRuleTopic = "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-0001\n"

// csRepo builds a git-backed project: a fresh repo, the given config, and the
// given working files (untracked but nonignored, so the working Tree includes
// them). It writes an Implemented ADR-0001 the topic can cite unless the caller
// supplies its own decisions file.
func csRepo(t *testing.T, cfg string, files map[string]string) *ProjectState {
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
	testsupport.WriteFile(t, lockFile(p.Root()), string(b))

	report, err := checkCurrentStateProject(p, testContext(t))
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
	notes := append(report.Information(), currentStateWarningNotes(report)...)
	if len(notes) != 0 {
		t.Errorf("notes = %#v; want none", notes)
	}
}

// TestCheckCurrentStateNoPolicy proves coverage and fan-out both evaluate for a
// tree that declares no currentState block: evaluation does not depend on the
// block's presence (ADR-0192). internal/foo/** carries nine topics, one
// claim-bearing so the path is covered and eight claimless, which together
// exceed the nil-receiver default budget of 8 and yield the fan-out finding;
// internal/bar.go is owned by the domain but scoped by no claim-bearing topic,
// so it yields the coverage finding.
//
// The DeepEqual below pins severity.Error and severity.Warn exactly, which is
// also what backs severity-not-configurable's fixed-rank clause; that clause lost
// its only marker when this test's previous assertion (of the struck
// block-presence sentence) was inverted.
// invariant: rendering/sync-and-drift:coverage-evaluation-unconditional (TestCheckCurrentStateNoPolicy)
// invariant: config/configuration:severity-not-configurable (TestCheckCurrentStateNoPolicy)
func TestCheckCurrentStateNoPolicy(t *testing.T) {
	cfg := "prefix: example\nprofile: full\nintegrationBranch: main\ndomains: [alpha]\n"
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
	report, err := checkCurrentStateProject(p, testContext(t))
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

// TestCheckCurrentStateOutsideRepo covers the filesystem working-tree fallback
// for a scaffolded project that is not a git repository.
func TestCheckStagedRootOutsideRepo(t *testing.T) {
	if _, err := CheckStagedRoot(testContext(t), t.TempDir()); err == nil {
		t.Fatal("CheckStagedRoot accepted a non-repository")
	}
}

func TestCheckCurrentStateOutsideRepo(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checkCurrentStateProject(p, testContext(t)); err != nil {
		t.Fatalf("filesystem current-state fallback: %v", err)
	}
}

func TestCheckCurrentStateNoInvariantClaims(t *testing.T) {
	p := csRepo(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", map[string]string{})
	report, err := checkCurrentStateProject(p, testContext(t))
	if err != nil {
		t.Fatalf("current-state check with no invariant claims: %v", err)
	}
	if findings := report.Findings(); len(findings) != 0 {
		t.Fatalf("current-state findings with no invariant claims = %v, want none", findings)
	}
}

// TestCheckCurrentStateCorruptLock covers the lock-read failure: a malformed
// awf.lock is not gated before this project method.
func TestCheckCurrentStateCorruptLock(t *testing.T) {
	p := csRepo(t, csYAML, map[string]string{".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"})
	testsupport.WriteFile(t, lockFile(p.Root()), "{not json")
	if _, err := checkCurrentStateProject(p, testContext(t)); err == nil {
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
	if _, err := checkCurrentStateProject(p, testContext(t)); err == nil {
		t.Fatal("expected a corpus load error from the malformed ADR")
	}
}

func TestCommitAuthorizationResultDiagnosticRejectsInvalidAction(t *testing.T) {
	result := CommitAuthorizationResult{Category: "operation", Condition: "refused", NextActions: []string{""}}
	if _, err := result.Diagnostic(); err == nil {
		t.Fatal("invalid action accepted")
	}
}
