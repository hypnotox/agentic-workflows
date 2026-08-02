package audit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: tooling/audit-and-snapshots:audit-history-operation-owned (TestHistoryOperationCollectsRangeOnceAndCachesStates)
func TestHistoryOperationCollectsRangeOnceAndCachesStates(t *testing.T) {
	ctx := testContext(t)
	commits := []awfgit.Commit{{Hash: "child", Revision: "child-revision", Subject: "feat(awf): child", Parents: []string{"outside-revision"}}}
	collects := 0
	collect := func(context.Context, string, string) ([]awfgit.Commit, error) {
		collects++
		return commits, nil
	}
	loadErr := errors.New("load failed")
	loads := map[string]int{}
	load := func(_ context.Context, revision string) (*revisionState, error) {
		loads[revision]++
		if revision == "broken-revision" {
			return nil, loadErr
		}
		return fixedRevisionState(nil, false, currentstate.Universe{}), nil
	}
	liveCalls := 0
	live := func(context.Context) ([]Finding, error) {
		liveCalls++
		return nil, nil
	}

	op, err := newHistoryOperation(ctx, "base", "head", Inputs{}, collect, load, live)
	if err != nil {
		t.Fatal(err)
	}
	op.transitionFindings(ctx)
	if _, err := op.state(ctx, "broken-revision"); !errors.Is(err, loadErr) {
		t.Fatalf("first cached error = %v", err)
	}
	if _, err := op.state(ctx, "broken-revision"); !errors.Is(err, loadErr) {
		t.Fatalf("second cached error = %v", err)
	}
	if collects != 1 {
		t.Fatalf("range collections = %d, want 1", collects)
	}
	for revision, want := range map[string]int{"child-revision": 1, "outside-revision": 1, "broken-revision": 1} {
		if got := loads[revision]; got != want {
			t.Fatalf("loads[%s] = %d, want %d", revision, got, want)
		}
	}
	if loads["outside-parent-of-outside"] != 0 {
		t.Fatal("operation recursively traversed ancestry outside the selected range")
	}

	second, err := newHistoryOperation(ctx, "base", "head", Inputs{}, collect, load, live)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.state(ctx, "child-revision"); err != nil {
		t.Fatal(err)
	}
	if collects != 2 || loads["child-revision"] != 2 {
		t.Fatalf("separate invocation reused state: collections=%d child loads=%d", collects, loads["child-revision"])
	}
}

// invariant: tooling/audit-and-snapshots:audit-history-operation-owned (TestHistoryOperationSharesStatesAcrossTransitionAndStaleReplay)
func TestHistoryOperationSharesStatesAcrossTransitionAndStaleReplay(t *testing.T) {
	ctx := testContext(t)
	source := []byte(staleADR(adr.CurrentStateV1, "0001"))
	record, err := adr.ParseRecord("0001-old.md", source)
	if err != nil {
		t.Fatal(err)
	}
	claim := topic.Claim{ID: "alpha/one:owned", Slug: "owned", Type: topic.Invariant, Prose: "Owned.", Origin: "legacy", Backing: topic.ExplicitNoBacking}
	first := currentstate.Universe{Topics: []topic.Topic{{ID: topic.TopicID{Domain: "alpha", Slug: "one"}, Claims: []topic.Claim{claim}}}}
	withRecord := currentstate.Universe{ADRs: []adr.ADR{record}, Sources: map[string][]byte{"0001": source}}
	states := map[string]*revisionState{
		"ordinary": fixedRevisionState(nil, false, currentstate.Universe{}),
		"result":   fixedRevisionState(&manifest.Lock{SchemaVersion: 31}, true, withRecord),
		"first":    fixedRevisionState(&manifest.Lock{SchemaVersion: 31}, true, first),
		"incoming": fixedRevisionState(&manifest.Lock{SchemaVersion: 31}, true, withRecord),
	}
	commits := []awfgit.Commit{
		{Hash: "pure", Revision: "ordinary", Subject: "not conventional"},
		{Hash: "merge", Revision: "result", Subject: "Merge feature", Message: "Merge feature", Parents: []string{"first", "incoming"}, IsMerge: true},
	}
	loads := map[string]int{}
	load := func(_ context.Context, revision string) (*revisionState, error) {
		loads[revision]++
		state, ok := states[revision]
		if !ok {
			return nil, errors.New("unexpected revision " + revision)
		}
		return state, nil
	}
	liveCalls := 0
	op, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) { return commits, nil },
		load,
		func(context.Context) ([]Finding, error) {
			liveCalls++
			return []Finding{{Severity: severity.Error, Rule: "live-cleanliness"}}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	findings, err := op.run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotRules := make([]string, len(findings))
	for i, finding := range findings {
		gotRules[i] = finding.Rule
	}
	want := "conventional-commits,stale-merge-authorization,live-cleanliness,current-state-transition"
	if got := strings.Join(gotRules, ","); got != want {
		t.Fatalf("finding order = %s, want %s; findings=%#v", got, want, findings)
	}
	for revision := range states {
		if loads[revision] != 1 {
			t.Fatalf("loads[%s] = %d, want 1", revision, loads[revision])
		}
	}
	if liveCalls != 1 {
		t.Fatalf("live calls = %d, want 1", liveCalls)
	}
}

