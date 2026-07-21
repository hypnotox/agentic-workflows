package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// ctxConfig configures two domains (alpha owns internal/foo/**, core owns
// nothing so it can carry a global topic) and a marker source over internal/**.
const ctxConfig = `prefix: example
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
  - core
currentState:
  sources:
    - globs: ["internal/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`

// ctxFiles is the standard current-state fixture: alpha owns internal/foo/** and
// owns the scoped topic alpha/one (a rule plus test-backed and unbacked
// invariants); core owns the global topic core/g. A state marker on
// internal/foo/x.go targets one claim.
func ctxFiles() map[string]string {
	return map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":                       "paths: []\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder is deterministic.\nOrigin: ADR-0001\n\n### `invariant: tested`\nTests protect output.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n",
		".awf/topics/metadata/core/g.yaml":             "title: Global\nsummary: Global rules.\napplies: global\n",
		".awf/topics/parts/core/g/current-state.md":    "Intro.\n\n## Claims\n\n### `rule: everywhere`\nApplies everywhere.\nOrigin: ADR-0001\n",
		"internal/foo/x.go":                            "package foo\n// state: alpha/one:order\n",
		"internal/foo/y.go":                            "package foo\n",
		"internal/foo/y_test.go":                       "package foo\n// invariant: alpha/one:tested\n",
	}
}

// claimIDs joins a topic context's claim IDs for compact assertions.
func claimIDs(tc TopicContext) string {
	var ids []string
	for _, c := range tc.Claims {
		ids = append(ids, c.ID)
	}
	return strings.Join(ids, ",")
}

func topicByID(res ContextResult, id string) (TopicContext, bool) {
	for _, t := range res.Topics {
		if t.ID == id {
			return t, true
		}
	}
	return TopicContext{}, false
}

