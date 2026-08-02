package audit

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

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
			parent.config = &config.Config{DocsDir: "docs"}
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
			if _, cachedErr := op.state(testContext(t), "shared"); !errors.Is(cachedErr, termination) {
				t.Fatalf("cached termination = %v", cachedErr)
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
						return loadCompleteRevision(ctx, repo.Root(), handle, revision)
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

func TestLoadCompleteRevisionPropagatesCommittedTreeFailure(t *testing.T) {
	repo, _ := staleAuditRepo(t, 31)
	handle, _, err := awfgit.OpenContaining(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := loadCompleteRevision(ctx, repo.Root(), handle, "HEAD"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled complete revision load = %v", err)
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
		nil,
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
			Settings: Settings{
				AllowedTypes:        []string{"feat"},
				DependencyManifests: []string{"go.mod"},
				DiffThreshold:       1,
				PlainPunctuation:    true,
			},
			ADRDir: "docs/decisions", DocsDir: "docs", PlansDir: "docs/plans",
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
		state.config = &config.Config{DocsDir: "docs"}
		switch revision {
		case "custom-config", "custom-adr", "delete", "rename", "merge":
			state.config.DocsDir = "records"
		}
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
	for _, commit := range commits {
		if _, err := op.stateForCommit(ctx, commit); err != nil {
			t.Fatalf("state for %s: %v", commit.Revision, err)
		}
	}
	if loads["root"] != 1 || loads["outside"] != 1 || loads["code"] != 0 || loads["marker"] != 0 || loads["merge-irrelevant"] != 0 {
		t.Fatalf("irrelevant code or marker changes reloaded state: %#v", loads)
	}
	for _, revision := range []string{"sidecar", "config", "topic", "default-adr", "custom-config", "custom-adr", "delete", "rename", "merge", "merge-ambiguous"} {
		if loads[revision] != 1 {
			t.Errorf("loads[%s] = %d, want 1", revision, loads[revision])
		}
	}
	if loads["incoming-zero"] != 0 || loads["incoming"] != 0 || loads["incoming-two"] != 0 {
		t.Errorf("incoming merge parent was loaded during first-parent relevance: %#v", loads)
	}

	canonicalLoads := map[string]int{}
	parent := fixedRevisionState(nil, false, currentstate.Universe{})
	parent.configReady = true
	parent.config = &config.Config{DocsDir: "./docs/"}
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
		t.Fatalf("non-canonical docsDir reused stale state: loads=%#v err=%v", canonicalLoads, err)
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
