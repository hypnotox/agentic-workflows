package audit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Legacy focused assertions use these test-only adapters; production replay
// always executes the single interleaved graph schedule through run.
func (h *historyOperation) transitionFindings(ctx context.Context) ([]Finding, error) {
	graph, err := newReplayGraph(ctx, h.commits)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, commit := range graph.schedule {
		step, err := h.replayTransition(ctx, commit)
		if err != nil {
			return nil, err
		}
		findings = append(findings, step...)
	}
	return findings, nil
}

func (h *historyOperation) staleMergeFindings(ctx context.Context) ([]Finding, error) {
	graph, err := newReplayGraph(ctx, h.commits)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, commit := range graph.schedule {
		step, err := h.replayStale(ctx, commit)
		if err != nil {
			return nil, err
		}
		findings = append(findings, step...)
	}
	return findings, nil
}

func TestRevisionStoreTracksLogicalHeavyHighWater(t *testing.T) {
	store := newRevisionStore()
	first := &revisionEntry{result: revisionResult{state: fixedRevisionState(nil, false, currentstate.Universe{})}}
	second := &revisionEntry{result: revisionResult{state: fixedRevisionState(nil, false, currentstate.Universe{})}}
	store.retainHeavy(first)
	store.retainHeavy(second)
	store.releaseHeavy(first)
	store.retainHeavy(first)
	store.releaseHeavy(second)
	store.releaseHeavy(first)
	if store.currentHeavy != 0 || store.highWaterHeavy != 2 {
		t.Fatalf("logical heavy entries current=%d high-water=%d, want 0 and 2", store.currentHeavy, store.highWaterHeavy)
	}
}

func TestHistoryOperationReleasesAliasedHeavyStateAtFinalUse(t *testing.T) {
	loads := map[string]int{}
	parent := &revisionState{lockReady: true, configReady: true, config: &config.Config{}}
	parent.loadUniverse = func() (currentstate.Universe, error) { return currentstate.Universe{}, nil }
	op := newHistoryOperationFromCompact([]replayCommit{
		{Hash: "child", Revision: "child", Parents: []string{"parent"}, Paths: []string{"internal/code.go"}},
		{Hash: "parent", Revision: "parent"},
	}, nil, 2, func(_ context.Context, revision string) (*revisionState, error) {
		loads[revision]++
		if revision != "parent" {
			return nil, errors.New("unexpected distinct load " + revision)
		}
		return parent, nil
	}, func(context.Context, string) ([]string, error) { return nil, nil }, func(context.Context) ([]Finding, error) { return nil, nil })
	if _, err := op.run(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if loads["parent"] != 1 || loads["child"] != 0 {
		t.Fatalf("light alias loads=%#v, want one parent load", loads)
	}
	if op.store.currentHeavy != 0 || op.store.highWaterHeavy != 1 || parent.loadUniverse != nil || parent.universe.Sources != nil || len(parent.universe.ADRs) != 0 || len(parent.universe.Topics) != 0 {
		t.Fatalf("final release current=%d high-water=%d loader=%v universe=%#v", op.store.currentHeavy, op.store.highWaterHeavy, parent.loadUniverse != nil, parent.universe)
	}
}

func TestHistoryOperationPreResolvesRecursiveAliasesAndFrontier(t *testing.T) {
	commits := []replayCommit{
		{Hash: "leaf", Revision: "zz-leaf", Parents: []string{"yy-middle"}, Paths: []string{"internal/leaf.go"}},
		{Hash: "middle", Revision: "yy-middle", Parents: []string{"aa-parent"}, Paths: []string{"internal/middle.go"}},
		{Hash: "sibling", Revision: "xx-sibling", Parents: []string{"aa-parent"}, Paths: []string{"internal/sibling.go"}},
		{Hash: "relevant", Revision: "ww-relevant", Parents: []string{"aa-parent"}, Paths: []string{".awf/config.yaml"}},
		{Hash: "parent", Revision: "aa-parent"},
	}
	lightLoads := map[string]int{}
	heavyLoads := map[string]int{}
	states := map[string]*revisionState{}
	op := newHistoryOperationFromCompact(commits, nil, len(commits), func(_ context.Context, revision string) (*revisionState, error) {
		lightLoads[revision]++
		state := &revisionState{lockReady: true, configReady: true, config: &config.Config{}}
		state.loadUniverse = func() (currentstate.Universe, error) {
			heavyLoads[revision]++
			return currentstate.Universe{}, nil
		}
		states[revision] = state
		return state, nil
	}, func(context.Context, string) ([]string, error) { return nil, nil }, func(context.Context) ([]Finding, error) { return nil, nil })
	if findings, err := op.run(testContext(t)); err != nil || len(findings) != 0 {
		t.Fatalf("recursive alias replay findings = %#v, error = %v", findings, err)
	}
	if lightLoads["aa-parent"] != 1 || lightLoads["ww-relevant"] != 1 || lightLoads["yy-middle"] != 0 || lightLoads["zz-leaf"] != 0 || lightLoads["xx-sibling"] != 0 {
		t.Fatalf("recursive alias light loads = %#v", lightLoads)
	}
	if heavyLoads["aa-parent"] != 1 || heavyLoads["ww-relevant"] != 1 || len(heavyLoads) != 2 {
		t.Fatalf("canonical heavy loads = %#v, want parent and relevant once", heavyLoads)
	}
	if op.store.currentHeavy != 0 || op.store.highWaterHeavy != 2 || len(op.store.keys) != 0 || len(op.store.entries) != 0 {
		t.Fatalf("terminal ownership current=%d high-water=%d keys=%d entries=%d", op.store.currentHeavy, op.store.highWaterHeavy, len(op.store.keys), len(op.store.entries))
	}
	for revision, state := range states {
		if state.loadUniverse != nil || state.config != nil || state.universeErr != nil {
			t.Fatalf("revision %s retained controls or heavy outcome: %#v", revision, state)
		}
	}
}

func TestHistoryOperationAliasHistoryLengthKeepsTheSameHeavyFrontier(t *testing.T) {
	for _, length := range []int{1, 32} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			root := &revisionState{lockReady: true, configReady: true, config: &config.Config{}}
			root.loadUniverse = func() (currentstate.Universe, error) { return currentstate.Universe{}, nil }
			commits := make([]replayCommit, 0, length+1)
			commits = append(commits, replayCommit{Hash: "root", Revision: "00-root"})
			parent := "00-root"
			for i := range length {
				revision := fmt.Sprintf("%02d-alias", i+1)
				commits = append(commits, replayCommit{Hash: revision, Revision: revision, Parents: []string{parent}, Paths: []string{"internal/code.go"}})
				parent = revision
			}
			loads := 0
			op := newHistoryOperationFromCompact(commits, nil, len(commits), func(_ context.Context, revision string) (*revisionState, error) {
				loads++
				if revision != "00-root" {
					return nil, fmt.Errorf("unexpected distinct alias load %s", revision)
				}
				return root, nil
			}, func(context.Context, string) ([]string, error) { return nil, nil }, func(context.Context) ([]Finding, error) { return nil, nil })
			if _, err := op.run(testContext(t)); err != nil {
				t.Fatal(err)
			}
			if loads != 1 || op.store.highWaterHeavy != 1 || op.store.currentHeavy != 0 {
				t.Fatalf("alias length %d loads=%d high-water=%d current=%d, want 1, 1, 0", length, loads, op.store.highWaterHeavy, op.store.currentHeavy)
			}
		})
	}
}

func TestHistoryOwnershipPlanningPropagatesNestedMergeCancellation(t *testing.T) {
	boom := context.Canceled
	for _, tc := range []struct {
		name  string
		outer replayCommit
	}{
		{"selected first parent", replayCommit{Hash: "outer", Revision: "outer", Parents: []string{"nested"}, Paths: []string{"internal/outer.go"}}},
		{"selected incoming parent", replayCommit{Hash: "outer", Revision: "outer", IsMerge: true, Parents: []string{"outer-first", "nested"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commits := []replayCommit{
				tc.outer,
				{Hash: "nested", Revision: "nested", IsMerge: true, Parents: []string{"nested-first", "nested-incoming"}},
			}
			op := newHistoryOperationFromCompact(commits, nil, len(commits), func(context.Context, string) (*revisionState, error) {
				return &revisionState{lockReady: true, configReady: true, config: &config.Config{}, universeReady: true}, nil
			}, func(_ context.Context, revision string) ([]string, error) {
				if revision == "nested" {
					return nil, boom
				}
				return nil, nil
			}, func(context.Context) ([]Finding, error) { return nil, nil })
			graph, err := newReplayGraph(testContext(t), commits)
			if err != nil {
				t.Fatal(err)
			}
			if err := op.planRevisionOwnership(testContext(t), graph.schedule); !errors.Is(err, boom) {
				t.Fatalf("nested merge planning error = %v, want %v", err, boom)
			}
		})
	}
}

func TestHistoryOwnershipPlanningStopsAfterAliasResolutionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(testContext(t))
	commits := []replayCommit{
		{Hash: "leaf", Revision: "a-leaf", Parents: []string{"b-middle"}, Paths: []string{"internal/leaf.go"}},
		{Hash: "middle", Revision: "b-middle", Parents: []string{"c-root"}, Paths: []string{"internal/middle.go"}},
		{Hash: "root", Revision: "c-root"},
		{Hash: "later", Revision: "z-later"},
	}
	loads := map[string]int{}
	op := newHistoryOperationFromCompact(commits, nil, len(commits), func(_ context.Context, revision string) (*revisionState, error) {
		loads[revision]++
		if revision == "c-root" {
			cancel()
		}
		return &revisionState{lockReady: true, configReady: true, config: &config.Config{}, universeReady: true}, nil
	}, func(context.Context, string) ([]string, error) { return nil, nil }, func(context.Context) ([]Finding, error) { return nil, nil })
	graph, err := newReplayGraph(testContext(t), commits)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.planRevisionOwnership(ctx, graph.schedule); !errors.Is(err, context.Canceled) {
		t.Fatalf("alias planning cancellation = %v, loads = %#v", err, loads)
	}
	if loads["z-later"] != 0 {
		t.Fatalf("alias planning continued to later revision: loads = %#v", loads)
	}
}

func TestHistoryOwnershipPlanningObservesEachCancellationCheckpoint(t *testing.T) {
	commits := []replayCommit{{Hash: "child", Revision: "child", Parents: []string{"parent"}}}
	run := func(ctx context.Context) error {
		op := newHistoryOperationFromCompact(commits, nil, len(commits), func(context.Context, string) (*revisionState, error) {
			return &revisionState{lockReady: true, configReady: true, config: &config.Config{}, universeReady: true}, nil
		}, nil, func(context.Context) ([]Finding, error) { return nil, nil })
		return op.planRevisionOwnership(ctx, commits)
	}
	baseline := &cancellationCheckpointContext{Context: testContext(t)}
	if err := run(baseline); err != nil {
		t.Fatal(err)
	}
	for cancelAt := 1; cancelAt <= baseline.checks; cancelAt++ {
		ctx := &cancellationCheckpointContext{Context: testContext(t), cancelAt: cancelAt}
		if err := run(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("ownership cancellation checkpoint %d/%d = %v", cancelAt, baseline.checks, err)
		}
	}
}

