package generatedcheck

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestTrackingClassifiesMissingPlanOutput(t *testing.T) {
	out := outputplan.NewOutput(outputplan.OutputSpec{Path: "AGENTS.md"})
	plan := planFor(out)
	got, err := Tracking(context.Background(), false, func(context.Context) ([]string, error) { return nil, nil }, plan)
	if err != nil {
		t.Fatal(err)
	}
	findings := got.Findings()
	if len(findings) != 2 || findings[1].Evidence.Path != "AGENTS.md" {
		t.Fatalf("findings=%#v", findings)
	}
}
func TestStagedFreshnessPrecedesObservation(t *testing.T) {
	out := outputplan.NewOutput(outputplan.OutputSpec{Path: "out", Content: "fresh", TemplateHash: "t", ConfigHash: "c"})
	plan := planFor(out)
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"out": {TemplateHash: "t", ConfigHash: "c", OutputHash: manifest.Hash([]byte("old"))}}}
	tree := reader{map[string][]byte{"out": []byte("edit"), ".awf/awf.lock": []byte("lock")}}
	got, err := Staged(false, lock, plan, tree, map[string]bool{"out": true, ".awf/awf.lock": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Findings()) != 1 || got.Findings()[0].Evidence.Kind != "stale" {
		t.Fatalf("drift=%#v", got)
	}
}