// TestContextForAssembles proves the topic-centric assembly over the working
// universe: owning domains, the applicable scoped and global topics with their
// current claims, and no false unowned/pending.
func TestContextForUniverseSetupErrors(t *testing.T) {
	for _, tc := range []struct {
		name, cfg string
		sidecar   bool
	}{{"unknown-target", ctxConfig + "targets: [unknown]\n", false}, {"malformed-local", strings.Replace(ctxConfig, "  - tdd", "  - mine", 1), true}} {
		t.Run(tc.name, func(t *testing.T) {
			p := csRepo(t, ctxConfig, ctxFiles())
			if err := os.WriteFile(filepath.Join(p.Root, ".awf", "config.yaml"), []byte(tc.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.sidecar {
				path := filepath.Join(p.Root, ".awf", "skills", "mine.yaml")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("local: [bad"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := p.ContextFor([]string{"README.md"}); err == nil {
				t.Fatal("context accepted invalid selected universe")
			}
		})
	}
}

func TestContextForDeclarationReadError(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	path := filepath.Join(p.Root, ".awf", "skills", "tdd.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("local: [bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ContextFor([]string{"README.md"}); err == nil {
		t.Fatal("context accepted unreadable declaration sidecar")
	}
}

func TestContextAndTopicShareApplicability(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	ctx, err := p.ContextFor([]string{"internal/foo/y.go"})
	if err != nil {
		t.Fatal(err)
	}
	query, err := p.QueryTopic("alpha/one", topic.QueryOptions{Coverage: true})
	if err != nil {
		t.Fatal(err)
	}
	var got topic.TopicApplicability
	for _, path := range ctx.Paths {
		for _, pt := range path.Topics {
			if pt.ID == "alpha/one" {
				got = pt.Applicability
			}
		}
	}
	if !reflect.DeepEqual(got, query.Coverage.Applicability) {
		t.Fatalf("context=%#v topic=%#v", got, query.Coverage.Applicability)
	}
}

func TestContextForAssembles(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	res, err := p.ContextFor([]string{"internal/foo"})
	if err != nil {
		t.Fatalf("ContextFor: %v", err)
	}
	if len(res.Domains) != 1 || res.Domains[0].Name != "alpha" || res.Domains[0].CurrentState != "docs/domains/alpha.md" {
		t.Fatalf("domains = %#v; want just alpha with its current-state pointer", res.Domains)
	}
	if len(res.Topics) != 2 || res.Topics[0].ID != "alpha/one" || res.Topics[1].ID != "core/g" {
		t.Fatalf("topics = %#v; want [alpha/one core/g] sorted", res.Topics)
	}
	// A directory query has no exact-path state marker, so the whole topic applies.
	if got := claimIDs(res.Topics[0]); got != "alpha/one:order,alpha/one:tested,alpha/one:stable" {
		t.Errorf("alpha/one claims = %q; want all three", got)
	}
	one := res.Topics[0]
	if one.Summary != "The one topic." {
		t.Errorf("topic summary = %q", one.Summary)
	}
	if one.Claims[0].Backing != "" || one.Claims[0].Verify != "" {
		t.Errorf("rule invented backing metadata: %#v", one.Claims[0])
	}
	if one.Claims[1].Backing != "test" || one.Claims[1].Verify != "" {
		t.Errorf("test-backed projection = %#v", one.Claims[1])
	}
	if one.Claims[2].Backing != "unbacked" || one.Claims[2].Verify != "by hand." {
		t.Errorf("unbacked projection = %#v", one.Claims[2])
	}
	if !res.Topics[1].Global || claimIDs(res.Topics[1]) != "core/g:everywhere" {
		t.Errorf("core/g = %#v; want the global everywhere claim", res.Topics[1])
	}
	if len(res.Unowned) != 0 || len(res.Pending) != 0 {
		t.Errorf("unowned=%v pending=%v; want neither", res.Unowned, res.Pending)
	}
}

func TestContextForRootExpandsEligibleDescendants(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	res, err := p.ContextFor([]string{"."})
	if err != nil {
		t.Fatal(err)
	}
	got := "," + strings.Join(contextPathNames(res.Paths), ",") + ","
	for _, want := range []string{"README.md", "internal/foo/x.go", "internal/foo/y.go"} {
		if !strings.Contains(got, ","+want+",") {
			t.Errorf("root-expanded paths = %v; missing %s", res.Paths, want)
		}
	}
	if strings.Contains(got, ",.,") {
		t.Errorf("root-expanded paths retained directory literal: %v", res.Paths)
	}
}

func TestContextForNonexistentPathRemainsLiteral(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	res, err := p.ContextFor([]string{"missing/path.go"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(contextPathNames(res.Paths), ",") != "missing/path.go" || strings.Join(res.Unowned, ",") != "missing/path.go" {
		t.Fatalf("nonexistent literal result = paths %v, unowned %v", res.Paths, res.Unowned)
	}
}

func TestContextForAllIneligibleDirectoryExpandsToNothing(t *testing.T) {
	cfg := strings.Replace(ctxConfig, "currentState:", "contextIgnore:\n  - internal/ignored/**\ncurrentState:", 1)
	files := ctxFiles()
	files["internal/ignored/x.go"] = "package ignored\n"
	p := csRepo(t, cfg, files)
	res, err := p.ContextFor([]string{"internal/ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 0 || len(res.Unowned) != 0 {
		t.Fatalf("all-ineligible directory result = paths %v, unowned %v; want neither", res.Paths, res.Unowned)
	}
}

func TestContextForDirectoryExpandsEligibleDescendants(t *testing.T) {
	cfg := strings.Replace(ctxConfig, "currentState:", "contextIgnore:\n  - internal/foo/ignored.go\ncurrentState:", 1)
	files := ctxFiles()
	files["internal/foo/ignored.go"] = "package foo\n"
	files["internal/foo/nested/.awf/config.yaml"] = "prefix: nested\n"
	files["internal/foo/nested/z.go"] = "package nested\n"
	p := csRepo(t, cfg, files)
	lock := &manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 14, Files: map[string]manifest.Entry{"internal/foo/y.go": {}, "internal/foo/y_test.go": {}}}
	if err := lock.Save(lockFile(p.Root)); err != nil {
		t.Fatal(err)
	}
	res, err := p.ContextFor([]string{"internal/foo"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(contextPathNames(res.Paths), ","); got != "internal/foo/x.go" {
		t.Fatalf("expanded paths = %s; want only eligible x.go", got)
	}
	one, _ := topicByID(res, "alpha/one")
	if got := claimIDs(one); got != "alpha/one:order" {
		t.Fatalf("directory marker narrowing = %s", got)
	}
}

// TestContextForStateMarkerNarrows proves a state marker under the exact queried
// path narrows its already-applicable topic to the marked claim, while a topic
// with no marker at that path keeps its whole claim set.
func TestContextForStateMarkerNarrows(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	res, err := p.ContextFor([]string{"internal/foo/x.go"})
	if err != nil {
		t.Fatalf("ContextFor: %v", err)
	}
	one, ok := topicByID(res, "alpha/one")
	if !ok || claimIDs(one) != "alpha/one:order" {
		t.Fatalf("alpha/one = %#v; want narrowed to the marked order claim", one)
	}
	g, ok := topicByID(res, "core/g")
	if !ok || claimIDs(g) != "core/g:everywhere" {
		t.Fatalf("core/g = %#v; want the whole global topic (no marker narrows it)", g)
	}
}

// TestContextForUnownedWithGlobal proves an unowned queried path lands in Unowned
// yet still receives the always-applicable global topic.
func TestContextForUnownedWithGlobal(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	res, err := p.ContextFor([]string{"README.md"})
	if err != nil {
		t.Fatalf("ContextFor: %v", err)
	}
	if len(res.Domains) != 0 || strings.Join(res.Unowned, ",") != "README.md" {
		t.Fatalf("domains=%#v unowned=%v; want no domain and README.md unowned", res.Domains, res.Unowned)
	}
	if len(res.Topics) != 1 || res.Topics[0].ID != "core/g" {
		t.Fatalf("topics = %#v; want only the global core/g", res.Topics)
	}
}

// acceptedV1 builds a valid Accepted current-state-v1 ADR whose Status history
// records the content digest of its five canonical sections, computed from the
// Proposed scaffold that shares those sections byte-for-byte.
func acceptedV1(t *testing.T, num, title, date, stateChanges string) string {
	t.Helper()
	doc := func(status, history string) string {
		return "---\nformat: current-state-v1\nstatus: " + status + "\ndate: " + date + "\n---\n" +
			"# ADR-" + num + ": " + title + "\n\n" +
			"## Context\n\nBackground prose.\n\n" +
			"## Decision\n\n1. The decision.\n\n" +
			"## State changes\n\n" + stateChanges + "\n\n" +
			"## Consequences\n\nConsequence prose.\n\n" +
			"## Alternatives Considered\n\nNone considered.\n\n" +
			"## Status history\n\n" + history + "\n"
	}
	scaffold, err := adr.ParseV1(num+"-x.md", []byte(doc("Proposed", "- "+date+": Proposed")))
	if err != nil {
		t.Fatalf("scaffold parse: %v", err)
	}
	digest := adr.ContentDigest(scaffold.Sections)
	return doc("Accepted", "- "+date+": Proposed\n- "+date+": Accepted; content-sha256: "+digest)
}

// TestContextForPending proves an Accepted-ADR State-changes operation targeting
// a matched topic surfaces in the pending section, not among current claims.
func TestContextForPending(t *testing.T) {
	files := ctxFiles()
	files["docs/decisions/0002-later.md"] = acceptedV1(t, "0002", "Later", "2026-07-20",
		"- add `alpha/one:pending-rule`")
	p := csRepo(t, ctxConfig, files)
	// A cutoff of 2 routes ADR-0001 legacy and ADR-0002 as current-state-v1.
	writeCutoffLock(t, p, 2)

	res, err := p.ContextFor([]string{"internal/foo"})
	if err != nil {
		t.Fatalf("ContextFor: %v", err)
	}
	if len(res.Pending) != 1 {
		t.Fatalf("pending = %#v; want the one Accepted operation", res.Pending)
	}
	pc := res.Pending[0]
	if pc.ADR != "0002" || pc.Title != "Later" || pc.Status != "Accepted" || pc.Applied != 0 || pc.Declared != 1 || pc.Op != "add" || pc.Claim != "alpha/one:pending-rule" {
		t.Errorf("pending change = %#v", pc)
	}
}

// TestPendingChangesLifecycleGolden pins remaining-only projection and ordering
// across Accepted, first and middle Implementing, Implemented, and partially
// Abandoned ADRs. Terminal records contribute no pending guidance.
func TestPendingChangesLifecycleGolden(t *testing.T) {
	addDone := adr.Operation{Verb: adr.OpAdd, ID: "alpha/one:done"}
	removeOld := adr.Operation{Verb: adr.OpRemove, ID: "alpha/one:old"}
	updateRule := adr.Operation{Verb: adr.OpUpdate, ID: "alpha/one:rule"}
	middleA := adr.Operation{Verb: adr.OpAdd, ID: "alpha/one:a-applied"}
	middleB := adr.Operation{Verb: adr.OpUpdate, ID: "alpha/one:b-applied"}
	middleRemaining := adr.Operation{Verb: adr.OpRemove, ID: "alpha/one:z-remaining"}
	adrs := []adr.ADR{
		{Number: "0002", Title: "ADR-0002: Accepted", Status: "Accepted", Format: adr.CurrentStateV1, Operations: []adr.Operation{
			{Verb: adr.OpAdd, ID: "alpha/one:zeta"},
			{Verb: adr.OpUpdate, ID: "alpha/one:alpha"},
			{Verb: adr.OpAdd, ID: "beta/two:unmatched"},
		}},
		{Number: "0003", Title: "ADR-0003: First batch", Status: "Implementing", Format: adr.CurrentStateV2,
			Operations: []adr.Operation{addDone, removeOld, updateRule},
			History:    []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 7, Operations: []adr.Operation{addDone}}}},
		{Number: "0004", Title: "ADR-0004: Middle batch", Status: "Implementing", Format: adr.CurrentStateV2,
			Operations: []adr.Operation{middleA, middleB, middleRemaining},
			History: []adr.HistoryEvent{
				{Kind: adr.HistoryApplied, Sequence: 8, Operations: []adr.Operation{middleA}},
				{Kind: adr.HistoryApplied, Sequence: 9, Operations: []adr.Operation{middleB}},
			}},
		{Number: "0005", Title: "ADR-0005: Done", Status: "Implemented", Format: adr.CurrentStateV2,
			Operations: []adr.Operation{addDone}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 10, Operations: []adr.Operation{addDone}}}},
		{Number: "0006", Title: "ADR-0006: Stopped", Status: "Abandoned", Format: adr.CurrentStateV2,
			Operations: []adr.Operation{addDone, removeOld}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 11, Operations: []adr.Operation{addDone}}}},
		{Number: "0009", Title: "ADR-0009: Proposed", Status: "Proposed", Format: adr.CurrentStateV1,
			Operations: []adr.Operation{{Verb: adr.OpAdd, ID: "alpha/one:skipme"}}},
	}
	got := pendingChanges(adrs, map[string]bool{"alpha/one": true})
	want := []PendingChange{
		{ADR: "0002", Title: "Accepted", Status: "Accepted", Applied: 0, Declared: 3, Op: "update", Claim: "alpha/one:alpha"},
		{ADR: "0002", Title: "Accepted", Status: "Accepted", Applied: 0, Declared: 3, Op: "add", Claim: "alpha/one:zeta"},
		{ADR: "0003", Title: "First batch", Status: "Implementing", Applied: 1, Declared: 3, Op: "remove", Claim: "alpha/one:old"},
		{ADR: "0003", Title: "First batch", Status: "Implementing", Applied: 1, Declared: 3, Op: "update", Claim: "alpha/one:rule"},
		{ADR: "0004", Title: "Middle batch", Status: "Implementing", Applied: 2, Declared: 3, Op: "remove", Claim: "alpha/one:z-remaining"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pending changes:\n got: %#v\nwant: %#v", got, want)
	}
	invalid := adr.ADR{Number: "0010", Status: "Accepted", Format: adr.CurrentStateV1, Operations: []adr.Operation{{Verb: adr.OpAdd, ID: "alpha/one:bad"}}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 1}}}
	if got := pendingChanges([]adr.ADR{invalid}, map[string]bool{"alpha/one": true}); len(got) != 0 {
		t.Errorf("invalid progress produced pending rows: %#v", got)
	}
}

// TestAttestationBoundariesPermanentLockFields covers the post-cutover path
// where permanent lock fields carry both boundaries and the legacy gaps.
func TestAttestationBoundariesPermanentLockFields(t *testing.T) {
	if b, g := attestationBoundaries(nil); b != (adr.FormatBoundaries{}) || g != nil {
		t.Errorf("nil lock = %#v, %v; want zero boundaries, nil", b, g)
	}
	lock := &manifest.Lock{ADRFormatV1From: 5, ADRFormatV2From: 9, LegacyADRGaps: []int{2, 3}}
	b, g := attestationBoundaries(lock)
	if b != (adr.FormatBoundaries{V1From: 5, V2From: 9}) || len(g) != 2 || g[0] != 2 || g[1] != 3 {
		t.Errorf("permanent boundaries = %#v, gaps = %v", b, g)
	}
}

// TestNormalizeContextPaths covers root preservation, duplicates, and slash
// normalization into a sorted set.
func TestNormalizeContextPaths(t *testing.T) {
	got := NormalizeContextPaths([]string{"", ".", "./", "b/../b", "a", "a", "c"})
	if strings.Join(got, ",") != ".,a,b,c" {
		t.Errorf("NormalizeContextPaths = %v, want [. a b c]", got)
	}
}

// writeCutoffLock writes a lock whose sealed attestation sets the format-v1 cutoff.
func writeCutoffLock(t *testing.T, p *Project, cutoff int) {
	t.Helper()
	lock := &manifest.Lock{
		AWFVersion: "0.18.0", SchemaVersion: 14,
		BridgeAttestation: &manifest.BridgeAttestation{Version: 1, PreparedHead: "x", TreeDigest: "sha256:x", ADRFormatV1From: cutoff, LegacyADRGaps: []int{}},
	}
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, lockFile(p.Root), string(b))
}

// TestStagedContextFor proves the staged query reads the index universe: files
// staged (but never committed) are the ones assembled.
func TestStagedContextFor(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	staged := map[string]string{
		".awf/awf.lock":                                `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`,
		".awf/config.yaml":                             ctxConfig,
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":                       "paths: []\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder is deterministic.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md": testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First"),
			testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
		"internal/foo/x.go": "package foo\n",
	}
	gitfixture.Stage(t, repo, dir, staged)
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := StagedContextRoot(p.Root, []string{"internal/foo/x.go"})
	if err != nil {
		t.Fatalf("StagedContextFor: %v", err)
	}
	one, ok := topicByID(res, "alpha/one")
	if !ok || claimIDs(one) != "alpha/one:order" {
		t.Fatalf("staged topics = %#v; want alpha/one from the index", res.Topics)
	}
	full, err := StagedContextRootFull(p.Root, []string{"internal/foo/x.go"})
	if err != nil || full.Projection != ContextFull || full.Paths[0].Topics[0].Full == nil {
		t.Fatalf("staged full = %#v, %v", full, err)
	}
	selected, err := StagedContextRootFullGitSelection(p.Root, []string{"internal/foo/x.go"})
	if err != nil || selected.Requests[0].Status != RequestGitSelected || selected.Projection != ContextFull {
		t.Fatalf("staged selected full = %#v, %v", selected, err)
	}
}

func TestContextADRProjectionPreservesStagedWorkingDivergence(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := ctxFiles()
	files[".awf/awf.lock"] = `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`
	files[".awf/config.yaml"] = ctxConfig
	files["docs/decisions/0001-first.md"] = testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: Staged title"))
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "base", nil)
	testsupport.WriteFile(t, filepath.Join(dir, "docs/decisions/0001-first.md"), testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: Working title")))
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	working, err := p.ContextFor([]string{"docs/decisions/0001-first.md"})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := StagedContextRoot(dir, []string{"docs/decisions/0001-first.md"})
	if err != nil {
		t.Fatal(err)
	}
	if working.Paths[0].ADR == nil || staged.Paths[0].ADR == nil || working.Paths[0].ADR.Title != "Working title" || staged.Paths[0].ADR.Title != "Staged title" {
		t.Fatalf("working ADR=%#v staged ADR=%#v", working.Paths[0].ADR, staged.Paths[0].ADR)
	}
}

func TestStagedContextRootExpandsEligibleDescendants(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	files := ctxFiles()
	files[".awf/awf.lock"] = `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`
	files[".awf/config.yaml"] = ctxConfig
	files["docs/decisions/0001-first.md"] = testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"))
	gitfixture.Stage(t, repo, dir, files)
	res, err := StagedContextRoot(dir, []string{"."})
	if err != nil {
		t.Fatal(err)
	}
	got := "," + strings.Join(contextPathNames(res.Paths), ",") + ","
	for _, want := range []string{"README.md", "internal/foo/x.go", "internal/foo/y.go"} {
		if !strings.Contains(got, ","+want+",") {
			t.Errorf("staged root-expanded paths = %v; missing %s", res.Paths, want)
		}
	}
	if strings.Contains(got, ",.,") {
		t.Errorf("staged root-expanded paths retained directory literal: %v", res.Paths)
	}
}

func TestStagedContextAllIneligibleDirectoryExpandsToNothing(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	cfg := strings.Replace(ctxConfig, "currentState:", "contextIgnore:\n  - internal/ignored/**\ncurrentState:", 1)
	files := ctxFiles()
	files[".awf/awf.lock"] = `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`
	files[".awf/config.yaml"] = cfg
	files["docs/decisions/0001-first.md"] = testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"))
	files["internal/ignored/x.go"] = "package ignored\n"
	gitfixture.Stage(t, repo, dir, files)
	res, err := StagedContextRoot(dir, []string{"internal/ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 0 || len(res.Unowned) != 0 {
		t.Fatalf("staged all-ineligible directory result = paths %v, unowned %v; want neither", res.Paths, res.Unowned)
	}
}

func TestStagedContextDirectoryExpandsIndexDescendants(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	cfg := strings.Replace(ctxConfig, "currentState:", "contextIgnore:\n  - internal/foo/ignored.go\ncurrentState:", 1)
	gitfixture.Stage(t, repo, dir, map[string]string{
		".awf/awf.lock":                                `{"awfVersion":"0.19.0","schemaVersion":14,"files":{"internal/foo/y.go":{"templateId":"x"}}}`,
		".awf/config.yaml":                             cfg,
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":                       "paths: []\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder.\nOrigin: ADR-0001\n\n### `rule: other`\nOther.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md":                 testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25")),
		"internal/foo/x.go":                            "package foo\n// state: alpha/one:order\n",
		"internal/foo/y.go":                            "package foo\n",
		"internal/foo/ignored.go":                      "package foo\n",
		"internal/foo/nested/.awf/config.yaml":         "prefix: nested\n",
		"internal/foo/nested/z.go":                     "package nested\n",
	})
	res, err := StagedContextRoot(dir, []string{"internal/foo"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(contextPathNames(res.Paths), ","); got != "internal/foo/x.go" {
		t.Fatalf("staged expanded paths = %s", got)
	}
	one, _ := topicByID(res, "alpha/one")
	if got := claimIDs(one); got != "alpha/one:order" {
		t.Fatalf("staged marker narrowing = %s", got)
	}
}

func TestStagedContextRootDeclarationSetupErrors(t *testing.T) {
	for _, tc := range []struct {
		name, cfg string
		extra     map[string]string
	}{{"unknown-target", "prefix: x\ntargets: [unknown]\n", nil}, {"malformed-local", "prefix: x\nskills: [mine]\ntargets: [claude]\n", map[string]string{".awf/skills/mine.yaml": "local: [bad"}}} {
		t.Run(tc.name, func(t *testing.T) {
			repo, dir := gitfixture.InitRepo(t)
			gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
			files := map[string]string{".awf/awf.lock": `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`, ".awf/config.yaml": tc.cfg}
			for p, b := range tc.extra {
				files[p] = b
			}
			gitfixture.Stage(t, repo, dir, files)
			if _, err := StagedContextRoot(dir, []string{"README.md"}); err == nil {
				t.Fatal("staged setup error accepted")
			}
		})
	}
}

func TestStagedContextForNoLock(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/config.yaml": ctxConfig})
	if _, err := StagedContextRoot(dir, []string{"internal/foo"}); err == nil || !strings.Contains(err.Error(), "no staged .awf/awf.lock") {
		t.Fatalf("missing lock error = %v", err)
	}
}

// TestStagedContextForNoConfig covers the missing-staged-config branch: a repo
// whose index carries no .awf/config.yaml.
func TestStagedContextForNoConfig(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	writeLock(t, p, &manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 14})
	// The index holds a lock but no staged config.
	if _, err := StagedContextRoot(p.Root, []string{"internal/foo"}); err == nil {
		t.Fatal("expected a no-staged-config error")
	}
}

// TestStagedContextForOutsideRepo covers the index-tree failure outside a repo.
func TestStagedContextForOutsideRepo(t *testing.T) {
	root := scaffoldFiles(t, ctxConfig, map[string]string{".awf/domains/core.yaml": "paths: []\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StagedContextRoot(p.Root, []string{"internal/foo"}); err == nil {
		t.Fatal("expected an index-tree error outside a git repository")
	}
}

// TestStagedContextForCorruptLock proves a corrupt working lock is irrelevant
// when the index lock is valid.
func TestStagedContextForCorruptLock(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, dir, map[string]string{
		".awf/awf.lock":           `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`,
		".awf/config.yaml":        ctxConfig,
		".awf/domains/alpha.yaml": "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":  "paths: []\n",
	})
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, lockFile(p.Root), "{not json")
	if _, err := StagedContextRoot(p.Root, []string{"internal/foo"}); err != nil {
		t.Fatalf("staged context consulted corrupt working lock: %v", err)
	}
}

// TestStagedContextForCorpusError propagates a corpus load failure from a
// malformed staged ADR through the index loader.
func TestStagedContextForCorpusError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, dir, map[string]string{
		".awf/awf.lock":                `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`,
		".awf/config.yaml":             ctxConfig,
		".awf/domains/core.yaml":       "paths: []\n",
		"docs/decisions/0001-first.md": "---\nstatus: [unterminated\n---\n# X\n",
	})
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StagedContextRoot(p.Root, []string{"internal/foo"}); err == nil {
		t.Fatal("expected a corpus load error from the staged tree")
	}
}