func TestHistoryOperationKeepsBoundaryRevisionsDistinct(t *testing.T) {
	commits := []replayCommit{
		{Hash: "left", Revision: "left", Parents: []string{"boundary-left"}, Paths: []string{"internal/left.go"}},
		{Hash: "right", Revision: "right", Parents: []string{"boundary-right"}, Paths: []string{"internal/right.go"}},
	}
	loads := map[string]int{}
	op := newHistoryOperationFromCompact(commits, nil, len(commits), func(_ context.Context, revision string) (*revisionState, error) {
		loads[revision]++
		state := &revisionState{lockReady: true, configReady: true, config: &config.Config{}}
		state.loadUniverse = func() (currentstate.Universe, error) { return currentstate.Universe{}, nil }
		return state, nil
	}, func(context.Context, string) ([]string, error) { return nil, nil }, func(context.Context) ([]Finding, error) { return nil, nil })
	if _, err := op.run(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if loads["boundary-left"] != 1 || loads["boundary-right"] != 1 || loads["left"] != 0 || loads["right"] != 0 {
		t.Fatalf("boundary resolution loads = %#v", loads)
	}
	if op.store.highWaterHeavy != 1 || len(op.store.keys) != 0 || len(op.store.entries) != 0 {
		t.Fatalf("boundary frontier high-water=%d keys=%d entries=%d", op.store.highWaterHeavy, len(op.store.keys), len(op.store.entries))
	}
}

func TestHistoryOperationDischargesSkippedAndFailedConsumers(t *testing.T) {
	t.Run("pre-schema merge", func(t *testing.T) {
		commits := []replayCommit{{Hash: "merge", Revision: "result", IsMerge: true, Parents: []string{"first", "incoming"}}}
		heavyLoads := map[string]int{}
		op := newHistoryOperationFromCompact(commits, nil, 1, func(_ context.Context, revision string) (*revisionState, error) {
			state := &revisionState{lockReady: true, lock: &manifest.Lock{SchemaVersion: 30}, lockFound: true, configReady: true, config: &config.Config{}}
			state.loadUniverse = func() (currentstate.Universe, error) {
				heavyLoads[revision]++
				return currentstate.Universe{Sources: map[string][]byte{revision: []byte(revision)}}, nil
			}
			return state, nil
		}, nil, func(context.Context) ([]Finding, error) { return nil, nil })
		if err := op.planRevisionOwnership(testContext(t), commits); err != nil {
			t.Fatal(err)
		}
		op.reserveConsumers(commits)
		if _, err := op.replayStale(testContext(t), commits[0]); err != nil {
			t.Fatal(err)
		}
		if op.store.keys["incoming"] != nil || op.store.keys["result"] == nil || op.store.keys["first"] == nil {
			t.Fatalf("pre-schema stale discharge keys = %#v", op.store.keys)
		}
		for _, revision := range []string{"result", "first"} {
			entry := op.store.keys[revision].entry
			if entry.sourceUses != 0 || entry.universeUses != 1 || entry.lightUses != 1 {
				t.Fatalf("pre-schema %s remaining uses = light %d source %d universe %d", revision, entry.lightUses, entry.sourceUses, entry.universeUses)
			}
		}
		resultState, err := op.state(testContext(t), "result")
		if err != nil {
			t.Fatal(err)
		}
		resultUniverse, err := op.currentState("result", resultState)
		if err != nil {
			t.Fatal(err)
		}
		if resultUniverse.Sources != nil || resultState.universe.Sources != nil || !resultState.heavyLive {
			t.Fatalf("post-stale materialization retained sources: returned=%#v state=%#v", resultUniverse.Sources, resultState.universe.Sources)
		}
		if _, err := op.replayTransition(testContext(t), commits[0]); err != nil {
			t.Fatal(err)
		}
		if heavyLoads["result"] != 1 || heavyLoads["first"] != 1 || heavyLoads["incoming"] != 0 {
			t.Fatalf("pre-schema heavy loads = %#v", heavyLoads)
		}
		if op.store.currentHeavy != 0 || len(op.store.keys) != 0 || len(op.store.entries) != 0 {
			t.Fatalf("pre-schema replay retained ownership: current=%d keys=%d entries=%d", op.store.currentHeavy, len(op.store.keys), len(op.store.entries))
		}
	})

	t.Run("cached heavy error", func(t *testing.T) {
		boom := errors.New("heavy failure")
		heavyLoads := 0
		state := &revisionState{lockReady: true, configReady: true, config: &config.Config{}}
		state.loadUniverse = func() (currentstate.Universe, error) { heavyLoads++; return currentstate.Universe{}, boom }
		op := newHistoryOperationFromCompact([]replayCommit{{Hash: "ordinary", Revision: "ordinary"}}, nil, 1,
			func(context.Context, string) (*revisionState, error) { return state, nil }, nil,
			func(context.Context) ([]Finding, error) { return nil, nil })
		if err := op.planRevisionOwnership(testContext(t), op.commits); err != nil {
			t.Fatal(err)
		}
		op.reserveConsumers(op.commits)
		findings, err := op.replayTransition(testContext(t), op.commits[0])
		if err != nil || countRule(findings, currentStateTransitionRule, severity.Warn) != 1 {
			t.Fatalf("heavy failure findings = %#v, error = %v", findings, err)
		}
		if heavyLoads != 1 || state.universeErr != nil || state.loadUniverse != nil || len(op.store.keys) != 0 || len(op.store.entries) != 0 {
			t.Fatalf("heavy failure retained outcome: loads=%d state=%#v keys=%d entries=%d", heavyLoads, state, len(op.store.keys), len(op.store.entries))
		}
	})
}

func TestHistoryOperationReleasesSourcesBeforeUniverse(t *testing.T) {
	source := []byte("historical ADR")
	state := &revisionState{lockReady: true, lock: &manifest.Lock{SchemaVersion: 31}, lockFound: true}
	state.loadUniverse = func() (currentstate.Universe, error) {
		return currentstate.Universe{Sources: map[string][]byte{"0001": source}}, nil
	}
	commit := replayCommit{Hash: "merge", Revision: "result", IsMerge: true, Message: "Merge", Parents: []string{"first", "incoming"}}
	op := newHistoryOperationFromCompact([]replayCommit{commit}, nil, 1, func(context.Context, string) (*revisionState, error) {
		return nil, errors.New("planned canonical keys must avoid revision reloads")
	}, nil, func(context.Context) ([]Finding, error) { return nil, nil })
	entry := op.store.addDistinct(commit.Revision, revisionResult{state: state})
	for _, parent := range commit.Parents {
		op.store.addKey(parent, entry)
	}
	op.reserveConsumers([]replayCommit{commit})
	if _, err := op.replayStale(testContext(t), commit); err != nil {
		t.Fatal(err)
	}
	if state.universe.Sources != nil || !state.heavyLive || op.store.currentHeavy != 1 {
		t.Fatalf("stale final use did not release only sources: state=%#v current=%d", state.universe, op.store.currentHeavy)
	}
	if _, err := op.replayTransition(testContext(t), commit); err != nil {
		t.Fatal(err)
	}
	if state.heavyLive || op.store.currentHeavy != 0 || len(state.universe.Sources) != 0 {
		t.Fatalf("transition final use did not release universe: state=%#v current=%d", state.universe, op.store.currentHeavy)
	}
}

func TestHistoryOperationTracksHeterogeneousOctopusFrontier(t *testing.T) {
	commit := replayCommit{Hash: "octopus", Revision: "result", IsMerge: true, Message: "Merge", Parents: []string{"first", "incoming-one", "incoming-two", "incoming-three"}}
	states := map[string]*revisionState{}
	op := newHistoryOperationFromCompact([]replayCommit{commit}, nil, 1, func(_ context.Context, revision string) (*revisionState, error) {
		state := &revisionState{lockReady: true, lock: &manifest.Lock{SchemaVersion: 31}, lockFound: true, configReady: true, config: &config.Config{}}
		state.loadUniverse = func() (currentstate.Universe, error) {
			return currentstate.Universe{Sources: map[string][]byte{revision: []byte(revision)}}, nil
		}
		states[revision] = state
		return state, nil
	}, nil, func(context.Context) ([]Finding, error) { return nil, nil })
	if err := op.planRevisionOwnership(testContext(t), []replayCommit{commit}); err != nil {
		t.Fatal(err)
	}
	op.reserveConsumers([]replayCommit{commit})
	if _, err := op.replayStale(testContext(t), commit); err != nil {
		t.Fatal(err)
	}
	if op.store.currentHeavy != 2 || op.store.highWaterHeavy != 5 {
		t.Fatalf("octopus stale frontier current=%d high-water=%d, want 2 and 5", op.store.currentHeavy, op.store.highWaterHeavy)
	}
	for _, revision := range []string{"result", "first"} {
		state := states[revision]
		if state.universe.Sources != nil || op.store.keys[revision] == nil {
			t.Fatalf("octopus retained stale evidence for %s: state=%#v key=%v", revision, state, op.store.keys[revision] != nil)
		}
	}
	for _, revision := range commit.Parents[1:] {
		state := states[revision]
		if op.store.keys[revision] != nil || state.loadUniverse != nil || state.universeErr != nil || state.config != nil {
			t.Fatalf("octopus incoming %s retained ownership: state=%#v key=%v", revision, state, op.store.keys[revision] != nil)
		}
	}
	if _, err := op.replayTransition(testContext(t), commit); err != nil {
		t.Fatal(err)
	}
	if op.store.currentHeavy != 0 || len(op.store.keys) != 0 || len(op.store.entries) != 0 {
		t.Fatalf("octopus terminal ownership current=%d keys=%d entries=%d", op.store.currentHeavy, len(op.store.keys), len(op.store.entries))
	}
}

func TestStreamingProjectionDetachesRetainedSubjects(t *testing.T) {
	message := "not conventional\n\n" + strings.Repeat("body", 4096)
	subject := message[:len("not conventional")]
	commit := awfgit.Commit{Hash: "deadbeef", Revision: strings.Repeat("d", 40), Subject: subject}
	replay := compactReplayCommit(0, commit)
	retainedFinding := finding(severity.Error, "test", commit, "detail")
	if unsafe.StringData(replay.Subject) == unsafe.StringData(subject) {
		t.Fatal("compact replay subject retains the rich message backing allocation")
	}
	if unsafe.StringData(retainedFinding.Subject) == unsafe.StringData(subject) {
		t.Fatal("ordinary finding subject retains the rich message backing allocation")
	}
}

type cancellationCheckpointContext struct {
	context.Context
	cancelAt int
	checks   int
}

func (c *cancellationCheckpointContext) Err() error {
	c.checks++
	if c.cancelAt > 0 && c.checks >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestReplayGraphObservesCancellationThroughoutConstruction(t *testing.T) {
	commits := []replayCommit{
		{Revision: "child", Parents: []string{"parent", "boundary"}},
		{Revision: "parent"},
	}
	baseline := &cancellationCheckpointContext{Context: testContext(t)}
	if _, err := newReplayGraph(baseline, commits); err != nil {
		t.Fatal(err)
	}
	for cancelAt := 1; cancelAt <= baseline.checks; cancelAt++ {
		ctx := &cancellationCheckpointContext{Context: testContext(t), cancelAt: cancelAt}
		if _, err := newReplayGraph(ctx, commits); !errors.Is(err, context.Canceled) {
			t.Fatalf("graph cancellation checkpoint %d/%d = %v", cancelAt, baseline.checks, err)
		}
	}
}

func TestReplayGraphSchedulesChildrenBeforeParentsDeterministically(t *testing.T) {
	commits := []replayCommit{
		{Revision: "merge", Parents: []string{"left", "right"}, IsMerge: true},
		{Revision: "left", Parents: []string{"root"}},
		{Revision: "right", Parents: []string{"root"}},
		{Revision: "root", Parents: []string{"boundary"}},
		{Revision: "isolated", Parents: []string{"boundary"}},
	}
	graph, err := newReplayGraph(testContext(t), commits)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, commit := range graph.schedule {
		got = append(got, commit.Revision)
	}
	if want := []string{"isolated", "merge", "left", "right", "root"}; !slices.Equal(got, want) {
		t.Fatalf("schedule = %v, want %v", got, want)
	}
	if !graph.boundaries["boundary"] {
		t.Fatalf("boundary parents = %#v", graph.boundaries)
	}
	for _, permuted := range replayPermutations(commits) {
		permutedGraph, err := newReplayGraph(testContext(t), permuted)
		if err != nil {
			t.Fatal(err)
		}
		var permutedSchedule []string
		for _, commit := range permutedGraph.schedule {
			permutedSchedule = append(permutedSchedule, commit.Revision)
		}
		if !slices.Equal(got, permutedSchedule) {
			t.Fatalf("permuted schedule = %v, want %v", permutedSchedule, got)
		}
	}
	octopusParents := []string{"first", "second", "third", "shared-boundary"}
	octopusGraph, err := newReplayGraph(testContext(t), []replayCommit{
		{Revision: "octopus", Parents: octopusParents, IsMerge: true},
		{Revision: "first", Parents: []string{"shared-boundary"}},
		{Revision: "second", Parents: []string{"shared-boundary"}},
		{Revision: "third", Parents: []string{"shared-boundary"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(octopusGraph.schedule[0].Parents, octopusParents) || !octopusGraph.boundaries["shared-boundary"] {
		t.Fatalf("octopus graph lost ordered parent roles or shared boundary: %#v", octopusGraph)
	}
	for _, invalid := range [][]replayCommit{
		{{}},
		{{Revision: "empty-parent", Parents: []string{""}}},
		{{Revision: "same"}, {Revision: "same"}},
		{{Revision: "self", Parents: []string{"self"}}},
		{{Revision: "one", Parents: []string{"two"}}, {Revision: "two", Parents: []string{"one"}}},
	} {
		if _, err := newReplayGraph(testContext(t), invalid); err == nil {
			t.Fatalf("invalid graph accepted: %#v", invalid)
		}
	}
	op := newHistoryOperationFromCompact([]replayCommit{{Revision: "duplicate"}, {Revision: "duplicate"}}, nil, 2,
		func(context.Context, string) (*revisionState, error) {
			t.Fatal("invalid graph loaded a revision")
			return fixedRevisionState(nil, false, currentstate.Universe{}), nil
		}, nil,
		func(context.Context) ([]Finding, error) {
			t.Fatal("invalid graph ran live evaluation")
			return []Finding{}, nil
		})
	if _, err := op.run(testContext(t)); err == nil {
		t.Fatal("run accepted an invalid graph")
	}
}

func replayPermutations(commits []replayCommit) [][]replayCommit {
	working := slices.Clone(commits)
	var permutations [][]replayCommit
	var generate func(int)
	generate = func(index int) {
		if index == len(working) {
			permutations = append(permutations, slices.Clone(working))
			return
		}
		for candidate := index; candidate < len(working); candidate++ {
			working[index], working[candidate] = working[candidate], working[index]
			generate(index + 1)
			working[index], working[candidate] = working[candidate], working[index]
		}
	}
	generate(0)
	return permutations
}

func TestHistoryOperationPreservesStreamFindingOrderAcrossGraphReplay(t *testing.T) {
	ordinary := Finding{Severity: severity.Warn, Rule: "ordinary"}
	liveFinding := Finding{Severity: severity.Error, Rule: "live"}
	t.Run("stale merge findings", func(t *testing.T) {
		commits := []replayCommit{
			{Ordinal: 0, Hash: "stream-first", Revision: "z-result", IsMerge: true, Message: "Merge\n\nAWF-Allow-Version: bad", Parents: []string{"z-first", "z-incoming"}},
			{Ordinal: 1, Hash: "stream-second", Revision: "a-result", IsMerge: true, Message: "Merge\n\nAWF-Allow-Version: bad", Parents: []string{"a-first", "a-incoming"}},
		}
		op := newHistoryOperationFromCompact(commits, []Finding{ordinary}, len(commits), func(context.Context, string) (*revisionState, error) {
			return fixedRevisionState(&manifest.Lock{SchemaVersion: 31}, true, currentstate.Universe{}), nil
		}, nil, func(context.Context) ([]Finding, error) { return []Finding{liveFinding}, nil })
		findings, err := op.run(testContext(t))
		if err != nil {
			t.Fatal(err)
		}
		if got := []string{findings[1].Commit, findings[2].Commit}; !slices.Equal(got, []string{"stream-first", "stream-second"}) {
			t.Fatalf("stale finding commits = %v, want stream order; findings=%#v", got, findings)
		}
		if got := []string{findings[0].Rule, findings[3].Rule}; !slices.Equal(got, []string{"ordinary", "live"}) {
			t.Fatalf("external finding groups = %v; findings=%#v", got, findings)
		}
	})

	t.Run("transition warnings", func(t *testing.T) {
		commits := []replayCommit{
			{Ordinal: 0, Hash: "stream-first", Revision: "z-result"},
			{Ordinal: 1, Hash: "stream-second", Revision: "a-result"},
		}
		op := newHistoryOperationFromCompact(commits, []Finding{ordinary}, len(commits), func(_ context.Context, revision string) (*revisionState, error) {
			return nil, errors.New("cannot load " + revision)
		}, nil, func(context.Context) ([]Finding, error) { return []Finding{liveFinding}, nil })
		findings, err := op.run(testContext(t))
		if err != nil {
			t.Fatal(err)
		}
		if got := []string{findings[2].Commit, findings[3].Commit}; !slices.Equal(got, []string{"stream-first", "stream-second"}) {
			t.Fatalf("transition finding commits = %v, want stream order; findings=%#v", got, findings)
		}
		if got := []string{findings[0].Rule, findings[1].Rule}; !slices.Equal(got, []string{"ordinary", "live"}) {
			t.Fatalf("external finding groups = %v; findings=%#v", got, findings)
		}
	})
}

func TestHistoryOperationConsumesOrderedOctopusParents(t *testing.T) {
	commits := []replayCommit{{
		Hash:     "octopus",
		Revision: "result",
		IsMerge:  true,
		Parents:  []string{"first", "incoming-one", "incoming-two", "incoming-three"},
	}}
	var loads []string
	op := newHistoryOperationFromCompact(commits, nil, 1, func(_ context.Context, revision string) (*revisionState, error) {
		loads = append(loads, revision)
		return fixedRevisionState(&manifest.Lock{SchemaVersion: 31}, true, currentstate.Universe{}), nil
	}, nil, func(context.Context) ([]Finding, error) { return nil, nil })
	if findings, err := op.run(testContext(t)); err != nil || len(findings) != 0 {
		t.Fatalf("octopus replay findings = %#v, error = %v", findings, err)
	}
	if want := []string{"result", "first", "incoming-one", "incoming-two", "incoming-three"}; !slices.Equal(loads, want) {
		t.Fatalf("octopus revision loads = %v, want ordered roles %v", loads, want)
	}
}

func TestHistoryOperationUsesGraphOrderForCoexistingFatalFailures(t *testing.T) {
	graphFirst := errors.New("graph-first failure")
	streamFirst := errors.New("stream-first failure")
	var loads []string
	op := newHistoryOperationFromCompact([]replayCommit{
		{Ordinal: 0, Hash: "stream-first", Revision: "z-result", IsMerge: true},
		{Ordinal: 1, Hash: "graph-first", Revision: "a-result", IsMerge: true},
	}, nil, 2, func(_ context.Context, revision string) (*revisionState, error) {
		loads = append(loads, revision)
		if revision == "a-result" {
			return nil, graphFirst
		}
		return nil, streamFirst
	}, nil, func(context.Context) ([]Finding, error) {
		t.Fatal("fatal replay ran live evaluation")
		return nil, nil
	})
	if _, err := op.run(testContext(t)); !errors.Is(err, graphFirst) || errors.Is(err, streamFirst) {
		t.Fatalf("coexisting fatal error = %v, want graph-first identity", err)
	}
	if !slices.Equal(loads, []string{"a-result", "z-result"}) {
		t.Fatalf("light planning loads = %v, want deterministic graph order", loads)
	}
}

func TestHistoryOperationChecksCancellationBetweenCachedConsumers(t *testing.T) {
	t.Run("between stale and transition", func(t *testing.T) {
		ctx, cancel := context.WithCancel(testContext(t))
		state := &revisionState{
			loadLock: func() (*manifest.Lock, bool, error) {
				cancel()
				return &manifest.Lock{SchemaVersion: 30}, true, nil
			},
			universeReady: true,
		}
		liveCalls := 0
		op := newHistoryOperationFromCompact([]replayCommit{{Hash: "merge", Revision: "merge", IsMerge: true}}, nil, 1,
			func(context.Context, string) (*revisionState, error) { return state, nil }, nil,
			func(context.Context) ([]Finding, error) { liveCalls++; return nil, nil })
		if _, err := op.run(ctx); !errors.Is(err, context.Canceled) || liveCalls != 0 {
			t.Fatalf("between-consumer cancellation = %v, live calls=%d", err, liveCalls)
		}
	})

	t.Run("before live evaluation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(testContext(t))
		state := fixedRevisionState(nil, false, currentstate.Universe{})
		state.loadUniverse = func() (currentstate.Universe, error) {
			cancel()
			return currentstate.Universe{}, nil
		}
		state.universeReady = false
		op := newHistoryOperationFromCompact([]replayCommit{{Hash: "only", Revision: "only"}}, nil, 1,
			func(context.Context, string) (*revisionState, error) { return state, nil }, nil,
			func(context.Context) ([]Finding, error) {
				t.Fatal("canceled replay ran live evaluation")
				return nil, nil
			})
		if _, err := op.run(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("pre-live cancellation = %v", err)
		}
	})

	t.Run("between scheduled commits", func(t *testing.T) {
		ctx, cancel := context.WithCancel(testContext(t))
		loads := map[string]int{}
		heavyLoads := map[string]int{}
		op := newHistoryOperationFromCompact([]replayCommit{
			{Hash: "first", Revision: "a", Ordinal: 1},
			{Hash: "later", Revision: "b", Ordinal: 0},
		}, nil, 2, func(_ context.Context, revision string) (*revisionState, error) {
			loads[revision]++
			state := fixedRevisionState(nil, false, currentstate.Universe{})
			state.loadUniverse = func() (currentstate.Universe, error) {
				heavyLoads[revision]++
				if revision == "a" {
					cancel()
				}
				return currentstate.Universe{}, nil
			}
			state.universeReady = false
			return state, nil
		}, nil, func(context.Context) ([]Finding, error) {
			t.Fatal("canceled replay ran live evaluation")
			return nil, nil
		})
		if _, err := op.run(ctx); !errors.Is(err, context.Canceled) || loads["b"] != 1 || heavyLoads["b"] != 0 {
			t.Fatalf("scheduled cancellation = %v, light=%#v heavy=%#v", err, loads, heavyLoads)
		}
	})
}

func TestStreamingHistoryOperationReportsMalformedMergeAuthorization(t *testing.T) {
	state := fixedRevisionState(&manifest.Lock{SchemaVersion: 31}, true, currentstate.Universe{})
	op := newHistoryOperationFromCompact([]replayCommit{{Hash: "merge", Revision: "merge", IsMerge: true, Message: "Merge\n\nAWF-Allow-Version: bad", Parents: []string{"first", "incoming"}}}, nil, 1, func(context.Context, string) (*revisionState, error) { return state, nil }, nil, func(context.Context) ([]Finding, error) { return nil, nil })
	findings, err := op.staleMergeFindings(testContext(t))
	if err != nil || countRule(findings, "stale-merge-authorization", severity.Error) != 1 {
		t.Fatalf("malformed merge findings = %#v, %v", findings, err)
	}
}

func TestStreamingHistoryOperationProjectsCompactReplayEvidence(t *testing.T) {
	commit := awfgit.Commit{Hash: "short", Revision: "full", Subject: "feat(awf): stream", Message: "full message", Changes: []awfgit.FileChange{{OldPath: "z.md", Path: "a.md", OldText: "large before", NewText: "large after", Added: 9, Deleted: 8}}}
	walks := 0
	op, err := newStreamingHistoryOperation(testContext(t), "base", "head", Inputs{}, func(_ context.Context, base, head string, visit func(awfgit.Commit) error) (int, error) {
		walks++
		if base != "base" || head != "head" {
			t.Fatalf("range = %q..%q", base, head)
		}
		if err := visit(commit); err != nil {
			return 0, err
		}
		return 1, nil
	}, func(context.Context, string) (*revisionState, error) {
		return fixedRevisionState(nil, false, currentstate.Universe{}), nil
	}, nil, func(context.Context) ([]Finding, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if walks != 1 || op.visited != 1 || len(op.commits) != 1 {
		t.Fatalf("walks=%d visited=%d commits=%#v", walks, op.visited, op.commits)
	}
	record := op.commits[0]
	if record.Message != "" || !slices.Equal(record.Paths, []string{"a.md", "z.md"}) || record.Subject != commit.Subject || record.Revision != commit.Revision {
		t.Fatalf("compact record = %#v", record)
	}
}

type testRangeCollector func(context.Context, string, string) ([]awfgit.Commit, error)

func newHistoryOperation(ctx context.Context, base, head string, _ Inputs, collect testRangeCollector, load revisionLoader, _ firstParentPaths, live liveEvaluator) (*historyOperation, error) {
	commits, err := collect(ctx, base, head)
	if err != nil {
		return nil, fmt.Errorf("collect audit range: %w", err)
	}
	return newHistoryOperationWithRelevance(commits, Inputs{}, load, nil, live), nil
}

func newHistoryOperationWithRelevance(commits []awfgit.Commit, _ Inputs, load revisionLoader, paths firstParentPaths, live liveEvaluator) *historyOperation {
	evaluator := newRangeEvaluator(Inputs{})
	compact := make([]replayCommit, 0, len(commits))
	for i, commit := range commits {
		evaluator.observe(commit)
		compact = append(compact, compactReplayCommit(i, commit))
	}
	return newHistoryOperationFromCompact(compact, evaluator.findings(), len(compact), load, paths, live)
}

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
	var requested []string
	load := func(_ context.Context, revision string) (*revisionState, error) {
		loads[revision]++
		requested = append(requested, revision)
		switch revision {
		case "broken-revision":
			return nil, loadErr
		case "child-revision", "outside-revision":
			return fixedRevisionState(nil, false, currentstate.Universe{}), nil
		default:
			return nil, errors.New("unexpected ancestry traversal to " + revision)
		}
	}
	liveCalls := 0
	live := func(context.Context) ([]Finding, error) {
		liveCalls++
		return nil, nil
	}

	op, err := newHistoryOperation(ctx, "base", "head", Inputs{}, collect, load, nil, live)
	if err != nil {
		t.Fatal(err)
	}
	if findings, err := op.transitionFindings(ctx); err != nil || len(findings) != 0 {
		t.Fatalf("boundary transition findings = %#v, %v", findings, err)
	}
	if got := strings.Join(requested, ","); got != "child-revision,outside-revision" {
		t.Fatalf("boundary requests = %s, want direct child and first parent only", got)
	}
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
	second, err := newHistoryOperation(ctx, "base", "head", Inputs{}, collect, load, nil, live)
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

func TestHistoricalAuthoredTransactionsUseObservableOperations(t *testing.T) {
	operation := func(verb adr.OpVerb, id string) adr.Operation { return adr.Operation{Verb: verb, ID: id} }
	status := func(value string) adr.HistoryEvent {
		return adr.HistoryEvent{Kind: adr.HistoryStatus, Date: "2026-08-05", Status: value}
	}
	batch := func(kind adr.HistoryEventKind, operations ...adr.Operation) adr.HistoryEvent {
		return adr.HistoryEvent{Kind: kind, Date: "2026-08-05", Operations: operations}
	}
	record := func(statusValue string, operations []adr.Operation, history ...adr.HistoryEvent) adr.ADR {
		return adr.ADR{Number: "0141", Format: adr.CurrentStateV2, Status: statusValue, Operations: operations, History: history}
	}
	universe := func(records []adr.ADR, claims ...topic.Claim) currentstate.Universe {
		return currentstate.Universe{ADRs: records, Topics: []topic.Topic{{ID: topic.TopicID{Domain: "alpha", Slug: "one"}, Claims: claims}}}
	}
	run := func(t *testing.T, before, after currentstate.Universe) []Finding {
		t.Helper()
		op, err := newHistoryOperation(testContext(t), "base", "head", Inputs{},
			func(context.Context, string, string) ([]awfgit.Commit, error) {
				return []awfgit.Commit{{Hash: "child", Revision: "child", Subject: "feat(invariants): child", Parents: []string{"parent"}}}, nil
			},
			func(_ context.Context, revision string) (*revisionState, error) {
				if revision == "parent" {
					return fixedRevisionState(nil, false, before), nil
				}
				return fixedRevisionState(nil, false, after), nil
			}, nil, func(context.Context) ([]Finding, error) { return nil, nil })
		if err != nil {
			t.Fatal(err)
		}
		findings, err := op.transitionFindings(testContext(t))
		if err != nil {
			t.Fatal(err)
		}
		return findings
	}

	a, b, pending := operation(adr.OpAdd, "alpha/one:a"), operation(adr.OpAdd, "alpha/one:b"), operation(adr.OpAdd, "alpha/one:pending")
	proposed := record("Proposed", []adr.Operation{a, b, pending}, status("Proposed"))
	applied := record("Implementing", []adr.Operation{a, b, pending}, status("Proposed"), status("Implementing"), batch(adr.HistoryApplied, a), batch(adr.HistoryApplied, b))
	clean := run(t, universe([]adr.ADR{proposed}), universe([]adr.ADR{applied},
		topic.Claim{ID: a.ID, Origin: "0141"}, topic.Claim{ID: b.ID, Origin: "0141"}))
	if countRule(clean, currentStateTransitionRule, severity.Error) != 0 {
		t.Fatalf("distinct-target batches and legal multi-event history produced an authored transition finding: %#v", clean)
	}

	x := operation(adr.OpAdd, "alpha/one:x")
	chainBefore := record("Proposed", []adr.Operation{x, pending}, status("Proposed"))
	chainAfter := record("Implementing", []adr.Operation{x, pending}, status("Proposed"), status("Implementing"), batch(adr.HistoryApplied, x), batch(adr.HistoryReapplied, x))
	chained := run(t, universe([]adr.ADR{chainBefore}), universe([]adr.ADR{chainAfter}, topic.Claim{ID: x.ID, Origin: "0141", Prose: "material endpoint"}))
	var duplicate bool
	for _, finding := range chained {
		duplicate = duplicate || strings.Contains(finding.Detail, "target of more than one operation")
	}
	if countRule(chained, currentStateTransitionRule, severity.Error) == 0 || !duplicate {
		t.Fatalf("same-claim authored chain did not produce the expected transition finding: %#v", chained)
	}
}

func TestHistoryOperationRootTransitionUsesEmptyUniverse(t *testing.T) {
	claim := topic.Claim{ID: "alpha/one:new", Slug: "new", Type: topic.Invariant, Prose: "New.", Origin: "0001", Backing: topic.ExplicitNoBacking}
	after := currentstate.Universe{Topics: []topic.Topic{{ID: topic.TopicID{Domain: "alpha", Slug: "one"}, Claims: []topic.Claim{claim}}}}
	loads := 0
	op, err := newHistoryOperation(testContext(t), "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) {
			return []awfgit.Commit{{Hash: "root", Revision: "root", Subject: "feat(awf): root"}}, nil
		},
		func(_ context.Context, revision string) (*revisionState, error) {
			loads++
			if revision != "root" {
				return nil, errors.New("root transition requested a parent")
			}
			return fixedRevisionState(nil, false, after), nil
		},
		nil,
		func(context.Context) ([]Finding, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	findings, err := op.transitionFindings(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if loads != 1 {
		t.Fatalf("root revision loads = %d, want 1", loads)
	}
	var observed bool
	for _, finding := range findings {
		observed = observed || strings.Contains(finding.Detail, "was added with no ADR add operation")
	}
	if !observed {
		t.Fatalf("root transition did not compare against the empty universe: %#v", findings)
	}
}

// invariant: tooling/audit-and-snapshots:audit-cancellation-propagates (TestAuditPropagatesHistoricalCancellation)
func TestAuditPropagatesHistoricalCancellation(t *testing.T) {
	type runCase func(*testing.T, error) ([]Finding, []string, error)
	cases := map[string]runCase{
		"range collection": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			_, err := newHistoryOperation(testContext(t), "base", "head", Inputs{},
				func(context.Context, string, string) ([]awfgit.Commit, error) {
					events = append(events, "termination")
					return nil, termination
				},
				func(context.Context, string) (*revisionState, error) {
					events = append(events, "later revision")
					return fixedRevisionState(nil, false, currentstate.Universe{}), nil
				}, nil,
				func(context.Context) ([]Finding, error) {
					events = append(events, "later live")
					return nil, nil
				})
			return nil, events, err
		},
		"transition result derivation": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			state := &revisionState{
				lockReady: true,
				loadUniverse: func() (currentstate.Universe, error) {
					events = append(events, "termination")
					return currentstate.Universe{}, termination
				},
			}
			op := newHistoryOperationWithRelevance(
				[]awfgit.Commit{{Hash: "first", Revision: "first"}, {Hash: "later", Revision: "later"}}, Inputs{},
				func(_ context.Context, revision string) (*revisionState, error) {
					if revision == "later" {
						events = append(events, "later revision")
					}
					return state, nil
				}, nil,
				func(context.Context) ([]Finding, error) {
					events = append(events, "live before termination")
					return nil, nil
				})
			findings, err := op.run(testContext(t))
			return findings, events, err
		},
		"first-parent derivation": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			op := newHistoryOperationWithRelevance(
				[]awfgit.Commit{{Hash: "child", Revision: "child", Parents: []string{"parent"}, Changes: []awfgit.FileChange{{Path: "code.go"}}}}, Inputs{},
				func(_ context.Context, revision string) (*revisionState, error) {
					if revision == "parent" {
						events = append(events, "termination")
						return nil, termination
					}
					events = append(events, "later child revision")
					return fixedRevisionState(nil, false, currentstate.Universe{}), nil
				}, func(context.Context, string) ([]string, error) { return nil, nil },
				func(context.Context) ([]Finding, error) {
					events = append(events, "live before termination")
					return nil, nil
				})
			findings, err := op.run(testContext(t))
			return findings, events, err
		},
		"first-parent configuration derivation": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			parent := &revisionState{
				lockReady: true,
				loadConfig: func() (*config.Config, error) {
					events = append(events, "termination")
					return nil, termination
				},
			}
			op := newHistoryOperationWithRelevance(
				[]awfgit.Commit{{Hash: "child", Revision: "child", Parents: []string{"parent"}, Changes: []awfgit.FileChange{{Path: "code.go"}}}}, Inputs{},
				func(_ context.Context, revision string) (*revisionState, error) {
					if revision == "parent" {
						return parent, nil
					}
					events = append(events, "later child revision")
					return fixedRevisionState(nil, false, currentstate.Universe{}), nil
				}, func(context.Context, string) ([]string, error) { return nil, nil },
				func(context.Context) ([]Finding, error) {
					events = append(events, "live before termination")
					return nil, nil
				})
			findings, err := op.run(testContext(t))
			return findings, events, err
		},
		"transition first-parent load": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			op := newHistoryOperationWithRelevance(
				[]awfgit.Commit{{Hash: "child", Revision: "child", Parents: []string{"parent"}}}, Inputs{},
				func(_ context.Context, revision string) (*revisionState, error) {
					if revision == "parent" {
						events = append(events, "termination")
						return nil, termination
					}
					return fixedRevisionState(nil, false, currentstate.Universe{}), nil
				}, nil,
				func(context.Context) ([]Finding, error) {
					events = append(events, "live before termination")
					return nil, nil
				})
			findings, err := op.run(testContext(t))
			return findings, events, err
		},
		"transition first-parent current state": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			parent := &revisionState{
				lockReady: true,
				loadUniverse: func() (currentstate.Universe, error) {
					events = append(events, "termination")
					return currentstate.Universe{}, termination
				},
			}
			op := newHistoryOperationWithRelevance(
				[]awfgit.Commit{{Hash: "child", Revision: "child", Parents: []string{"parent"}}}, Inputs{},
				func(_ context.Context, revision string) (*revisionState, error) {
					if revision == "parent" {
						return parent, nil
					}
					return fixedRevisionState(nil, false, currentstate.Universe{}), nil
				}, nil,
				func(context.Context) ([]Finding, error) {
					events = append(events, "live before termination")
					return nil, nil
				})
			findings, err := op.run(testContext(t))
			return findings, events, err
		},
		"merge changed paths": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			parent := fixedRevisionState(nil, false, currentstate.Universe{})
			parent.configReady = true
			parent.config = &config.Config{}
			op := newHistoryOperationWithRelevance(
				[]awfgit.Commit{{Hash: "merge", Revision: "merge", Parents: []string{"parent", "incoming"}, IsMerge: true}}, Inputs{},
				func(_ context.Context, revision string) (*revisionState, error) {
					if revision == "parent" {
						return parent, nil
					}
					events = append(events, "later "+revision+" revision")
					return fixedRevisionState(nil, false, currentstate.Universe{}), nil
				},
				func(context.Context, string) ([]string, error) {
					events = append(events, "termination")
					return nil, termination
				},
				func(context.Context) ([]Finding, error) {
					events = append(events, "later live")
					return nil, nil
				})
			findings, err := op.run(testContext(t))
			return findings, events, err
		},
		"state shared with stale-merge replay": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			op := newHistoryOperationWithRelevance(
				[]awfgit.Commit{{Hash: "merge", Revision: "shared", Parents: []string{"first", "incoming"}, IsMerge: true}}, Inputs{},
				func(_ context.Context, revision string) (*revisionState, error) {
					if revision == "shared" {
						events = append(events, "termination")
						return nil, termination
					}
					events = append(events, "later "+revision+" revision")
					return fixedRevisionState(nil, false, currentstate.Universe{}), nil
				}, nil,
				func(context.Context) ([]Finding, error) {
					events = append(events, "later live")
					return nil, nil
				})
			findings, err := op.run(testContext(t))
			if len(op.store.keys) != 0 || len(op.store.entries) != 0 {
				t.Fatalf("terminated operation retained revision ownership: keys=%d entries=%d", len(op.store.keys), len(op.store.entries))
			}
			return findings, events, err
		},
		"stale-merge-only evidence": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			result := fixedRevisionState(&manifest.Lock{SchemaVersion: 31}, true, currentstate.Universe{})
			incoming := &revisionState{
				lockReady: true,
				loadUniverse: func() (currentstate.Universe, error) {
					events = append(events, "termination")
					return currentstate.Universe{}, termination
				},
			}
			op := newHistoryOperationWithRelevance(
				[]awfgit.Commit{{Hash: "merge", Revision: "result", Parents: []string{"first", "incoming"}, IsMerge: true}}, Inputs{},
				func(_ context.Context, revision string) (*revisionState, error) {
					switch revision {
					case "result":
						return result, nil
					case "first":
						return fixedRevisionState(nil, false, currentstate.Universe{}), nil
					case "incoming":
						return incoming, nil
					default:
						events = append(events, "later revision")
						return nil, errors.New("unexpected revision")
					}
				}, nil,
				func(context.Context) ([]Finding, error) {
					events = append(events, "later live")
					return nil, nil
				})
			findings, err := op.run(testContext(t))
			return findings, events, err
		},
		"live cleanliness": func(t *testing.T, termination error) ([]Finding, []string, error) {
			var events []string
			op := newHistoryOperationWithRelevance(nil, Inputs{},
				func(context.Context, string) (*revisionState, error) {
					events = append(events, "later revision")
					return fixedRevisionState(nil, false, currentstate.Universe{}), nil
				}, nil,
				func(context.Context) ([]Finding, error) {
					events = append(events, "termination")
					return nil, termination
				})
			findings, err := op.run(testContext(t))
			return findings, events, err
		},
	}

	for _, termination := range []error{context.Canceled, context.DeadlineExceeded} {
		termination := termination
		for name, run := range cases {
			t.Run(termination.Error()+"/"+name, func(t *testing.T) {
				findings, events, err := run(t, termination)
				if !errors.Is(err, termination) {
					t.Fatalf("error = %v, want identity %v; findings=%#v events=%v", err, termination, findings, events)
				}
				if countRule(findings, currentStateTransitionRule, severity.Warn) != 0 {
					t.Fatalf("termination became a transition warning: %#v", findings)
				}
				terminationAt := slices.Index(events, "termination")
				if terminationAt < 0 || terminationAt != len(events)-1 {
					t.Fatalf("work continued after termination: %v", events)
				}
			})
		}
	}

	t.Run("production committed evidence", func(t *testing.T) {
		repo, _ := staleAuditRepo(t, 31)
		handle, _, err := awfgit.OpenContaining(repo.Root())
		if err != nil {
			t.Fatal(err)
		}
		for _, termination := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run(termination.Error(), func(t *testing.T) {
				var ctx context.Context
				var cancel context.CancelFunc
				if errors.Is(termination, context.Canceled) {
					ctx, cancel = context.WithCancel(context.Background())
					cancel()
				} else {
					ctx, cancel = context.WithDeadline(context.Background(), time.Time{})
					defer cancel()
				}
				op := newHistoryOperationWithRelevance(
					[]awfgit.Commit{{Hash: "committed", Revision: "HEAD", Subject: "feat(awf): committed evidence"}}, Inputs{},
					func(ctx context.Context, revision string) (*revisionState, error) {
						return loadSelectedRevision(ctx, repo.Root(), revision, handle.CommitEntries, handle.CommitBlobsAt)
					}, nil,
					func(context.Context) ([]Finding, error) { return nil, nil })
				findings, err := op.run(ctx)
				if !errors.Is(err, termination) {
					t.Fatalf("committed evidence error = %v, want %v; findings=%#v", err, termination, findings)
				}
				if countRule(findings, currentStateTransitionRule, severity.Warn) != 0 {
					t.Fatalf("committed evidence termination became a warning: %#v", findings)
				}
			})
		}
	})

	t.Run("selected historical evidence", func(t *testing.T) {
		for _, termination := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run(termination.Error(), func(t *testing.T) {
				var events []string
				entryRead := func(context.Context, string) ([]awfgit.TreeEntry, error) {
					events = append(events, "enumerate")
					return nil, termination
				}
				blobRead := func(context.Context, string, []string) ([]awfgit.IndexBlob, error) {
					events = append(events, "later blob read")
					return nil, nil
				}
				_, err := loadSelectedRevision(testContext(t), t.TempDir(), "revision", entryRead, blobRead)
				if !errors.Is(err, termination) || !slices.Equal(events, []string{"enumerate"}) {
					t.Fatalf("enumeration cancellation = %v; events=%v", err, events)
				}

				events = nil
				_, err = loadSelectedRevision(testContext(t), t.TempDir(), "revision",
					func(context.Context, string) ([]awfgit.TreeEntry, error) {
						events = append(events, "enumerate")
						return []awfgit.TreeEntry{{Path: ".awf/config.yaml", Mode: awfgit.BlobRegular}}, nil
					},
					func(context.Context, string, []string) ([]awfgit.IndexBlob, error) {
						events = append(events, "selected blob read")
						return nil, termination
					})
				if !errors.Is(err, termination) || !slices.Equal(events, []string{"enumerate", "selected blob read"}) {
					t.Fatalf("selected-read cancellation = %v; events=%v", err, events)
				}
			})
		}

		for _, cancelAfterRead := range []int{1, 2} {
			cancelAfterRead := cancelAfterRead
			t.Run("reader returns data after cancellation/read "+strconv.Itoa(cancelAfterRead), func(t *testing.T) {
				ctx, cancel := context.WithCancel(testContext(t))
				defer cancel()
				var events []string
				state, err := loadSelectedRevision(ctx, t.TempDir(), "revision",
					func(context.Context, string) ([]awfgit.TreeEntry, error) {
						events = append(events, "enumerate")
						return []awfgit.TreeEntry{{Path: ".awf/config.yaml", Mode: awfgit.BlobRegular}}, nil
					},
					func(_ context.Context, _ string, _ []string) ([]awfgit.IndexBlob, error) {
						events = append(events, "selected blob read")
						if len(events)-1 == cancelAfterRead {
							cancel()
						}
						return []awfgit.IndexBlob{{Path: ".awf/config.yaml", Mode: awfgit.BlobRegular, Bytes: []byte("prefix: test\nintegrationBranch: main\n")}}, nil
					})
				if err == nil {
					_, err = state.currentState()
				}
				wantEvents := []string{"enumerate", "selected blob read"}
				if cancelAfterRead == 2 {
					wantEvents = append(wantEvents, "selected blob read")
				}
				if !errors.Is(err, context.Canceled) || !slices.Equal(events, wantEvents) {
					t.Fatalf("data-after-cancellation = %v; events=%v, want %v", err, events, wantEvents)
				}
			})
		}
	})

	t.Run("non-context transition failure stays advisory", func(t *testing.T) {
		boom := errors.New("ordinary transition failure")
		op := newHistoryOperationWithRelevance(
			[]awfgit.Commit{{Hash: "ordinary", Revision: "ordinary", Subject: "feat(awf): ordinary failure"}}, Inputs{},
			func(context.Context, string) (*revisionState, error) {
				return &revisionState{lockReady: true, universeReady: true, universeErr: boom}, nil
			}, nil,
			func(context.Context) ([]Finding, error) { return nil, nil })
		findings, err := op.run(testContext(t))
		if err != nil {
			t.Fatalf("ordinary transition failure became fatal: %v", err)
		}
		if countRule(findings, currentStateTransitionRule, severity.Warn) != 1 || !strings.Contains(findings[0].Detail, boom.Error()) {
			t.Fatalf("ordinary transition findings = %#v", findings)
		}
	})
}