// invariant: rendering/sync-and-drift:agent-guide-size-advisory (TestLockedValidationAndAdvisory)
// invariant: tooling/cli:check-severity-by-protected-property (TestLockedValidationAndAdvisory)
func TestLockedValidationAndAdvisory(t *testing.T) {
	for _, content := range []string{"plain", "---\nname: x\n---\n", "---\nname: x\ndescription: ' '\n---\n", "---\nname: ' '\ndescription: x\n---\n"} {
		if ValidateFrontmatter([]byte(content)) == nil {
			t.Fatalf("accepted %q", content)
		}
	}
	out := outputplan.NewOutput(outputplan.OutputSpec{Path: "out", Content: "---\nname: x\ndescription: x\n---\n", TemplateHash: "t", ConfigHash: "c", Policy: outputplan.Policy{ValidateFrontmatter: true}})
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"out": {TemplateHash: "t", ConfigHash: "c", OutputHash: manifest.Hash([]byte(out.Content()))}}}
	got, err := Locked(false, lock, planFor(out), func(string) ([]byte, error) { return []byte("edit"), nil }, empty(t))
	if err != nil || len(got.Findings()) != 1 || got.Findings()[0].Evidence.Kind != "hand-edited" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	_, err = Locked(false, lock, planFor(out), func(string) ([]byte, error) { return nil, os.ErrNotExist }, empty(t))
	if err != nil {
		t.Fatal(err)
	}
	guide := outputplan.NewOutput(outputplan.OutputSpec{Path: "AGENTS.md", Content: strings.Repeat("x", 12*1024+1)})
	advisory, err := GuideSizeAdvisory(planFor(guide))
	if err != nil || len(advisory.Findings()) != 1 {
		t.Fatalf("advisory=%#v err=%v", advisory, err)
	}
	finding := advisory.Findings()[0]
	if finding.Rank != severity.Warn || finding.Property != "heuristic-quality" {
		t.Fatalf("guide advisory classification = rank %v property %q", finding.Rank, finding.Property)
	}
}
func TestGeneratedOwnerExceptionalBranches(t *testing.T) {
	regenLock := &manifest.Lock{Files: map[string]manifest.Entry{"gone": {}}}
	got, err := Locked(false, regenLock, outputplan.Plan{}, func(string) ([]byte, error) { return nil, nil }, empty(t))
	if err != nil || len(got.Findings()) != 1 || got.Findings()[0].Evidence.Kind != "orphaned" {
		t.Fatalf("orphan=%#v err=%v", got, err)
	}
	regen := outputplan.NewOutput(outputplan.OutputSpec{Path: "regen", Content: "x", Policy: outputplan.Policy{Regenerate: true}})
	untracked, err := checkresult.New([]checkresult.Finding{{Rank: 1, Property: PropertyReproducibility, Evidence: checkresult.Evidence{Kind: "untracked", Path: "regen", Detail: "x"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Locked(false, &manifest.Lock{Files: map[string]manifest.Entry{"regen": {}}}, planFor(regen), func(string) ([]byte, error) { return nil, os.ErrNotExist }, untracked); err != nil {
		t.Fatal(err)
	}
	ordinary := outputplan.NewOutput(outputplan.OutputSpec{Path: "ordinary", Content: "x", TemplateHash: "t", ConfigHash: "c"})
	untracked, _ = checkresult.New([]checkresult.Finding{{Rank: 1, Property: PropertyReproducibility, Evidence: checkresult.Evidence{Kind: "untracked", Path: "ordinary", Detail: "x"}}}, nil)
	if _, err = Locked(false, &manifest.Lock{Files: map[string]manifest.Entry{"ordinary": {TemplateHash: "t", ConfigHash: "c", OutputHash: manifest.Hash([]byte("x"))}}}, planFor(ordinary), func(string) ([]byte, error) { return nil, os.ErrNotExist }, untracked); err != nil {
		t.Fatal(err)
	}
	stale, err := Locked(false, &manifest.Lock{Files: map[string]manifest.Entry{"ordinary": {TemplateHash: "t", ConfigHash: "c", OutputHash: manifest.Hash([]byte("old"))}}}, planFor(ordinary), func(string) ([]byte, error) { return []byte("x"), nil }, empty(t))
	if err != nil || stale.Findings()[0].Evidence.Kind != "stale" {
		t.Fatalf("stale=%#v err=%v", stale, err)
	}
	invalid := outputplan.NewOutput(outputplan.OutputSpec{Path: "front", Content: "plain", TemplateHash: "t", ConfigHash: "c", Policy: outputplan.Policy{ValidateFrontmatter: true}})
	got, err = Locked(false, &manifest.Lock{Files: map[string]manifest.Entry{"front": {TemplateHash: "t", ConfigHash: "c", OutputHash: manifest.Hash([]byte("plain"))}}}, planFor(invalid), func(string) ([]byte, error) { return []byte("plain"), nil }, empty(t))
	if err != nil || got.Findings()[0].Evidence.Kind != "invalid-frontmatter" {
		t.Fatalf("front=%#v err=%v", got, err)
	}
	if ValidateFrontmatter([]byte("---\nname: [\n---\n")) == nil {
		t.Fatal("malformed frontmatter accepted")
	}
	staged := outputplan.NewOutput(outputplan.OutputSpec{Path: "staged", Content: "x", TemplateHash: "t", ConfigHash: "c"})
	drift, err := Staged(false, &manifest.Lock{Files: map[string]manifest.Entry{"staged": {TemplateHash: "t", ConfigHash: "c", OutputHash: manifest.Hash([]byte("x"))}}}, planFor(staged), reader{map[string][]byte{"staged": []byte("edit"), ".awf/awf.lock": []byte("lock")}}, map[string]bool{"staged": true, ".awf/awf.lock": true})
	if err != nil || len(drift.Findings()) != 1 || drift.Findings()[0].Evidence.Kind != "hand-edited" {
		t.Fatalf("drift=%#v err=%v", drift, err)
	}
}
func TestStagedReadFailure(t *testing.T) {
	_, err := Staged(false, nil, outputplan.Plan{}, failingReader{}, map[string]bool{".awf/awf.lock": true})
	if err == nil {
		t.Fatal("read failure accepted")
	}
}
func planFor(out outputplan.Output) outputplan.Plan {
	return outputplan.New([]outputplan.Node{outputplan.NewNode(outputplan.NodeSpec{Path: out.Path(), Output: &out})})
}
func empty(t *testing.T) checkresult.Result {
	t.Helper()
	r, err := checkresult.New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

type reader struct{ values map[string][]byte }

func (r reader) ReadFile(p string) ([]byte, bool, error) { return r.values[p], true, nil }
func (reader) Paths(string) ([]string, error)            { return nil, nil }

type failingReader struct{}

func (failingReader) ReadFile(string) ([]byte, bool, error) { return nil, false, errors.New("read") }
func (failingReader) Paths(string) ([]string, error)        { return nil, nil }