// TestContextForOutsideRepo covers the working-Tree open failure outside a repo.
func TestContextForOutsideRepo(t *testing.T) {
	root := scaffoldFiles(t, ctxConfig, map[string]string{".awf/domains/core.yaml": "paths: []\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ContextFor([]string{"internal/foo"}); err == nil {
		t.Fatal("expected a working-tree error outside a git repository")
	}
}

// TestContextForLoadError propagates a corpus load failure from a malformed ADR.
func TestContextForLoadError(t *testing.T) {
	p := csRepo(t, ctxConfig, map[string]string{
		".awf/domains/core.yaml":       "paths: []\n",
		"docs/decisions/0001-first.md": "---\nstatus: [unterminated\n---\n# X\n",
	})
	if _, err := p.ContextFor([]string{"internal/foo"}); err == nil {
		t.Fatal("expected a corpus load error")
	}
}

// TestContextForCorruptLock covers the lock-read failure in the working loader.
func TestContextForCorruptLock(t *testing.T) {
	p := csRepo(t, ctxConfig, map[string]string{".awf/domains/core.yaml": "paths: []\n"})
	testsupport.WriteFile(t, lockFile(p.Root), "{not json")
	if _, err := p.ContextFor([]string{"internal/foo"}); err == nil {
		t.Fatal("expected a lock parse error")
	}
}

// uncoveredConfig makes alpha own internal/** while the topic covers only
// internal/foo/**, so internal/bar.go is owned-but-uncovered.
const uncoveredConfig = `prefix: example
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
contextIgnore:
  - .awf/**
currentState:
  topicCoverage: error
  topicFanout: off
`

func uncoveredFiles() map[string]string {
	return map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder.\nOrigin: ADR-0001\n",
		"internal/foo/x.go":                            "package foo\n",
		"internal/bar.go":                              "package internalx\n",
		"docs/thing.md":                                "doc\n",
	}
}

