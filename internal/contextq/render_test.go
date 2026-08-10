package contextq

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/contextdelivery"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// acceptedV1 builds a valid Accepted current-state-v1 ADR whose Status history
// records the content digest of its five canonical sections.
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
	return doc("Accepted", "- "+date+": Proposed\n- "+date+": Accepted; content-sha256: "+adr.ContentDigest(scaffold.Sections))
}

// renderFixture builds the adopted tree the render assertions read: a current
// lock with a format-v1 cutoff of 2, domain alpha owning internal/foo/** plus a
// global core topic, the scoped topic alpha/one (a rule plus test-backed and
// unbacked invariants), an Accepted v1 ADR with a pending add on alpha/one, and
// a state marker under internal/foo/x.go.
func renderFixture(t *testing.T) *project.Project {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, ctxConfig)
	lock := &manifest.Lock{
		AWFVersion: project.Version, SchemaVersion: migrate.Current(),
		Files:             map[string]manifest.Entry{},
		BridgeAttestation: &manifest.BridgeAttestation{Version: 1, PreparedHead: "x", TreeDigest: "sha256:x", ADRFormatV1From: 2, LegacyADRGaps: []int{}},
	}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	files := ctxFiles()
	// The rule carries no declared Summary here, so the projection falls back to
	// the first prose paragraph and the full-versus-union assertion below can
	// prove the later paragraphs never reach the report.
	files[".awf/topics/parts/alpha/one/current-state.md"] = "Intro.\n\n## Claims\n\n### `rule: order`\nOrder is deterministic.\n\nFULL PROSE SECRET.\nOrigin: ADR-0001\n\n### `invariant: tested`\nTests protect output.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n"
	files["docs/decisions/0001-first.md"] = testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"),
		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n"))
	files["docs/decisions/0002-later.md"] = acceptedV1(t, "0002", "Later", "2026-07-20", "- add `alpha/one:pending-rule`")
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	p, err := project.Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// invariant: tooling/context-and-topic:context-concise-projection (TestRenderContextFullMatchesEightFacetUnion)
// invariant: tooling/context-and-topic:context-full-authority-packet (TestRenderContextFullMatchesEightFacetUnion)
func TestRenderContextFullMatchesEightFacetUnion(t *testing.T) {
	t.Parallel()
	q := queryFor(t, renderFixture(t))
	full, err := ParseContextFacets(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := ParseContextFacets([]string{"relationships", "invariants", "all-rules", "evidence", "selectors", "references", "pending", "artifacts"}, false)
	if err != nil {
		t.Fatal(err)
	}
	render := func(facets []ContextFacet) string {
		t.Helper()
		return RenderContextText(q.ContextForOptions([]string{"internal/foo/x.go"}, ContextOptions{Selection: SelectionExplicit, Facets: facets}), "header", facets)
	}
	got, want := render(full), render(explicit)
	if got != want {
		t.Fatalf("full differs from union:\n--- full ---\n%s\n--- union ---\n%s", got, want)
	}
	if strings.Count(got, "alpha/one | One") != 1 || strings.Contains(got, "FULL PROSE SECRET") || strings.Contains(got, "Direct rules:") {
		t.Fatalf("full restored repetition, prose, or legacy rosters:\n%s", got)
	}
}

// TestBareRepositoryContextFitsDirectDelivery keeps the bare report for this
// repository's own hot paths under the direct-delivery cap.
func TestBareRepositoryContextFitsDirectDelivery(t *testing.T) {
	t.Parallel()
	p, err := project.Open(testContext(t), "../..")
	if err != nil {
		t.Fatal(err)
	}
	q := queryFor(t, p)
	for _, paths := range [][]string{{"internal/project", "cmd/awf"}, {"cmd/awf/context.go"}} {
		rendered := RenderContextText(q.ContextForOptions(paths, ContextOptions{Selection: SelectionExplicit}), "context: live state for this project", nil)
		if strings.HasPrefix(rendered, "AWF_CONTEXT_SPILL_V1") || len(rendered) > contextdelivery.MaxDirectBytes {
			t.Errorf("bare context %v rendered %d bytes; direct limit is %d", paths, len(rendered), contextdelivery.MaxDirectBytes)
		}
	}
}

func TestRenderContextGrammar(t *testing.T) {
	t.Parallel()
	inside := false
	impact := contextPathImpact{Classification: pathSymlink, TargetInsideRepository: &inside, Provenance: []contextProvenance{{Role: "template", Identity: "skills/example/SKILL.md.tmpl", Sources: []artifactLink{{Path: "templates/x", Label: "template source"}}, Outputs: []artifactLink{}, Navigation: []artifactLink{}}}, Domains: []domainRef{{Name: "tooling"}}, Topics: []contextPathTopic{{ID: "tooling/example"}}, Relationships: contextRelationships{State: []string{"tooling/example:r"}, Touches: []string{}, Proofs: []string{}}, Warnings: []contextWarning{warningGlobLiteral}}
	res := ContextResult{
		Selection: SelectionRange, Range: "a..b",
		Requests: []contextRequestReport{{Index: 1, Argument: "x", Exact: &contextExactEntry{Path: "x", Context: impact}}},
		Topics:   []topicImpact{{ID: "tooling/example", Title: "Example", Summary: "Summary.", Counts: contextAuthorityCounts{Invariants: 1, Rules: 2}, Direct: []contextClaimImpact{{ID: "tooling/example:r", Type: "rule", Summary: "Rule.", Sources: []contextRelationshipSource{{RequestIndex: 1, Kinds: []string{"State"}}}, Incoming: []string{"a"}, Outgoing: []string{"b"}}}}},
	}
	out := RenderContextText(res, "header", []ContextFacet{FacetArtifacts})
	const contextGolden = "context: header\nselection: range a..b\n\nrequests:\n  request-1:\n    argument: x\n    file: x\n    classification: symlink\n    symlink-target-inside-repository: false\n    provenance: template | skills/example/SKILL.md.tmpl\n    source: templates/x | template source\n    domains: tooling\n    topics: tooling/example\n    warning: globs are not expanded; pass a directory or an exact file\n    state: tooling/example:r\n\nauthority:\n  topics:\n    tooling/example | Example | Summary. | 1 | 2\n  direct-claims:\n    tooling/example | tooling/example:r | rule | Rule.\n  claim-sources:\n    tooling/example | tooling/example:r | 1 | State\n"
	if out != contextGolden {
		t.Fatalf("context grammar:\n--- got ---\n%s--- want ---\n%s", out, contextGolden)
	}
	for _, want := range []string{"selection: range a..b", "file: x", "symlink-target-inside-repository: false", "source: templates/x", "state: tooling/example:r", "topics:\n    tooling/example | Example | Summary. | 1 | 2", "warning: globs"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

// invariant: tooling/context-and-topic:context-applicability-navigation (TestRenderContextSelectorsSeparateGlobalDeclarationAndOwnership)
func TestRenderContextSelectorsSeparateGlobalDeclarationAndOwnership(t *testing.T) {
	res := ContextResult{Selection: SelectionExplicit, Topics: []topicImpact{{
		ID: "code-design/presentation-ownership", Title: "Presentation ownership", Summary: "Summary.",
		Selectors: &contextSelectorImpact{DomainPaths: []string{"internal/**"}, TopicPaths: []string{"internal/presentation/**"}, DeclaredGlobal: true},
	}}}
	out := RenderContextText(res, "header", []ContextFacet{FacetSelectors})
	want := "selectors:\n    code-design/presentation-ownership | internal/** | applies: global | internal/presentation/** | both ownership and owning-domain selectors must match\n"
	if !strings.Contains(out, want) {
		t.Fatalf("selectors did not preserve their complete record:\n%s", out)
	}
	for _, forbidden := range []string{"applicable-paths", "owned-paths", "matched-path"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("selectors exposed path witnesses through %q:\n%s", forbidden, out)
		}
	}
}

// TestRenderContextFacetOwnedAuthorityOmitsAbsentData prevents unrequested
// authority facets from asserting that test backing or known reference edges are
// absent. The selected form retains one fixed schema for every claim row.
// invariant: tooling/context-and-topic:context-full-authority-packet (TestRenderContextFacetOwnedAuthorityOmitsAbsentData)
func TestRenderContextFacetOwnedAuthorityOmitsAbsentData(t *testing.T) {
	claim := contextClaimImpact{
		ID:       "alpha/one:tested",
		Type:     "invariant",
		Summary:  "Test-backed claim.",
		Backing:  "test",
		Verify:   "run focused test",
		Sources:  []contextRelationshipSource{{RequestIndex: 1, Kinds: []string{"State"}}},
		Evidence: []contextEvidence{{Kind: "invariant", Count: 1, Sites: []topic.MarkerSite{{Path: "internal/a  b_test.go", Line: 7}}}},
		Incoming: []string{"alpha/one:incoming"},
		Outgoing: []string{"alpha/one:outgoing"},
	}
	res := ContextResult{Selection: SelectionExplicit, Topics: []topicImpact{{ID: "alpha/one", Title: "One", Summary: "Summary.", Direct: []contextClaimImpact{claim}}}}
	bare := RenderContextText(res, "header", nil)
	for _, forbidden := range []string{"claim-evidence:", "claim-references:", "| test | run focused test"} {
		if strings.Contains(bare, forbidden) {
			t.Errorf("bare authority asserted facet-owned data: %q in:\n%s", forbidden, bare)
		}
	}
	if !strings.Contains(bare, "claim-sources:\n    alpha/one | alpha/one:tested | 1 | State") {
		t.Errorf("bare exact-file authority omitted direct source attribution:\n%s", bare)
	}
	faceted := RenderContextText(res, "header", []ContextFacet{FacetRelationships, FacetEvidence, FacetReferences})
	for _, want := range []string{"alpha/one | alpha/one:tested | invariant | Test-backed claim. | test | run focused test", "claim-sources:\n    alpha/one | alpha/one:tested | 1 | State", "internal/a  b_test.go:7", "claim-references:\n    alpha/one | alpha/one:tested | alpha/one:incoming | alpha/one:outgoing"} {
		if !strings.Contains(faceted, want) {
			t.Errorf("faceted authority missing %q:\n%s", want, faceted)
		}
	}
}

func TestRenderContextPreservesLiteralIdentities(t *testing.T) {
	const identity = "internal/a  b\tfile.go"
	res := ContextResult{Selection: SelectionExplicit, Requests: []contextRequestReport{{Index: 1, Argument: identity, Exact: &contextExactEntry{Path: identity, Context: contextPathImpact{Classification: pathCovered, Provenance: []contextProvenance{}, Domains: []domainRef{}, Topics: []contextPathTopic{}, Relationships: emptyContextRelationships(), Warnings: []contextWarning{}}}}}, Topics: []topicImpact{{ID: "alpha/two  tabs\tclaim", Title: "Title prose", Summary: "Summary prose"}}}
	got := RenderContextText(res, "header", nil)
	for _, want := range []string{"argument: " + identity, "file: " + identity, "alpha/two  tabs\tclaim | Title prose | Summary prose"} {
		if !strings.Contains(got, want) {
			t.Errorf("literal identity collapsed in %q:\n%s", want, got)
		}
	}
}

func TestRenderAllContextBranches(t *testing.T) {
	t.Parallel()
	uncovered := RenderUncoveredText(UncoveredResult{ScanRoots: []string{"internal"}, Uncovered: []uncoveredTopic{{Path: "internal/x", Domain: "d"}}, Unowned: []unownedEntry{{Path: "file", UnownedCount: 1}, {Path: "dir/", UnownedCount: 1, ExcludedCount: 2}, {Path: ".", UnownedCount: 2}}}, "header")
	for _, want := range []string{"scan-roots", "uncovered:", "unowned:", "file | 1 | 0", "dir/ | 1 | 2"} {
		if !strings.Contains(uncovered, want) {
			t.Errorf("uncovered missing %q: %s", want, uncovered)
		}
	}
	current := contextClaimImpact{ID: "d/t:i", Type: "invariant", Summary: "Invariant.", Backing: "unbacked", Verify: "inspect", Sources: []contextRelationshipSource{{RequestIndex: 1, Kinds: []string{"State"}}}, Evidence: []contextEvidence{{Kind: "state", Count: 4}, {Kind: "invariant", Count: 1, Sites: []topic.MarkerSite{{Path: "x_test.go", Line: 3}}}}}
	impact := contextPathImpact{Classification: pathNestedAdopter, NestedRoot: "child/.awf/config.yaml", Provenance: []contextProvenance{{Role: "template", Identity: "x", Sources: []artifactLink{}, Outputs: []artifactLink{{Path: "out", Label: "managed output"}}, Navigation: []artifactLink{{Path: "nav", Label: "managed output"}}}}, Domains: []domainRef{}, Topics: []contextPathTopic{}, Relationships: contextRelationships{State: []string{}, Touches: []string{}, Proofs: []string{}}, Warnings: []contextWarning{warningEligibleUnowned}, ADR: &adrArtifactContext{Number: "2", Title: "Decision", Status: "Implementing", Mutability: "frozen", AuthorityRole: "pending intent or decision history; not current authority", Operations: []adrOperationContext{{Operation: "update", Claim: "d/t:i", Progress: "applied", ClaimState: "active-current", Detail: &adrOperationDetail{Current: &current, Evidence: current.Evidence}}, {Operation: "remove", Claim: "d/t:old", Progress: "applied", ClaimState: "historically-removed", Detail: &adrOperationDetail{History: &topic.ClaimHistory{RemovedBy: &topic.ADRHistory{Number: "0002"}}}}}}}
	res := ContextResult{Selection: SelectionStaged, Requests: []contextRequestReport{{Index: 1, Argument: "empty", Directory: &contextDirectory{Included: 0, Excluded: []contextClassificationCount{{Classification: pathGeneratedOutput, Count: 2}}, Groups: []contextGroup{{Count: 2, Members: []string{"a", "b"}, Context: impact}}}}}, Topics: []topicImpact{{ID: "d/t", Title: "T", Summary: "S", Selectors: &contextSelectorImpact{DomainPaths: []string{}, TopicPaths: []string{}, DeclaredGlobal: false}, Invariants: []contextClaimImpact{current}, Pending: contextPendingImpact{OperationCount: 4, ADRs: []string{"0001", "0002", "0003"}, AdditionalADRCount: 1}}}}
	out := RenderContextText(res, "header", []ContextFacet{FacetArtifacts, FacetRelationships, FacetEvidence, FacetReferences})
	for _, want := range []string{"selection: staged", "excluded: generated-output=2", "members: a, b", "nested-root:", "output: out", "navigate: nav", "adr: ADR-2", "claim:\n      d/t:i", "removal-history:", "inspect", "state | 4 | none", "pending-summary:\n    d/t | 4 | 0001, 0002, 0003 | 1", "selectors:\n    d/t | none | none | none | both domain and topic selectors must match"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	res.Topics[0].Pending.Operations = []pendingChange{{ADR: "0002", Op: "add", Claim: "d/t:r", Progress: "remaining"}}
	res.Topics[0].Selectors.DeclaredGlobal = true
	out = RenderContextText(res, "header", nil)
	if !strings.Contains(out, "d/t | 0002 | add | d/t:r | remaining") || !strings.Contains(out, "d/t | none | applies: global | none | both ownership and owning-domain selectors must match") {
		t.Fatal(out)
	}
	// A report with neither a request nor an applicable topic renders both
	// "none" placeholders rather than an empty section.
	if bare := RenderContextText(ContextResult{Selection: SelectionExplicit}, "header", nil); !strings.Contains(bare, "requests:\n  status: none") || !strings.Contains(bare, "authority:\n  topics: none") {
		t.Fatalf("bare report placeholders:\n%s", bare)
	}
	if empty := RenderUncoveredText(UncoveredResult{}, "header"); !strings.Contains(empty, "all scanned paths are owned and covered") {
		t.Fatalf("empty coverage report:\n%s", empty)
	}
}

func TestRenderADRLinkedPlansOnlyWithReferences(t *testing.T) {
	result := ContextResult{Selection: SelectionExplicit, Requests: []contextRequestReport{{
		Index: 1, Argument: "docs/decisions/0007-example.md", Exact: &contextExactEntry{Path: "docs/decisions/0007-example.md", Context: contextPathImpact{
			Classification: pathContextIgnored,
			ADR:            &adrArtifactContext{Number: "0007", Title: "Example", Status: "Proposed", Mutability: "mutable", AuthorityRole: "pending intent or decision history; not current authority", LinkedPlans: []string{"docs/plans/2026-08-01-a.md", "docs/plans/2026-08-02-b.md"}},
		}},
	}}}
	withReferences := RenderContextText(result, "header", []ContextFacet{FacetReferences})
	if !strings.Contains(withReferences, "linked-plans: docs/plans/2026-08-01-a.md, docs/plans/2026-08-02-b.md") {
		t.Fatalf("linked plans missing from references output:\n%s", withReferences)
	}
	withoutReferences := RenderContextText(result, "header", nil)
	if strings.Contains(withoutReferences, "linked-plans:") {
		t.Fatalf("linked plans appeared without references facet:\n%s", withoutReferences)
	}
	result.Requests[0].Exact.Context.ADR.LinkedPlans = nil
	if empty := RenderContextText(result, "header", []ContextFacet{FacetReferences}); strings.Contains(empty, "linked-plans:") {
		t.Fatalf("empty linked plans rendered:\n%s", empty)
	}
}
