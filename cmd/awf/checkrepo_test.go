package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

type repoCheckCounters struct {
	loads, opens, reports, states, indexes int
}

func repoCheckTestDependencies(t *testing.T, cfg *config.Config, p *project.Project, check project.CheckReport, state project.CurrentStateReport, tree *snapshot.Tree, counts *repoCheckCounters) repoCheckDependencies {
	t.Helper()
	return repoCheckDependencies{
		loadConfig: func(string) (*config.Config, error) {
			counts.loads++
			return cfg, nil
		},
		openProject: func(_ context.Context, _ string, got *config.Config) (*project.Project, error) {
			counts.opens++
			if got != cfg {
				t.Fatalf("openProject config = %p, want prepared config %p", got, cfg)
			}
			return p, nil
		},
		checkReport: func(_ context.Context, got *project.Project) (project.CheckReport, error) {
			counts.reports++
			if got != p {
				t.Fatalf("checkReport project = %p, want prepared project %p", got, p)
			}
			return check, nil
		},
		currentState: func(_ context.Context, got *project.Project) (project.CurrentStateReport, error) {
			counts.states++
			if got != p {
				t.Fatalf("currentState project = %p, want prepared project %p", got, p)
			}
			return state, nil
		},
		indexTree: func(context.Context, string) (*snapshot.Tree, error) {
			counts.indexes++
			return tree, nil
		},
	}
}