func TestHistoryOperationErrorPaths(t *testing.T) {
	ctx := testContext(t)
	boom := errors.New("boom")
	emptyLive := func(context.Context) ([]Finding, error) { return nil, nil }
	if _, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) { return nil, boom }, nil, emptyLive); !errors.Is(err, boom) {
		t.Fatalf("collection error = %v", err)
	}

	nilOp, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) { return nil, nil },
		func(context.Context, string) (*revisionState, error) {
			//nolint:nilnil // this deliberately malformed dependency exercises the operation's fail-closed guard
			return nil, nil
		}, emptyLive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nilOp.state(ctx, "nil"); err == nil || !strings.Contains(err.Error(), "no state") {
		t.Fatalf("nil revision state error = %v", err)
	}

	liveOp, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) { return nil, nil },
		func(context.Context, string) (*revisionState, error) { return nil, boom },
		func(context.Context) ([]Finding, error) { return nil, boom })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := liveOp.run(ctx); !errors.Is(err, boom) {
		t.Fatalf("live error = %v", err)
	}

	staleOp, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) {
			return []awfgit.Commit{{Hash: "merge", Revision: "bad", IsMerge: true}}, nil
		},
		func(context.Context, string) (*revisionState, error) { return nil, boom }, emptyLive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := staleOp.run(ctx); !errors.Is(err, boom) {
		t.Fatalf("stale error = %v", err)
	}

	malformedLock := revisionStateFromTree(t.TempDir(), auditTree(t, []snapshot.File{{
		Path: ".awf/awf.lock", Mode: snapshot.Regular, Bytes: []byte("{"),
	}}))
	if _, err := malformedLock.currentState(); err == nil {
		t.Fatal("current state accepted malformed cached lock")
	}

	badUniverse := func(err error) *revisionState {
		return &revisionState{lockReady: true, universeReady: true, universeErr: err}
	}
	for _, tc := range []struct {
		name   string
		commit awfgit.Commit
		load   revisionLoader
	}{
		{"result load", awfgit.Commit{Hash: "c", Revision: "result"}, func(context.Context, string) (*revisionState, error) { return nil, boom }},
		{"result parse", awfgit.Commit{Hash: "c", Revision: "result"}, func(context.Context, string) (*revisionState, error) { return badUniverse(boom), nil }},
		{"parent load", awfgit.Commit{Hash: "c", Revision: "result", Parents: []string{"parent"}}, func(_ context.Context, revision string) (*revisionState, error) {
			if revision == "parent" {
				return nil, boom
			}
			return fixedRevisionState(nil, false, currentstate.Universe{}), nil
		}},
		{"parent parse", awfgit.Commit{Hash: "c", Revision: "result", Parents: []string{"parent"}}, func(_ context.Context, revision string) (*revisionState, error) {
			if revision == "parent" {
				return badUniverse(boom), nil
			}
			return fixedRevisionState(nil, false, currentstate.Universe{}), nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op, err := newHistoryOperation(ctx, "base", "head", Inputs{},
				func(context.Context, string, string) ([]awfgit.Commit, error) { return []awfgit.Commit{tc.commit}, nil }, tc.load, emptyLive)
			if err != nil {
				t.Fatal(err)
			}
			findings := op.transitionFindings(ctx)
			if len(findings) != 1 || findings[0].Severity != severity.Warn || !strings.Contains(findings[0].Detail, boom.Error()) {
				t.Fatalf("transition findings = %#v", findings)
			}
		})
	}

	result := fixedRevisionState(&manifest.Lock{SchemaVersion: 31}, true, currentstate.Universe{})
	firstError, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) {
			return []awfgit.Commit{{Hash: "merge", Revision: "result", IsMerge: true, Parents: []string{"first", "incoming"}}}, nil
		},
		func(_ context.Context, revision string) (*revisionState, error) {
			if revision == "result" {
				return result, nil
			}
			return nil, boom
		}, emptyLive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := firstError.staleMergeFindings(ctx); !errors.Is(err, boom) || !strings.Contains(err.Error(), "first parent") {
		t.Fatalf("first-parent error = %v", err)
	}
}

func fixedRevisionState(lock *manifest.Lock, found bool, universe currentstate.Universe) *revisionState {
	return &revisionState{
		lockReady: true, lock: lock, lockFound: found,
		universeReady: true, universe: universe,
	}
}

func TestHistoryOperationEmptyRangeRunsLiveOnce(t *testing.T) {
	loads, liveCalls := 0, 0
	op, err := newHistoryOperation(testContext(t), "same", "same", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) { return nil, nil },
		func(context.Context, string) (*revisionState, error) {
			loads++
			return fixedRevisionState(nil, false, currentstate.Universe{}), nil
		},
		func(context.Context) ([]Finding, error) {
			liveCalls++
			return nil, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if findings, err := op.run(testContext(t)); err != nil || len(findings) != 0 {
		t.Fatalf("run = %#v, %v", findings, err)
	}
	if liveCalls != 1 || loads != 0 {
		t.Fatalf("live calls=%d revision loads=%d, want 1 and 0", liveCalls, loads)
	}
}