func TestLoadSelectedRevisionStopsAfterCanceledEnumeration(t *testing.T) {
	ctx, cancel := context.WithCancel(testContext(t))
	blobReads := 0
	_, err := loadSelectedRevision(ctx, t.TempDir(), "revision", func(context.Context, string) ([]awfgit.TreeEntry, error) {
		cancel()
		return []awfgit.TreeEntry{{Path: ".awf/config.yaml", Mode: awfgit.BlobRegular}}, nil
	}, func(context.Context, string, []string) ([]awfgit.IndexBlob, error) {
		blobReads++
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) || blobReads != 0 {
		t.Fatalf("canceled enumeration error=%v blob reads=%d", err, blobReads)
	}
}

func TestLoadCompleteRevisionPropagatesCommittedTreeFailure(t *testing.T) {
	repo, _ := staleAuditRepo(t, 31)
	handle, _, err := awfgit.OpenContaining(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := loadSelectedRevision(ctx, repo.Root(), "HEAD", handle.CommitEntries, handle.CommitBlobsAt); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled complete revision load = %v", err)
	}
}

func retainedType(candidate, target reflect.Type, seen map[reflect.Type]bool) bool {
	if candidate == target {
		return true
	}
	if seen[candidate] {
		return false
	}
	seen[candidate] = true
	switch candidate.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return retainedType(candidate.Elem(), target, seen)
	case reflect.Map:
		return retainedType(candidate.Key(), target, seen) || retainedType(candidate.Elem(), target, seen)
	case reflect.Struct:
		for i := range candidate.NumField() {
			if retainedType(candidate.Field(i).Type, target, seen) {
				return true
			}
		}
	default:
		return false
	}
	return false
}