// invariant: tooling/cli:repo-check-capability-plan (TestRepoCheckCapabilityPlan)
func TestRepoCheckCapabilityPlan(t *testing.T) {
	t.Run("aggregate prepares each capability once and preserves successful output order", func(t *testing.T) {
		cfg := &config.Config{DocsDir: "docs", ProseGate: &config.ProseGateConfig{Enabled: true}, MemoryCite: &config.MemoryCiteConfig{Enabled: true}}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Notes: []string{"project-advisory-sentinel"}}, project.CurrentStateReport{}, tree, counts)
		var out bytes.Buffer
		err = runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepMemory, repoStepProse, repoStepState, repoStepDrift}, execution.ContinueOnFailure, true, deps)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := *counts, (repoCheckCounters{loads: 1, opens: 1, reports: 1, states: 1, indexes: 1}); got != want {
			t.Fatalf("capability counts = %+v, want %+v", got, want)
		}
		want := "note: project-advisory-sentinel\nawf check repo drift: clean\nawf check repo state: clean\ncheck repo prose: clean\ncheck repo memory: clean\n"
		if got := out.String(); got != want {
			t.Fatalf("output = %q, want %q", got, want)
		}
	})

	t.Run("aggregate keeps universes distinct and continues after action errors", func(t *testing.T) {
		cfg := &config.Config{DocsDir: "docs", ProseGate: &config.ProseGateConfig{Enabled: true}, MemoryCite: &config.MemoryCiteConfig{Enabled: true}}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree([]snapshot.File{
			{Path: "prose-index-sentinel.txt", Bytes: []byte("bad \u2014")},
			{Path: "docs/decisions/memory-index-sentinel.md", Bytes: []byte(".awf/efforts/example/memory.md")},
		})
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		check := project.CheckReport{
			Notes: []string{"working-advisory-sentinel"},
			Drift: []manifest.Drift{{Kind: "changed", Path: "working-drift-sentinel", Detail: "working bytes"}},
		}
		state := project.CurrentStateReport{Static: []currentstate.Finding{{Message: "current-state-sentinel"}}}
		deps := repoCheckTestDependencies(t, cfg, p, check, state, tree, counts)
		var out bytes.Buffer
		err = runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepDrift, repoStepState, repoStepProse, repoStepMemory}, execution.ContinueOnFailure, true, deps)
		if err == nil || !strings.Contains(err.Error(), "awf check repo drift") {
			t.Fatalf("error = %v, want first drift action error", err)
		}
		if got, want := *counts, (repoCheckCounters{loads: 1, opens: 1, reports: 1, states: 1, indexes: 1}); got != want {
			t.Fatalf("capability counts = %+v, want %+v", got, want)
		}
		text := out.String()
		ordered := []string{"working-advisory-sentinel", "working-drift-sentinel", "current-state-sentinel", "prose-index-sentinel.txt", "memory-index-sentinel.md"}
		position := -1
		for _, sentinel := range ordered {
			next := strings.Index(text, sentinel)
			if next <= position {
				t.Fatalf("output does not preserve universe/action order %v: %q", ordered, text)
			}
			position = next
		}
	})

	t.Run("later preparation failure emits no action or advisory output", func(t *testing.T) {
		cfg := &config.Config{ProseGate: &config.ProseGateConfig{Enabled: true}}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Notes: []string{"must-not-print"}}, project.CurrentStateReport{}, tree, counts)
		failure := errors.New("current-state preparation failed")
		deps.currentState = func(context.Context, *project.Project) (project.CurrentStateReport, error) {
			counts.states++
			return project.CurrentStateReport{}, failure
		}
		var out bytes.Buffer
		err = runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepDrift, repoStepState, repoStepProse}, execution.ContinueOnFailure, true, deps)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v, want %v", err, failure)
		}
		if out.Len() != 0 {
			t.Fatalf("preparation failure output = %q, want empty", out.String())
		}
		if got, want := *counts, (repoCheckCounters{loads: 1, opens: 1, reports: 1, states: 1}); got != want {
			t.Fatalf("capability counts = %+v, want %+v", got, want)
		}
	})

	t.Run("direct selections acquire no unrelated capability", func(t *testing.T) {
		cases := []struct {
			name     string
			cfg      *config.Config
			step     execution.StepID
			want     repoCheckCounters
			wantText string
		}{
			{name: "drift", cfg: &config.Config{}, step: repoStepDrift, want: repoCheckCounters{loads: 1, opens: 1, reports: 1}, wantText: "awf check repo drift: clean\n"},
			{name: "state", cfg: &config.Config{}, step: repoStepState, want: repoCheckCounters{loads: 1, opens: 1, states: 1}, wantText: "awf check repo state: clean\n"},
			{name: "prose enabled", cfg: &config.Config{ProseGate: &config.ProseGateConfig{Enabled: true}}, step: repoStepProse, want: repoCheckCounters{loads: 1, indexes: 1}, wantText: "check repo prose: clean\n"},
			{name: "memory disabled", cfg: &config.Config{}, step: repoStepMemory, want: repoCheckCounters{loads: 1}, wantText: "note: memory: disabled (memoryCite.enabled)\n"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := &project.Project{Root: "working-project-sentinel", Cfg: tc.cfg}
				tree, err := snapshot.NewTree(nil)
				if err != nil {
					t.Fatal(err)
				}
				counts := &repoCheckCounters{}
				deps := repoCheckTestDependencies(t, tc.cfg, p, project.CheckReport{Notes: []string{"aggregate-only"}}, project.CurrentStateReport{}, tree, counts)
				var out bytes.Buffer
				if err := runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{tc.step}, execution.StopOnFailure, false, deps); err != nil {
					t.Fatal(err)
				}
				if *counts != tc.want {
					t.Fatalf("capability counts = %+v, want %+v", *counts, tc.want)
				}
				if got := out.String(); got != tc.wantText {
					t.Fatalf("output = %q, want %q", got, tc.wantText)
				}
			})
		}
	})

	t.Run("execution cancellation remains separate from outcomes", func(t *testing.T) {
		cfg := &config.Config{}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{}, project.CurrentStateReport{}, tree, counts)
		ctx, cancel := context.WithCancel(context.Background())
		writer := &cancelOnWrite{cancel: cancel}
		err = runRepoCheckSelection(ctx, t.TempDir(), writer, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, deps)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context cancellation", err)
		}
	})
}

type cancelOnWrite struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *cancelOnWrite) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	w.cancel()
	return n, err
}
