package authoringop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func transactionFixture(t *testing.T, local bool) (string, *project.Loader) {
	t.Helper()
	root := t.TempDir()
	localConfig := ""
	if local {
		localConfig = "localDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n"
	}
	writeFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: make gate}\n"+localConfig)
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
	state, cfg, err := loader.OpenForOperation(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(root), project.Version).Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatal(err)
	}
	return root, loader
}

func bytesAt(t *testing.T, root, rel string) []byte {
	t.Helper()
	bytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

// invariant: rendering/sync-and-drift:authoring-sync-transaction (TestPartAuthoringValidatesBeforeSourceAndSynchronizesCommittedAuthority)
func TestPartAuthoringValidatesBeforeSourceAndSynchronizesCommittedAuthority(t *testing.T) {
	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["tdd"].Sections[0]
	request := Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("authored body\n")}
	outputPath := ".claude/skills/example-tdd/SKILL.md"
	outputBefore := bytesAt(t, root, outputPath)
	lockBefore := bytesAt(t, root, ".awf/awf.lock")
	outcome, err := Run(context.Background(), root, request, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Source != SourceCreated || !outcome.Publisher.HasCommittedEffects() {
		t.Fatalf("outcome = %#v", outcome)
	}
	if got := string(bytesAt(t, root, outcome.SourcePath)); got != "authored body\n" {
		t.Fatalf("source = %q", got)
	}
	if outputAfter := bytesAt(t, root, outputPath); string(outputAfter) == string(outputBefore) || !strings.Contains(string(outputAfter), "authored body\n") {
		t.Fatalf("successful authoring did not synchronize committed source to output: %q", outputAfter)
	}
	if string(bytesAt(t, root, ".awf/awf.lock")) == string(lockBefore) {
		t.Fatal("successful authoring did not update lock")
	}

	// Explicit empty content remains an authored override rather than reset.
	outcome, err = Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte{}}, loader, nil)
	if err != nil || outcome.Source != SourceReplaced {
		t.Fatalf("empty edit outcome=%#v err=%v", outcome, err)
	}
	if got := bytesAt(t, root, outcome.SourcePath); len(got) != 0 {
		t.Fatalf("empty override = %q", got)
	}

	outcome, err = Run(context.Background(), root, Request{Mode: Reset, Kind: "skill", Name: "tdd", Part: section}, loader, nil)
	if err != nil || outcome.Source != SourceRemoved {
		t.Fatalf("reset outcome=%#v err=%v", outcome, err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(outcome.SourcePath))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reset retained source: %v", err)
	}
}

// invariant: tooling/cli:semantic-artifact-authoring (TestInvalidCandidateLeavesSourceOutputAndLockUnchanged)
func TestInvalidCandidateLeavesSourceOutputAndLockUnchanged(t *testing.T) {
	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["tdd"].Sections[0]
	output := ".claude/skills/example-tdd/SKILL.md"
	beforeOutput := bytesAt(t, root, output)
	beforeLock := bytesAt(t, root, ".awf/awf.lock")
	_, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("{{=awf:notDeclared}}\n")}, loader, nil)
	if err == nil || !strings.Contains(err.Error(), "candidate") {
		t.Fatalf("invalid candidate error = %v", err)
	}
	part := filepath.Join(root, ".awf/skills/parts/tdd", section+".md")
	if _, statErr := os.Stat(part); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid candidate changed source: %v", statErr)
	}
	if string(bytesAt(t, root, output)) != string(beforeOutput) || string(bytesAt(t, root, ".awf/awf.lock")) != string(beforeLock) {
		t.Fatal("invalid candidate changed output or lock")
	}
}

// invariant: rendering/inplace-and-placeholders:local-doc-body-inline (TestLocalDocumentBodyEditAndResetPreserveShell)
func TestLocalDocumentBodyEditAndResetPreserveShell(t *testing.T) {
	root, loader := transactionFixture(t, true)
	path := "docs/runbooks/incident.md"
	original := bytesAt(t, root, path)
	pointerStart := strings.Index(string(original), "<!-- awf:edit-in-place body")
	if pointerStart < 0 {
		t.Fatalf("rendered local document body pointer missing: %q", original)
	}
	pointerLineEnd := strings.Index(string(original[pointerStart:]), "\n")
	if pointerLineEnd < 0 {
		t.Fatalf("rendered local document body pointer is unterminated: %q", original)
	}
	pointerEnd := pointerStart + pointerLineEnd + 1
	ownedPrefix := string(original[:pointerEnd])
	outcome, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "doc", Name: "runbooks/incident", Part: "body", Content: []byte("operator body\n\nexact")}, loader, nil)
	if err != nil || outcome.Source != SourceLocalBody {
		t.Fatalf("local edit outcome=%#v err=%v", outcome, err)
	}
	edited := bytesAt(t, root, path)
	if !strings.HasPrefix(string(edited), ownedPrefix) || string(edited) != ownedPrefix+"operator body\n\nexact\n" {
		t.Fatalf("edited local document did not retain its complete awf-owned prefix and final body section: %q", edited)
	}
	_, err = Run(context.Background(), root, Request{Mode: Reset, Kind: "doc", Name: "runbooks/incident", Part: "body"}, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	reset := bytesAt(t, root, path)
	if !strings.HasPrefix(string(reset), ownedPrefix) || string(reset) != ownedPrefix+"\n" {
		t.Fatalf("reset local document did not retain its complete awf-owned prefix and final empty body section: %q", reset)
	}
}