// invariant: tooling/audit-and-snapshots:audit-history-operation-owned (TestHistoryOperationStreamsAndReleasesFinalConsumers)
func TestHistoryOperationStreamsAndReleasesFinalConsumers(t *testing.T) {
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
	states["first"].configReady = true
	states["first"].config = &config.Config{}
	heavyLoads := map[string]int{}
	for revision, state := range states {
		revision, universe := revision, state.universe
		state.universe = currentstate.Universe{}
		state.universeReady = false
		state.loadUniverse = func() (currentstate.Universe, error) {
			heavyLoads[revision]++
			return universe, nil
		}
	}
	commits := []awfgit.Commit{
		{Hash: "pure", Revision: "ordinary", Subject: "not conventional"},
		{Hash: "alias", Revision: "irrelevant", Subject: "feat(awf): irrelevant", Parents: []string{"first"}, Changes: []awfgit.FileChange{{Path: "internal/code.go"}}},
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
	liveCalls, walks := 0, 0
	op, err := newStreamingHistoryOperation(ctx, "base", "head", Inputs{},
		func(_ context.Context, base, head string, visit func(awfgit.Commit) error) (int, error) {
			walks++
			if base != "base" || head != "head" {
				t.Fatalf("stream range = %q..%q", base, head)
			}
			for _, commit := range commits {
				if err := visit(commit); err != nil {
					return 0, err
				}
			}
			return len(commits), nil
		},
		load,
		func(context.Context, string) ([]string, error) { return []string{".awf/config.yaml"}, nil },
		func(context.Context) ([]Finding, error) {
			liveCalls++
			if states["result"].universe.Sources != nil {
				t.Fatal("merge result source evidence survived its final stale consumer")
			}
			if states["result"].universe.ADRs != nil || states["result"].universe.Topics != nil {
				t.Fatal("merge result universe survived its final heavy consumer")
			}
			return []Finding{{Severity: severity.Error, Rule: "live-cleanliness"}}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if retainedType(reflect.TypeOf(*op), reflect.TypeOf(awfgit.Commit{}), map[reflect.Type]bool{}) {
		t.Fatal("streaming history operation retains the rich Git commit representation")
	}
	for i, record := range op.commits {
		if unsafe.StringData(record.Subject) == unsafe.StringData(commits[i].Subject) {
			t.Fatalf("compact record %d retains rich subject backing storage", i)
		}
	}
	findings, err := op.run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantFindings := []Finding{
		{Severity: severity.Error, Rule: "conventional-commits", Commit: "pure", Subject: "not conventional", Detail: "subject is not Conventional Commits (type(scope)?: subject)"},
		{Severity: severity.Error, Rule: "stale-merge-authorization", Commit: "merge", Subject: "Merge feature", Detail: "missing authorization version current-state-v1 for ADR-0001"},
		{Severity: severity.Error, Rule: "live-cleanliness"},
		{Severity: severity.Error, Rule: currentStateTransitionRule, Commit: "alias", Subject: "feat(awf): irrelevant", Detail: "claim alpha/one:owned cites pending ADR-legacy which is not in the corpus"},
		{Severity: severity.Error, Rule: currentStateTransitionRule, Commit: "merge", Subject: "Merge feature", Detail: "claim alpha/one:owned was removed with no ADR remove operation in this transition"},
	}
	if !slices.Equal(findings, wantFindings) {
		t.Fatalf("grouped findings = %#v, want %#v", findings, wantFindings)
	}
	for revision := range states {
		if loads[revision] != 1 || heavyLoads[revision] != 1 {
			t.Fatalf("revision %s derivations = light %d heavy %d, want 1 and 1", revision, loads[revision], heavyLoads[revision])
		}
	}
	if loads["irrelevant"] != 0 {
		t.Fatalf("irrelevant alias loaded a distinct state: %#v", loads)
	}
	if walks != 1 || op.visited != len(commits) {
		t.Fatalf("stream ownership walks=%d visited=%d, want 1 and %d", walks, op.visited, len(commits))
	}
	if op.store.currentHeavy != 0 || op.store.highWaterHeavy != 3 || len(op.store.keys) != 0 || len(op.store.entries) != 0 {
		t.Fatalf("terminal heavy ownership current=%d high-water=%d keys=%d entries=%d", op.store.currentHeavy, op.store.highWaterHeavy, len(op.store.keys), len(op.store.entries))
	}
	if liveCalls != 1 {
		t.Fatalf("live calls = %d, want 1", liveCalls)
	}

	cachedBoom := errors.New("cached load failure")
	errorLoads := 0
	errorCommit := replayCommit{Hash: "cached-error", Revision: "cached-error", Subject: "feat(awf): cached error"}
	newErrorOperation := func() *historyOperation {
		return newHistoryOperationFromCompact([]replayCommit{errorCommit}, nil, 1, func(context.Context, string) (*revisionState, error) {
			errorLoads++
			return nil, cachedBoom
		}, nil, func(context.Context) ([]Finding, error) { return nil, nil })
	}
	errorOp := newErrorOperation()
	graph, err := newReplayGraph(ctx, errorOp.commits)
	if err != nil {
		t.Fatal(err)
	}
	if err := errorOp.planRevisionOwnership(ctx, graph.schedule); err != nil {
		t.Fatal(err)
	}
	errorOp.reserveConsumers(graph.schedule)
	for attempt := range 2 {
		if _, err := errorOp.state(ctx, errorCommit.Revision); !errors.Is(err, cachedBoom) {
			t.Fatalf("cached error attempt %d identity = %v, want exact %v", attempt+1, err, cachedBoom)
		}
	}
	errorFindings, err := errorOp.replayTransition(ctx, errorCommit)
	if err != nil {
		t.Fatal(err)
	}
	wantErrorFindings := []Finding{{
		Severity: severity.Warn,
		Rule:     currentStateTransitionRule,
		Commit:   errorCommit.Hash,
		Subject:  errorCommit.Subject,
		Detail:   "could not load the current-state universes for this commit: " + cachedBoom.Error(),
	}}
	if !slices.Equal(errorFindings, wantErrorFindings) || errorLoads != 1 || len(errorOp.store.keys) != 0 || len(errorOp.store.entries) != 0 {
		t.Fatalf("cached error outcome findings=%#v loads=%d keys=%d entries=%d", errorFindings, errorLoads, len(errorOp.store.keys), len(errorOp.store.entries))
	}
	freshOp := newErrorOperation()
	freshFindings, err := freshOp.run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(freshFindings, wantErrorFindings) || errorLoads != 2 || len(freshOp.store.keys) != 0 || len(freshOp.store.entries) != 0 || &freshOp.store == &errorOp.store {
		t.Fatalf("separate invocation reused cached operation state: findings=%#v loads=%d", freshFindings, errorLoads)
	}
}

func TestHistoryOperationErrorPaths(t *testing.T) {
	ctx := testContext(t)
	boom := errors.New("boom")
	emptyLive := func(context.Context) ([]Finding, error) { return nil, nil }
	if _, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) { return nil, boom }, nil, nil, emptyLive); !errors.Is(err, boom) {
		t.Fatalf("collection error = %v", err)
	}

	nilOp, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) { return nil, nil },
		func(context.Context, string) (*revisionState, error) {
			//nolint:nilnil // this deliberately malformed dependency exercises the operation's fail-closed guard
			return nil, nil
		}, nil, emptyLive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nilOp.state(ctx, "nil"); err == nil || !strings.Contains(err.Error(), "no state") {
		t.Fatalf("nil revision state error = %v", err)
	}

	parentLoads := map[string]int{}
	parentErrorOp := newHistoryOperationWithRelevance([]awfgit.Commit{{Revision: "child", Parents: []string{"parent"}, Changes: []awfgit.FileChange{{Path: "code.go"}}}}, Inputs{},
		func(_ context.Context, revision string) (*revisionState, error) {
			parentLoads[revision]++
			if revision == "parent" {
				return nil, boom
			}
			return fixedRevisionState(nil, false, currentstate.Universe{}), nil
		}, func(context.Context, string) ([]string, error) { return nil, nil }, emptyLive)
	if _, err := parentErrorOp.stateForCommit(ctx, parentErrorOp.commits[0]); err != nil || parentLoads["parent"] != 1 || parentLoads["child"] != 1 {
		t.Fatalf("ambiguous parent error did not reload child: loads=%#v err=%v", parentLoads, err)
	}

	liveOp, err := newHistoryOperation(ctx, "base", "head", Inputs{},
		func(context.Context, string, string) ([]awfgit.Commit, error) { return nil, nil },
		func(context.Context, string) (*revisionState, error) { return nil, boom },
		nil,
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
		func(context.Context, string) (*revisionState, error) { return nil, boom }, nil, emptyLive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := staleOp.run(ctx); !errors.Is(err, boom) {
		t.Fatalf("stale error = %v", err)
	}

	malformedLock := &revisionState{
		loadLock: func() (*manifest.Lock, bool, error) {
			return nil, true, errors.New("malformed lock")
		},
	}
	malformedLock.loadUniverse = func() (currentstate.Universe, error) {
		_, _, err := malformedLock.lockEvidence()
		return currentstate.Universe{}, err
	}
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
				func(context.Context, string, string) ([]awfgit.Commit, error) { return []awfgit.Commit{tc.commit}, nil }, tc.load, nil, emptyLive)
			if err != nil {
				t.Fatal(err)
			}
			findings, err := op.transitionFindings(ctx)
			if err != nil || len(findings) != 1 || findings[0].Severity != severity.Warn || !strings.Contains(findings[0].Detail, boom.Error()) {
				t.Fatalf("transition findings = %#v, %v", findings, err)
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
		}, nil, emptyLive)
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