// TestUncovered proves the report lists domain-owned paths with no scoped topic
// and, separately, the eligible paths owned by no domain (collapsed).
// invariant: invariants/current-state-authority:uncovered-lists-unowned-unignored
// invariant: tooling/cli:uncovered-collapses-directories
func TestUncovered(t *testing.T) {
	p := csRepo(t, uncoveredConfig, uncoveredFiles())
	res, err := p.Uncovered(nil)
	if err != nil {
		t.Fatalf("Uncovered: %v", err)
	}
	if len(res.Uncovered) != 1 || res.Uncovered[0].Path != "internal/bar.go" || res.Uncovered[0].Domain != "alpha" {
		t.Fatalf("uncovered = %#v; want internal/bar.go owned by alpha", res.Uncovered)
	}
	// docs/ and the committed README.md are owned by no domain; docs collapses.
	if strings.Join(res.Unowned, ",") != "README.md,docs/" {
		t.Errorf("unowned = %v; want [README.md docs/]", res.Unowned)
	}
}

// TestUncoveredCollapsesToRoot covers the root-collapse branch: when a domain
// owns nothing present, no path seeds the repository root as covered, so a
// whole-repo scan folds every unowned path up to ".".
func TestUncoveredCollapsesToRoot(t *testing.T) {
	cfg := "prefix: example\ndomains:\n  - alpha\ncontextIgnore:\n  - .awf/**\ncurrentState:\n  topicCoverage: error\n  topicFanout: off\n"
	files := map[string]string{
		".awf/domains/alpha.yaml": "paths:\n  - nonexistent/**\n",
		"top.txt":                 "x\n",
	}
	res, err := csRepo(t, cfg, files).Uncovered(nil)
	if err != nil {
		t.Fatalf("Uncovered: %v", err)
	}
	if strings.Join(res.Unowned, ",") != "." {
		t.Errorf("unowned = %v; want just \".\"", res.Unowned)
	}
}