// invariant: rendering/sync-and-drift:authoring-sync-transaction (TestCommittedMutationSourceEffectIncludesResetRemoval)
func TestCommittedMutationSourceEffectIncludesResetRemoval(t *testing.T) {
	cases := []struct {
		name    string
		request Request
		local   bool
		existed bool
		want    SourceEffect
	}{
		{name: "created", request: Request{Mode: Edit}, want: SourceCreated},
		{name: "replaced", request: Request{Mode: Edit}, existed: true, want: SourceReplaced},
		{name: "removed", request: Request{Mode: Reset}, existed: true, want: SourceRemoved},
		{name: "local body", request: Request{Mode: Reset}, local: true, existed: true, want: SourceLocalBody},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := committedSourceEffect(tc.request, tc.local, tc.existed); got != tc.want {
				t.Fatalf("committed source effect = %q, want %q", got, tc.want)
			}
		})
	}
}

// invariant: rendering/sync-and-drift:authoring-sync-transaction (TestPostRunCommittedPlainFaultNormalizesToPartial)
func TestPostRunCommittedPlainFaultNormalizesToPartial(t *testing.T) {
	fault := errors.New("handle close sentinel")
	outcome := Outcome{Source: SourceCreated, Residue: []string{"stale.awf-tmp"}}
	err := normalizePostRunError(outcome, fault)
	partial, ok := AsPartial(err)
	if !ok || !errors.Is(err, fault) || partial.Outcome.Source != outcome.Source || len(partial.Outcome.Residue) != 1 || partial.Outcome.Residue[0] != "stale.awf-tmp" {
		t.Fatalf("normalized error = %#v, want typed partial retaining %#v and %v", err, outcome, fault)
	}
	if len(partial.Recovery) < 2 || partial.Recovery[0] != "remove the reported publication residue" {
		t.Fatalf("recovery is not residue-first: %#v", partial.Recovery)
	}
	if setupPartial, ok := AsPartial(normalizePostRunError(Outcome{CreatedParents: []string{"parts"}}, fault)); !ok || !errors.Is(setupPartial, fault) || len(setupPartial.Outcome.CreatedParents) != 1 {
		t.Fatalf("setup committed plain fault lost typed effects: %#v", setupPartial)
	}

	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["tdd"].Sections[0]
	committedOutcome, runErr := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("body")}, loader, nil)
	if runErr != nil || !committedOutcome.Publisher.HasCommittedEffects() {
		t.Fatalf("publisher fixture outcome = %#v, %v", committedOutcome, runErr)
	}
	if publisherPartial, ok := AsPartial(normalizePostRunError(committedOutcome, fault)); !ok || !errors.Is(publisherPartial, fault) || !publisherPartial.Outcome.Publisher.HasCommittedEffects() {
		t.Fatalf("publisher committed plain fault lost typed effects: %#v", publisherPartial)
	}
}

// invariant: rendering/sync-and-drift:authoring-sync-transaction (TestCandidateOverlayServesProjectAndConfigReaderContracts)
func TestCandidateOverlayServesProjectAndConfigReaderContracts(t *testing.T) {
	root := t.TempDir()
	path := ".awf/skills/parts/tdd/guide.md"
	writeFile(t, filepath.Join(root, filepath.FromSlash(path)), "base")
	overlay := candidateOverlay{base: publisher.NewFilesystemReader(root), path: path, bytes: []byte("candidate"), present: true}
	projectBytes, projectFound, err := overlay.ReadFile(path)
	if err != nil || !projectFound || string(projectBytes) != "candidate" {
		t.Fatalf("project tree candidate = %q, %v, %v", projectBytes, projectFound, err)
	}
	projectBytes[0] = 'X'
	configBytes, configFound := (configOverlay{tree: overlay}).ReadFile("skills/parts/tdd/guide.md")
	if !configFound || string(configBytes) != "candidate" {
		t.Fatalf("config tree candidate = %q, %v", configBytes, configFound)
	}
	projectPaths := mustOverlayPaths(t, overlay, ".awf/skills")
	if len(projectPaths) != 1 || projectPaths[0] != path {
		t.Fatalf("project tree paths = %#v", projectPaths)
	}
	configPaths := (configOverlay{tree: overlay}).Paths("skills")
	if len(configPaths) != 1 || configPaths[0] != "skills/parts/tdd/guide.md" {
		t.Fatalf("config tree paths = %#v", configPaths)
	}
	removed := overlay
	removed.present = false
	if _, found, err := removed.ReadFile(path); err != nil || found {
		t.Fatalf("project tree removal = found %v, error %v", found, err)
	}
	if _, found := (configOverlay{tree: removed}).ReadFile("skills/parts/tdd/guide.md"); found {
		t.Fatal("config tree removal remained present")
	}
	if paths := mustOverlayPaths(t, removed, ".awf/skills"); len(paths) != 0 {
		t.Fatalf("project tree removal paths = %#v", paths)
	}
	if paths := (configOverlay{tree: removed}).Paths("skills"); len(paths) != 0 {
		t.Fatalf("config tree removal paths = %#v", paths)
	}
}

