package audit

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestReplayGraphSchedulesChildrenBeforeParents(t *testing.T) {
	graph, err := newReplayGraph(context.Background(), []replayCommit{
		{Revision: "child", Parents: []string{"parent"}},
		{Revision: "parent", Parents: []string{"root"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{graph.schedule[0].Revision, graph.schedule[1].Revision}
	if want := []string{"child", "parent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("schedule = %v, want %v", got, want)
	}
	if !graph.boundaries["root"] {
		t.Fatalf("boundaries = %v, want root", graph.boundaries)
	}
}

func TestReplayGraphRejectsInvalidHistory(t *testing.T) {
	for _, commits := range [][]replayCommit{
		{{Revision: ""}},
		{{Revision: "same"}, {Revision: "same"}},
		{{Revision: "self", Parents: []string{"self"}}},
		{{Revision: "a", Parents: []string{"b"}}, {Revision: "b", Parents: []string{"a"}}},
	} {
		if _, err := newReplayGraph(context.Background(), commits); err == nil {
			t.Fatalf("newReplayGraph(%#v) succeeded", commits)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newReplayGraph(ctx, []replayCommit{{Revision: "a"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled graph error = %v", err)
	}
}