// TestUncoveredScanRoot restricts the report to a scan root on segment boundaries.
func TestUncoveredScanRoot(t *testing.T) {
	p := csRepo(t, uncoveredConfig, uncoveredFiles())
	res, err := p.Uncovered([]string{"internal"})
	if err != nil {
		t.Fatalf("Uncovered: %v", err)
	}
	if len(res.Uncovered) != 1 || res.Uncovered[0].Path != "internal/bar.go" {
		t.Fatalf("uncovered = %#v; want just internal/bar.go", res.Uncovered)
	}
	if len(res.Unowned) != 0 {
		t.Errorf("unowned = %v; want none in scope (docs/ and README.md are out of scope)", res.Unowned)
	}
	if strings.Join(res.ScanRoots, ",") != "internal" {
		t.Errorf("scanRoots = %v", res.ScanRoots)
	}
}

func TestStagedUncoveredOutsideRepo(t *testing.T) {
	if _, err := StagedUncoveredRoot(t.TempDir(), nil); err == nil {
		t.Fatal("expected staged uncovered to reject a non-repository")
	}
}

// TestUncoveredOutsideRepo covers the working-Tree failure in Uncovered.
func TestUncoveredOutsideRepo(t *testing.T) {
	root := scaffoldFiles(t, uncoveredConfig, map[string]string{".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Uncovered(nil); err == nil {
		t.Fatal("expected a working-tree error outside a git repository")
	}
}

func contextPathNames(paths []ContextPath) []string {
	out := make([]string, len(paths))
	for i := range paths {
		out[i] = paths[i].Path
	}
	return out
}