func mustOverlayPaths(t *testing.T, overlay candidateOverlay, prefix string) []string {
	t.Helper()
	paths, err := overlay.Paths(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// invariant: rendering/sync-and-drift:authoring-sync-transaction (TestRunAcquiresLeaseBeforeAuthorityRead)
func TestRunAcquiresLeaseBeforeAuthorityRead(t *testing.T) {
	root, initializer := transactionFixture(t, false)
	leaseAcquired := false
	observeAuthority := false
	loader := project.NewLoaderWithoutRepository(func(path string) (*config.Config, error) {
		if observeAuthority && !leaseAcquired {
			t.Fatal("authority read before project lease acquisition")
		}
		return config.Load(path)
	}, catalog.Standard, func(context.Context, string) string { return root })
	acquire := func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		leaseAcquired = true
		lease, err := initializer.AcquireProjectLease(ctx, root)
		if err != nil {
			return nil, nil, err
		}
		return lease, lease.Release, nil
	}
	observeAuthority = true
	section := catalog.Standard.Skills["tdd"].Sections[0]
	if _, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("body")}, loader, acquire); err != nil {
		t.Fatal(err)
	}
}

// invariant: rendering/sync-and-drift:authoring-sync-transaction (TestPublisherFaultRetainsCommittedSourceAndRecovery)
// invariant: tooling/cli:semantic-artifact-authoring (TestPublisherFaultRetainsCommittedSourceAndRecovery)
func TestPublisherFaultRetainsCommittedSourceAndRecovery(t *testing.T) {
	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["tdd"].Sections[0]
	blockedOutput := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	if err := os.Remove(blockedOutput); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blockedOutput, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(blockedOutput, "blocker"), "preserved")

	outcome, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("committed before publisher fault\n")}, loader, nil)
	partial, ok := AsPartial(err)
	if !ok {
		t.Fatalf("publisher fault = %v, want typed partial", err)
	}
	if outcome.Source != SourceCreated || partial.Outcome.Source != SourceCreated {
		t.Fatalf("publisher fault lost source effect: outcome=%#v partial=%#v", outcome, partial)
	}
	if got := string(bytesAt(t, root, outcome.SourcePath)); got != "committed before publisher fault\n" {
		t.Fatalf("publisher fault source = %q", got)
	}
	if len(partial.Recovery) == 0 || !strings.Contains(partial.Recovery[len(partial.Recovery)-1], "repair the publisher fault") {
		t.Fatalf("publisher recovery = %#v", partial.Recovery)
	}
	if got := string(bytesAt(t, root, ".claude/skills/example-tdd/SKILL.md/blocker")); got != "preserved" {
		t.Fatalf("publisher fault changed blocking entry = %q", got)
	}
}

// invariant: rendering/sync-and-drift:authoring-sync-transaction (TestLeaseReleaseFaultRetainsEffectsAndCause)
func TestLeaseReleaseFaultRetainsEffectsAndCause(t *testing.T) {
	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["tdd"].Sections[0]
	releaseFault := errors.New("release sentinel")
	acquire := func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		lease, err := loader.AcquireProjectLease(ctx, root)
		if err != nil {
			return nil, nil, err
		}
		return lease, func() error { return errors.Join(lease.Release(), releaseFault) }, nil
	}
	outcome, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("body")}, loader, acquire)
	var partial *PartialError
	if !errors.As(err, &partial) || !errors.Is(err, releaseFault) {
		t.Fatalf("release error = %v", err)
	}
	if outcome.Source == SourceNone || !outcome.Publisher.HasCommittedEffects() || partial.Outcome.Source != outcome.Source {
		t.Fatalf("release partial lost effects: outcome=%#v partial=%#v", outcome, partial)
	}
	if len(partial.Recovery) == 0 || !strings.Contains(partial.Recovery[len(partial.Recovery)-1], "lease-release fault") {
		t.Fatalf("lease-release recovery = %#v", partial.Recovery)
	}
	if _, documentErr := partial.Document(); documentErr != nil {
		t.Fatalf("partial document error=%v", documentErr)
	}
}