// invariant: tooling/audit-and-snapshots:audit-history-policy-projection (TestHistoricalStateUsesPolicyProjectionAndReusesIrrelevantCommits)
func TestHistoricalStateUsesPolicyProjectionAndReusesIrrelevantCommits(t *testing.T) {
	ctx := testContext(t)
	t.Run("production reduced loader preserves ordinary findings", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		base := gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{
			".awf/config.yaml": "prefix: test\nintegrationBranch: master\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"internal/**/*_test.go\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n",
		})
		gitfixture.Commit(t, repo, "not conventional", map[string]string{"internal/code.go": "package internal\n"})
		gitfixture.Commit(t, repo, "feat(awf): malformed marker", map[string]string{
			"internal/proof_test.go": "package internal\n// invariant: alpha/one:missing (TestMissing)\nfunc TestMissing() {}\n",
		})
		gitfixture.Commit(t, repo, "feat(awf): malformed sidecar", map[string]string{".awf/domains/alpha.yaml": "unknown: [\n"})

		findings, _, err := Run(ctx, repo.Root(), base, "HEAD", Inputs{})
		if err != nil {
			t.Fatal(err)
		}
		if countRule(findings, "conventional-commits", severity.Error) != 1 {
			t.Fatalf("ordinary commit findings changed: %#v", findings)
		}
		if countRule(findings, currentStateTransitionRule, severity.Warn) != 0 {
			t.Fatalf("marker/domain-only historical bytes produced transition warnings: %#v", findings)
		}
	})
	t.Run("merge relevance stays outside ordinary rules", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		base := gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{
			".awf/config.yaml": "prefix: test\nintegrationBranch: master\n",
		})
		main := gitfixture.Merge(t, repo, "feat(awf): main")
		gitfixture.CheckoutNewBranch(t, repo, "feature", base)
		feature := gitfixture.Commit(t, repo, "feat(awf): feature", map[string]string{
			".awf/config.yaml": "prefix: [\n",
			"go.mod":           "module example.com/merge\n",
			"docs/merge.md":    "historical" + string(rune(0x2014)) + "punctuation\n",
			"large.go":         "package large\n" + strings.Repeat("var Value = 1\n", 5),
		})
		merge := gitfixture.Merge(t, repo, "Merge feature", main, feature)
		findings, _, err := Run(ctx, repo.Root(), feature, merge, Inputs{
			Settings: Settings{},
			ADRDir:   "docs/decisions", DocsDir: "docs", PlansDir: "docs/plans",
		})
		if err != nil {
			t.Fatal(err)
		}
		if countRule(findings, currentStateTransitionRule, severity.Warn) != 1 {
			t.Fatalf("merge authority change did not force a historical reload: %#v", findings)
		}
		for _, rule := range []string{"dependency-adr", "plan-for-large-change", "plain-punctuation"} {
			if countRule(findings, rule, severity.Warn) != 0 {
				t.Fatalf("merge relevance leaked into ordinary rule %s: %#v", rule, findings)
			}
		}
	})
	t.Run("stale replay ignores omitted projections", func(t *testing.T) {
		repo, base := staleAuditRepo(t, 31)
		main := gitfixture.Merge(t, repo, "feat(awf): main")
		gitfixture.CheckoutNewBranch(t, repo, "feature", base)
		feature := gitfixture.Commit(t, repo, "feat(awf): feature", map[string]string{
			".awf/config.yaml":        "prefix: test\nintegrationBranch: master\ntargets: [claude]\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"internal/**/*_test.go\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n",
			".awf/domains/alpha.yaml": "unknown: [\n",
			"internal/proof_test.go":  "package internal\n// invariant: alpha/one:missing (TestMissing)\nfunc TestMissing() {}\n",
		})
		merge := gitfixture.Merge(t, repo, "Merge feature", main, feature)
		commit := awfgit.Commit{Hash: merge[:8], Revision: merge, Subject: "Merge feature", Message: "Merge feature", Parents: []string{main, feature}, IsMerge: true}
		handle, _, err := awfgit.OpenContaining(repo.Root())
		if err != nil {
			t.Fatal(err)
		}
		if err := staleMergeFindingsForTest(t, repo.Root(), handle, []Commit{commit}); err != nil {
			t.Fatalf("stale replay rejected omitted marker/domain projections: %v", err)
		}
	})
	outside := awfgit.Commit{Revision: "outside"}
	commits := []awfgit.Commit{
		{Revision: "root"},
		{Revision: "code", Parents: []string{outside.Revision}, Changes: []awfgit.FileChange{{Path: "internal/code.go", Action: awfgit.Modified}}},
		{Revision: "marker", Parents: []string{"code"}, Changes: []awfgit.FileChange{{Path: "internal/proof_test.go", Action: awfgit.Modified}}},
		{Revision: "sidecar", Parents: []string{"marker"}, Changes: []awfgit.FileChange{{Path: ".awf/domains/alpha.yaml", Action: awfgit.Modified}}},
		{Revision: "config", Parents: []string{"sidecar"}, Changes: []awfgit.FileChange{{Path: ".awf/config.yaml", Action: awfgit.Modified}}},
		{Revision: "topic", Parents: []string{"config"}, Changes: []awfgit.FileChange{{Path: ".awf/topics/metadata/alpha/one.yaml", Action: awfgit.Modified}, {Path: ".awf/topics/parts/alpha/one/current-state.md", Action: awfgit.Modified}}},
		{Revision: "default-adr", Parents: []string{"topic"}, Changes: []awfgit.FileChange{{Path: "docs/decisions/0001-one.md", Action: awfgit.Modified}}},
		{Revision: "custom-config", Parents: []string{"default-adr"}, Changes: []awfgit.FileChange{{Path: ".awf/config.yaml", Action: awfgit.Modified}}},
		{Revision: "custom-adr", Parents: []string{"custom-config"}, Changes: []awfgit.FileChange{{Path: "records/decisions/0002-two.md", Action: awfgit.Modified}}},
		{Revision: "delete", Parents: []string{"custom-adr"}, Changes: []awfgit.FileChange{{Path: "records/decisions/0002-two.md", Action: awfgit.Deleted}}},
		{Revision: "rename", Parents: []string{"delete"}, Changes: []awfgit.FileChange{{Path: "records/decisions/0002-two.md", Action: awfgit.Deleted}, {Path: "records/decisions/0003-three.md", Action: awfgit.Added}}},
		{Revision: "merge-irrelevant", Parents: []string{"rename", "incoming-zero"}, IsMerge: true},
		{Revision: "merge", Parents: []string{"merge-irrelevant", "incoming"}, IsMerge: true},
		{Revision: "merge-ambiguous", Parents: []string{"merge", "incoming-two"}, IsMerge: true},
	}
	loads := map[string]int{}
	load := func(_ context.Context, revision string) (*revisionState, error) {
		loads[revision]++
		state := fixedRevisionState(nil, false, currentstate.Universe{})
		state.configReady = true
		state.config = &config.Config{}
		return state, nil
	}
	firstParentPaths := func(_ context.Context, revision string) ([]string, error) {
		switch revision {
		case "merge-irrelevant":
			return []string{"internal/merge.go"}, nil
		case "merge":
			return []string{"records/decisions/0004-merge.md"}, nil
		case "merge-ambiguous":
			return nil, errors.New("first-parent evidence unavailable")
		default:
			t.Fatalf("first-parent paths requested for %q", revision)
			return nil, nil
		}
	}
	op := newHistoryOperationWithRelevance(commits, Inputs{}, load, firstParentPaths, func(context.Context) ([]Finding, error) { return nil, nil })
	for _, commit := range op.commits {
		if _, err := op.stateForCommit(ctx, commit); err != nil {
			t.Fatalf("state for %s: %v", commit.Revision, err)
		}
	}
	if loads["root"] != 1 || loads["outside"] != 1 || loads["code"] != 0 || loads["marker"] != 0 || loads["merge-irrelevant"] != 0 {
		t.Fatalf("irrelevant code or marker changes reloaded state: %#v", loads)
	}
	for _, revision := range []string{"sidecar", "config", "topic", "default-adr", "custom-config", "merge-ambiguous"} {
		if loads[revision] != 1 {
			t.Errorf("loads[%s] = %d, want 1", revision, loads[revision])
		}
	}
	for _, revision := range []string{"custom-adr", "delete", "rename", "merge"} {
		if loads[revision] != 0 {
			t.Errorf("loads[%s] = %d, want 0: fixed docs/ remains the historical authority", revision, loads[revision])
		}
	}
	if loads["incoming-zero"] != 0 || loads["incoming"] != 0 || loads["incoming-two"] != 0 {
		t.Errorf("incoming merge parent was loaded during first-parent relevance: %#v", loads)
	}

	canonicalLoads := map[string]int{}
	parent := fixedRevisionState(nil, false, currentstate.Universe{})
	parent.configReady = true
	parent.config = &config.Config{}
	canonical := newHistoryOperationWithRelevance(
		[]awfgit.Commit{{Revision: "canonical-child", Parents: []string{"canonical-parent"}, Changes: []awfgit.FileChange{{Path: "docs/decisions/0001-one.md"}}}},
		Inputs{},
		func(_ context.Context, revision string) (*revisionState, error) {
			canonicalLoads[revision]++
			if revision == "canonical-parent" {
				return parent, nil
			}
			return fixedRevisionState(nil, false, currentstate.Universe{}), nil
		}, func(context.Context, string) ([]string, error) { return nil, nil }, func(context.Context) ([]Finding, error) { return nil, nil })
	if _, err := canonical.stateForCommit(ctx, canonical.commits[0]); err != nil || canonicalLoads["canonical-child"] != 1 {
		t.Fatalf("fixed docs/ change reused stale state: loads=%#v err=%v", canonicalLoads, err)
	}
}

