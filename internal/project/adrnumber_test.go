package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// numberingYAML is sampleYAML plus the one domain the numbering fixtures hang a
// topic off, so the authored claim parts the substitution rewrites belong to a
// configured topic.
const numberingYAML = `prefix: example
integrationBranch: main
vars:
  testCmd: go test ./...
  gateCmd: make gate
  gateCmdFull: make gate full
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
`

// numberingProject builds a project whose render is already current and then
// writes files on top, so a numbering run's re-render is a no-op on everything
// numbering must not touch. It returns the reopened project and its root.
func numberingProject(t *testing.T, files map[string]string) (*Project, string) {
	t.Helper()
	root := scaffoldFiles(t, numberingYAML, map[string]string{
		"domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		"topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/**\n",
		"topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: seed`\nSeed.\nOrigin: ADR-0001\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-seed.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-07-31"),
			testsupport.WithTitle("0001: Seed"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")))
	if err := p.Sync(); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, rel), body)
	}
	reopened, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	return reopened, root
}

// pendingRecord builds an Implemented pending current-state-v3 record whose
// declared ops are applied through the implicit batch its Implemented status
// event carries. The stamp its history owes is computed from the body the
// record actually carries, so the fixture parses as a real governed record
// rather than as a shape the loader happens to tolerate.
func pendingRecord(t *testing.T, slug string, ops ...string) string {
	t.Helper()
	declared := "- " + strings.Join(ops, "\n- ") + "\n"
	body := func(digest string) string {
		return "---\nformat: current-state-v3\nslug: " + slug + "\nstatus: Implemented\ndate: 2026-07-31\n---\n" +
			"# ADR-" + slug + ": A decision\n\n" +
			"## Context\n\nBackground prose.\n\n" +
			"## Decision\n\n1. The only decision.\n\n" +
			"## State changes\n\n" + declared + "\n" +
			"## Consequences\n\nConsequence prose.\n\n" +
			"## Alternatives Considered\n\nNone considered.\n\n" +
			"## Status history\n\n- 2026-07-31: Proposed\n" +
			"- 2026-07-31: Accepted; content-sha256: " + digest + "\n" +
			"- 2026-07-31: Implemented; content-sha256: " + digest + "\n"
	}
	// The digest covers the five body sections and never the history, so
	// stamping the document does not move the value being stamped.
	parsed, _, err := adr.ParseBytes(slug+".md", []byte(body("placeholder")))
	if err != nil {
		t.Fatalf("build pending record %s: %v", slug, err)
	}
	return body(adr.ContentDigest(parsed.Sections))
}

// numberingPart is the authored claim part the substitution runs over. It holds
// every case at once: a claim citing nothing this run numbers, an Origin naming
// a pending record beside a prose mention of that same slug, and a Revised-by
// list whose two pending entries are authored in the opposite order to the one
// numbering assigns them, so the canonical result is a re-sort rather than an
// append.
const numberingPart = "Intro.\n\n## Claims\n\n" +
	"### `rule: seed`\nSeed.\nOrigin: ADR-0001\n\n" +
	"### `rule: origin-pending`\nADR-early decided this, see ADR-early.\nOrigin: ADR-early\n\n" +
	"### `rule: list-pending`\nListed.\nOrigin: ADR-0001\nRevised-by: ADR-late, ADR-early\n"

// numberingFixture is the two-pending-record corpus the assignment and
// substitution tests share.
func numberingFixture(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		".awf/topics/parts/alpha/one/current-state.md": numberingPart,
		"docs/decisions/early.md":                      pendingRecord(t, "early", "add `alpha/one:origin-pending`", "update `alpha/one:list-pending`"),
		"docs/decisions/late.md":                       pendingRecord(t, "late", "update `alpha/one:list-pending`"),
	}
}

// Numbering's whole authored effect surface in one run: each named record takes
// the next number in argument order, its filename and heading follow, the
// retained slug key stays, and the authored provenance lines take the
// substitution with each touched list canonicalized into ascending order. The
// prose mention of the same slug survives untouched, which is what proves the
// substitution is anchored on the provenance lines rather than a document-wide
// replace, and the re-render leaves the generated index numbered.
// invariant: adr-system/adr-lifecycle:pending-adr-slug-identity
func TestNumberPendingADRsAssignsAndSubstitutes(t *testing.T) {
	p, root := numberingProject(t, numberingFixture(t))

	report, err := p.NumberPendingADRs(testContext(t), []string{"early", "late"})
	if err != nil {
		t.Fatalf("NumberPendingADRs: %v", err)
	}
	if got := report.String(); got != "early -> 0002\nlate -> 0003\n" {
		t.Errorf("report = %q", got)
	}
	for _, gone := range []string{"docs/decisions/early.md", "docs/decisions/late.md"} {
		if _, err := os.Stat(filepath.Join(root, gone)); !os.IsNotExist(err) {
			t.Errorf("%s survived numbering: %v", gone, err)
		}
	}
	numbered, err := os.ReadFile(filepath.Join(root, "docs/decisions/0002-early.md"))
	if err != nil {
		t.Fatalf("read numbered record: %v", err)
	}
	if !strings.Contains(string(numbered), "# ADR-0002: A decision") || !strings.Contains(string(numbered), "slug: early\n") {
		t.Errorf("numbered record heading or retained slug wrong:\n%s", numbered)
	}
	part, err := os.ReadFile(filepath.Join(root, ".awf/topics/parts/alpha/one/current-state.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "Intro.\n\n## Claims\n\n" +
		"### `rule: seed`\nSeed.\nOrigin: ADR-0001\n\n" +
		"### `rule: origin-pending`\nADR-early decided this, see ADR-early.\nOrigin: ADR-0002\n\n" +
		"### `rule: list-pending`\nListed.\nOrigin: ADR-0001\nRevised-by: ADR-0002, ADR-0003\n"
	if string(part) != want {
		t.Errorf("substitution wrong:\ngot:\n%s\nwant:\n%s", part, want)
	}
	index, err := os.ReadFile(filepath.Join(root, "docs/decisions/INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "0002") || strings.Contains(string(index), "ADR-early") {
		t.Errorf("the re-render must leave the generated index numbered:\n%s", index)
	}
}

// Numbering never rewrites a plan. The fixture plan links the pending record by
// slug and also carries two lines spelled exactly like a claim's provenance, so
// a substitution that walked the tree instead of the authored parts tree would
// rewrite it. Every byte under docs/plans/ must survive, and the slug link must
// still resolve afterwards, now against the numbered record's retained slug key.
// invariant: adr-system/plan-artifacts:plan-adr-link-resolved
func TestNumberPendingADRsLeavesPlansUntouched(t *testing.T) {
	const planBody = "---\ndate: 2026-07-31\nadrs: [early]\nstatus: Proposed\n---\n" +
		"# Plan: Numbering\n\nOrigin: ADR-early\n\nRevised-by: ADR-late, ADR-early\n"
	files := numberingFixture(t)
	files["docs/plans/2026-07-31-numbering.md"] = planBody
	p, root := numberingProject(t, files)
	before := snapshotDir(t, filepath.Join(root, "docs/plans"))

	if _, err := p.NumberPendingADRs(testContext(t), []string{"early", "late"}); err != nil {
		t.Fatalf("NumberPendingADRs: %v", err)
	}
	after := snapshotDir(t, filepath.Join(root, "docs/plans"))
	if len(before) != len(after) {
		t.Fatalf("the docs/plans file set changed: %v -> %v", keysOf(before), keysOf(after))
	}
	for name, body := range before {
		if after[name] != body {
			t.Errorf("numbering rewrote docs/plans/%s:\ngot:\n%s\nwant:\n%s", name, after[name], body)
		}
	}
	reopened, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := reopened.checkPlans(mustDeriveCorpus(t, reopened))
	if err != nil {
		t.Fatalf("checkPlans: %v", err)
	}
	if len(drift) != 0 {
		t.Errorf("the slug link must still resolve after numbering: %#v", drift)
	}
}

// A lone pending record is numbered by a bare invocation.
// invariant: adr-system/adr-lifecycle:pending-adr-slug-identity
func TestNumberPendingADRsBareInvocationNumbersTheOnlyRecord(t *testing.T) {
	p, root := numberingProject(t, map[string]string{
		"docs/decisions/only.md": pendingRecord(t, "only", "add `alpha/one:seed-two`"),
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n" +
			"### `rule: seed`\nSeed.\nOrigin: ADR-0001\n\n" +
			"### `rule: seed-two`\nSecond.\nOrigin: ADR-only\n",
	})
	report, err := p.NumberPendingADRs(testContext(t), nil)
	if err != nil {
		t.Fatalf("NumberPendingADRs: %v", err)
	}
	if got := report.String(); got != "only -> 0002\n" {
		t.Errorf("report = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/decisions/0002-only.md")); err != nil {
		t.Errorf("numbered record missing: %v", err)
	}
}

// The closing re-render is part of the operation, so a corpus that cannot be
// re-rendered still fails the command. It must not fail emptily: by then the
// rename is on disk, so the error arrives carrying the mapping the integration
// commit message needs. Returning an empty report here would strand the
// operator with a renamed corpus and no record of what it was renamed to, and a
// re-run would only report that there is no pending ADR left to number.
// invariant: adr-system/adr-lifecycle:pending-adr-slug-identity
func TestNumberPendingADRsReportsTheMappingWhenTheRerenderFails(t *testing.T) {
	p, root := numberingProject(t, map[string]string{
		"docs/decisions/only.md": pendingRecord(t, "only", "add `alpha/one:seed-two`"),
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n" +
			"### `rule: seed`\nSeed.\nOrigin: ADR-0001\n\n" +
			"### `rule: dangling`\nDangling.\nOrigin: ADR-nowhere\n",
	})
	report, err := p.NumberPendingADRs(testContext(t), nil)
	if err == nil || !strings.Contains(err.Error(), "cites missing ADR-nowhere") {
		t.Fatalf("re-render failure = %v", err)
	}
	if got := report.String(); got != "only -> 0002\n" {
		t.Errorf("mapping lost on a failed re-render: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/decisions/0002-only.md")); err != nil {
		t.Errorf("the rename the surviving mapping describes did not happen: %v", err)
	}
}

// Every way numbering refuses to run, over the corpus that produces it, each
// asserted on the whole message so a refusal cannot silently become a different
// one. The reset-remake recipe is offered verbatim only when duplicate numbers
// coexist with no pending record, because that is the only corpus where a stale
// numbering commit is the diagnosis; with a pending record present the corpus
// has to be resolved first and the recipe would be wrong advice. Every case also
// asserts the decisions directory is untouched: a refusal that had already
// renamed a record would leave the corpus worse than it found it.
// invariant: adr-system/adr-lifecycle:adr-number-immutable
func TestNumberPendingADRsRefusals(t *testing.T) {
	legacy := func(title string) string {
		return testsupport.ADR("Implemented", testsupport.WithDate("2026-07-31"),
			testsupport.WithTitle(title), testsupport.WithBody("## Context\nx\n## Consequences\nc\n"))
	}
	numberedTwin := strings.Replace(pendingRecord(t, "early", "add `alpha/one:x`"), "# ADR-early:", "# ADR-0002:", 1)
	for _, tc := range []struct {
		name  string
		files map[string]string
		args  []string
		want  string
	}{
		{
			name:  "no pending record and no duplicates",
			files: map[string]string{},
			want:  "no pending ADR to number",
		},
		{
			name: "duplicate numbers with a pending record",
			files: map[string]string{
				"docs/decisions/0002-a.md": legacy("0002: A"),
				"docs/decisions/0002-b.md": legacy("0002: B"),
				"docs/decisions/early.md":  pendingRecord(t, "early", "add `alpha/one:x`"),
			},
			want: "duplicate ADR numbers present; resolve the corpus before numbering",
		},
		{
			name: "duplicate numbers with no pending record",
			files: map[string]string{
				"docs/decisions/0002-a.md": legacy("0002: A"),
				"docs/decisions/0002-b.md": legacy("0002: B"),
			},
			want: "duplicate ADR numbers with no pending record: if a stale numbering commit collided, " +
				"run: git reset --hard HEAD~1 && git merge <integration branch> && awf adr number, then gate and merge back",
		},
		{
			name: "a duplicate slug",
			files: map[string]string{
				"docs/decisions/early.md":      pendingRecord(t, "early", "add `alpha/one:x`"),
				"docs/decisions/0002-early.md": numberedTwin,
			},
			want: `ADR slug "early" is declared by more than one file`,
		},
		{
			name:  "several pending records and no arguments",
			files: numberingFixture(t),
			want:  "several pending ADRs require an explicit list naming every pending slug:\nearly\nlate",
		},
		{
			name:  "an argument naming no pending record",
			files: numberingFixture(t),
			args:  []string{"early", "third"},
			want:  `"third" names no pending ADR`,
		},
		{
			name:  "an argument named twice",
			files: numberingFixture(t),
			args:  []string{"early", "early"},
			want:  `"early" is named more than once`,
		},
		{
			name:  "an explicit list omitting a pending record",
			files: numberingFixture(t),
			args:  []string{"early"},
			want:  "the explicit list must name every pending ADR; omitted: late",
		},
		{
			name: "an order revising before the add",
			files: map[string]string{
				"docs/decisions/early.md": pendingRecord(t, "early", "add `alpha/one:x`"),
				"docs/decisions/late.md":  pendingRecord(t, "late", "update `alpha/one:x`"),
			},
			args: []string{"late", "early"},
			want: "late revises a claim added by early; number early first",
		},
		{
			name: "an order removing before the add",
			files: map[string]string{
				"docs/decisions/early.md": pendingRecord(t, "early", "add `alpha/one:x`"),
				"docs/decisions/late.md":  pendingRecord(t, "late", "remove `alpha/one:x`"),
			},
			args: []string{"late", "early"},
			want: "late removes a claim added by early; number early first",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, root := numberingProject(t, tc.files)
			before := snapshotDir(t, filepath.Join(root, "docs/decisions"))
			_, err := p.NumberPendingADRs(testContext(t), tc.args)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			after := snapshotDir(t, filepath.Join(root, "docs/decisions"))
			if len(before) != len(after) {
				t.Fatalf("a refused numbering must write nothing: %v -> %v", keysOf(before), keysOf(after))
			}
			for name, body := range before {
				if after[name] != body {
					t.Errorf("a refused numbering rewrote docs/decisions/%s", name)
				}
			}
		})
	}
}

// snapshotDir reads every file under dir keyed by its dir-relative slash path.
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		out[filepath.ToSlash(rel)] = string(body)
		return nil
	}); err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return out
}

// keysOf lists a snapshot's paths for a failure message.
func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Numbering deliberately does not precondition on a green check (ADR-0194 item
// 11): a green check between merge-in and numbering is the norm, but an
// unrelated finding must not deadlock the one command that resolves the corpus.
// The claim says so about this engine, and every other proof for it sits on the
// transition-validation layer, so this is the case that can falsify it: a tree
// carrying real rendered-output drift still numbers.
// invariant: adr-system/adr-lifecycle:numbering-transition-mode
func TestNumberPendingADRsIgnoresUnrelatedDrift(t *testing.T) {
	p, root := numberingProject(t, map[string]string{
		"docs/decisions/only.md": pendingRecord(t, "only", "add `alpha/one:seed-two`"),
	})
	// Hand-edit a rendered output so the drift oracle is red for a reason that
	// has nothing to do with numbering.
	rendered := filepath.Join(root, "docs/topics/alpha/one.md")
	body, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, rendered, string(body)+"\nhand-edited drift\n")
	drift, err := p.Check(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) == 0 {
		t.Fatal("the fixture must be drifted for this test to mean anything")
	}

	report, err := p.NumberPendingADRs(testContext(t), nil)
	if err != nil {
		t.Fatalf("numbering must not require a green check: %v", err)
	}
	if got := report.String(); got != "only -> 0002\n" {
		t.Errorf("mapping = %q", got)
	}
}