// invariant: tooling/audit-and-snapshots:sparse-snapshot-explicit-selection (TestHistoricalStateSelectsOnlyAuthorityBlobs)
// TestHistoricalStateSelectsOnlyAuthorityBlobs pins the two-stage committed
// authority projection without a Git fixture: its dependencies record exactly
// which metadata and object reads the historical loader requests.
func TestHistoricalStateSelectsOnlyAuthorityBlobs(t *testing.T) {
	const configPath = ".awf/config.yaml"
	const lockPath = ".awf/awf.lock"
	const lock = `{"awfVersion":"v0.18.0","schemaVersion":31,"files":{}}`
	base := []awfgit.TreeEntry{
		{Path: configPath, Mode: awfgit.BlobRegular},
		{Path: lockPath, Mode: awfgit.BlobRegular},
		{Path: ".awf/topics/metadata/alpha/one.yaml", Mode: awfgit.BlobRegular},
		{Path: ".awf/topics/parts/alpha/one/current-state.md", Mode: awfgit.BlobRegular},
		{Path: "docs/decisions/0001-one.md", Mode: awfgit.BlobRegular},
		{Path: "internal/bad_marker_test.go", Mode: awfgit.BlobRegular},
		{Path: ".awf/domains/alpha.yaml", Mode: awfgit.BlobRegular},
		{Path: "nested/.awf/config.yaml", Mode: awfgit.BlobRegular},
		{Path: "nested/docs/decisions/0002-nested.md", Mode: awfgit.BlobRegular},
	}
	bodies := map[string][]byte{
		configPath:                            []byte("prefix: test\nintegrationBranch: main\ntargets: [claude]\ndomains: [alpha]\n"),
		lockPath:                              []byte(lock),
		".awf/topics/metadata/alpha/one.yaml": []byte("title: One\nsummary: O.\npaths: [\"internal/**\"]\n"),
		".awf/topics/parts/alpha/one/current-state.md": []byte(historicalTopicPart("0001")),
		"docs/decisions/0001-one.md":                   []byte(historicalLegacyADR()),
		"internal/bad_marker_test.go":                  []byte("package internal\n// invariant: broken\n"),
	}

	for _, tc := range []struct {
		name       string
		entries    []awfgit.TreeEntry
		configBody string
		lockBody   string
		wantReads  [][]string
		wantErr    bool
	}{
		{"default", base, string(bodies[configPath]), lock, [][]string{{configPath, lockPath}, {".awf/topics/metadata/alpha/one.yaml", ".awf/topics/parts/alpha/one/current-state.md", "docs/decisions/0001-one.md"}}, false},
		{"custom docs config still uses docs authority", base, "prefix: test\nintegrationBranch: main\ntargets: [claude]\ndomains: [alpha]\ndocsDir: records\n", lock, [][]string{{configPath, lockPath}, {".awf/topics/metadata/alpha/one.yaml", ".awf/topics/parts/alpha/one/current-state.md", "docs/decisions/0001-one.md"}}, false},
		{"absent config", base[1:], "", lock, nil, false},
		{"absent lock", append([]awfgit.TreeEntry{base[0]}, base[2:]...), string(bodies[configPath]), "", [][]string{{configPath}, {".awf/topics/metadata/alpha/one.yaml", ".awf/topics/parts/alpha/one/current-state.md", "docs/decisions/0001-one.md"}}, false},
		{"symlink authority", append([]awfgit.TreeEntry{{Path: configPath, Mode: awfgit.BlobSymlink}}, base[1:]...), string(bodies[configPath]), lock, [][]string{{configPath, lockPath}}, true},
		{"historical schema", base, "prefix: test\ndomains: [alpha]\nskills: []\nworkflowTelemetry:\n  retention:\n    maxCompletedEffortAgeDays: 1\n", `{"awfVersion":"v0.18.0","schemaVersion":19,"files":{}}`, [][]string{{configPath, lockPath}, {".awf/topics/metadata/alpha/one.yaml", ".awf/topics/parts/alpha/one/current-state.md", "docs/decisions/0001-one.md"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reads [][]string
			read := func(_ context.Context, _ string, paths []string) ([]awfgit.IndexBlob, error) {
				reads = append(reads, slices.Clone(paths))
				blobs := make([]awfgit.IndexBlob, 0, len(paths))
				for _, p := range paths {
					if p == configPath && tc.configBody != "" {
						blobs = append(blobs, awfgit.IndexBlob{Path: p, Mode: awfgit.BlobRegular, Bytes: []byte(tc.configBody)})
						continue
					}
					if p == lockPath && tc.lockBody != "" {
						blobs = append(blobs, awfgit.IndexBlob{Path: p, Mode: awfgit.BlobRegular, Bytes: []byte(tc.lockBody)})
						continue
					}
					if p == "records/decisions/0001-one.md" {
						blobs = append(blobs, awfgit.IndexBlob{Path: p, Mode: awfgit.BlobRegular, Bytes: []byte(historicalLegacyADR())})
						continue
					}
					blobs = append(blobs, awfgit.IndexBlob{Path: p, Mode: awfgit.BlobRegular, Bytes: slices.Clone(bodies[p])})
				}
				return blobs, nil
			}
			state, err := loadSelectedRevision(testContext(t), t.TempDir(), "revision", func(context.Context, string) ([]awfgit.TreeEntry, error) {
				return slices.Clone(tc.entries), nil
			}, read)
			if err != nil {
				t.Fatalf("loadSelectedRevision: %v", err)
			}
			if state == nil {
				t.Fatal("loader returned no state")
			}
			_, stateErr := state.currentState()
			if tc.wantErr {
				if stateErr == nil {
					t.Fatal("symlink authority was accepted")
				}
				lock, found, lockErr := state.lockEvidence()
				if lockErr != nil || !found || lock == nil || lock.SchemaVersion != 31 {
					t.Fatalf("symlink config lost safe lock evidence: lock=%#v found=%v err=%v", lock, found, lockErr)
				}
				if !slices.EqualFunc(reads, tc.wantReads, slices.Equal[[]string]) {
					t.Fatalf("selected reads before symlink error = %#v, want %#v", reads, tc.wantReads)
				}
				return
			}
			if stateErr != nil {
				t.Fatalf("derived current state: %v", stateErr)
			}
			if !slices.EqualFunc(reads, tc.wantReads, slices.Equal[[]string]) {
				t.Fatalf("selected reads = %#v, want %#v", reads, tc.wantReads)
			}
			if tc.name == "absent config" && len(reads) != 0 {
				t.Fatalf("absent config read blobs: %#v", reads)
			}
		})
	}

	entries, reads := 0, 0
	loader := func(ctx context.Context, revision string) (*revisionState, error) {
		return loadSelectedRevision(ctx, t.TempDir(), revision, func(context.Context, string) ([]awfgit.TreeEntry, error) {
			entries++
			return slices.Clone(base), nil
		}, func(_ context.Context, _ string, paths []string) ([]awfgit.IndexBlob, error) {
			reads += len(paths)
			out := make([]awfgit.IndexBlob, 0, len(paths))
			for _, p := range paths {
				out = append(out, awfgit.IndexBlob{Path: p, Mode: awfgit.BlobRegular, Bytes: slices.Clone(bodies[p])})
			}
			return out, nil
		})
	}
	op := newHistoryOperationWithRelevance([]awfgit.Commit{{Revision: "parent"}, {Revision: "irrelevant", Parents: []string{"parent"}, Changes: []awfgit.FileChange{{Path: "internal/code.go"}}}}, Inputs{}, loader, func(context.Context, string) ([]string, error) { return nil, nil }, func(context.Context) ([]Finding, error) { return nil, nil })
	if _, err := op.stateForCommit(testContext(t), op.commits[1]); err != nil {
		t.Fatal(err)
	}
	if entries != 1 || reads != 2 {
		t.Fatalf("irrelevant commit did not reuse light controls: entry reads=%d blob reads=%d, want 1 and 2", entries, reads)
	}
}

func historicalLegacyADR() string {
	return "---\nstatus: Implemented\ndate: 2026-07-20\n---\n# Historical decision\n"
}

func historicalTopicPart(origin string) string {
	return "Historical topic authority.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-" + origin + "\n"
}

// TestLoadSelectedRevisionRejectsIncompleteOrUnscannableEvidence exercises
// sparse-loader failures before they can become a partial policy universe.
func TestLoadSelectedRevisionRejectsIncompleteOrUnscannableEvidence(t *testing.T) {
	const configPath = ".awf/config.yaml"
	entries := []awfgit.TreeEntry{
		{Path: configPath, Mode: awfgit.BlobRegular},
		{Path: "docs/decisions/0001-one.md", Mode: awfgit.BlobRegular},
	}
	configBlob := awfgit.IndexBlob{Path: configPath, Mode: awfgit.BlobRegular, Bytes: []byte("prefix: test\nintegrationBranch: main\ntargets: [claude]\ndomains: []\n")}
	load := func(blobs ...[]awfgit.IndexBlob) func(context.Context, string, []string) ([]awfgit.IndexBlob, error) {
		calls := 0
		return func(context.Context, string, []string) ([]awfgit.IndexBlob, error) {
			got := blobs[calls]
			calls++
			return got, nil
		}
	}
	entryRead := func(ctx context.Context, _ string) ([]awfgit.TreeEntry, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return entries, nil
	}

	t.Run("invalid controls selection", func(t *testing.T) {
		_, err := loadSelectedRevision(testContext(t), t.TempDir(), "revision", entryRead,
			load([]awfgit.IndexBlob{{Path: configPath, Mode: awfgit.BlobMode(99)}}))
		if err == nil {
			t.Fatal("invalid selected controls accepted")
		}
	})
	t.Run("authority read failure", func(t *testing.T) {
		boom := errors.New("authority object missing")
		calls := 0
		state, err := loadSelectedRevision(testContext(t), t.TempDir(), "revision", entryRead,
			func(context.Context, string, []string) ([]awfgit.IndexBlob, error) {
				calls++
				if calls == 1 {
					return []awfgit.IndexBlob{configBlob}, nil
				}
				return nil, boom
			})
		if err == nil {
			_, err = state.currentState()
		}
		if !errors.Is(err, boom) {
			t.Fatalf("authority read error = %v, want %v", err, boom)
		}
	})
	for name, authority := range map[string][]awfgit.IndexBlob{
		"invalid authority selection": {{Path: "docs/decisions/0001-one.md", Mode: awfgit.BlobMode(99)}},
		"symlink authority":           {{Path: "docs/decisions/0001-one.md", Mode: awfgit.BlobSymlink, Bytes: []byte("target")}},
		"duplicate final selection":   {configBlob},
	} {
		t.Run(name, func(t *testing.T) {
			state, err := loadSelectedRevision(testContext(t), t.TempDir(), "revision", entryRead,
				load([]awfgit.IndexBlob{configBlob}, authority))
			if err == nil {
				_, err = state.currentState()
			}
			if err == nil {
				t.Fatal("invalid sparse authority accepted")
			}
		})
	}

	configErr := errors.New("config is a symlink")
	state := &revisionState{configReady: true, configErr: configErr, universeReady: true, universeErr: configErr}
	if _, err := state.currentState(); err == nil {
		t.Fatal("configuration error did not prevent current-state loading")
	}
	controls, err := snapshot.NewSelection([]snapshot.File{
		{Path: configPath, Mode: snapshot.Regular, Bytes: configBlob.Bytes},
		{Path: ".awf/awf.lock", Mode: snapshot.Regular, Bytes: []byte("not a lock")},
	})
	if err != nil {
		t.Fatal(err)
	}
	lock, found, lockErr := auditLockFromSelection(controls)
	if lockErr == nil {
		t.Fatal("malformed lock was accepted")
	}
	if _, err := revisionStateFromControlOutcome(lock, found, lockErr, lockErr).currentState(); err == nil {
		t.Fatal("malformed lock did not prevent current-state loading")
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
		nil,
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
