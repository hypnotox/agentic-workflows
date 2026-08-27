package initop

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

type scaffoldLinkResult struct {
	info fs.FileInfo
	err  error
}

type scaffoldFaultFilesystem struct {
	links            []scaffoldLinkResult
	mkdirInfo        fs.FileInfo
	mkdirIdentity    *filesystem.ExpectedIdentity
	expectedIdentity *filesystem.ExpectedIdentity
	mkdirErr         error
	publishErr       error
}

func (f *scaffoldFaultFilesystem) LinkInfo(string) (fs.FileInfo, error) {
	result := f.links[0]
	f.links = f.links[1:]
	return result.info, result.err
}
func (f *scaffoldFaultFilesystem) CreateDirectory(string, fs.FileMode) (*filesystem.ExpectedIdentity, error) {
	return f.mkdirIdentity, f.mkdirErr
}
func (f *scaffoldFaultFilesystem) ExpectedIdentity(string) (*filesystem.ExpectedIdentity, error) {
	if f.expectedIdentity != nil {
		return f.expectedIdentity, nil
	}
	result := f.links[0]
	f.links = f.links[1:]
	return nil, result.err
}
func (f *scaffoldFaultFilesystem) Publish(string, []byte, fs.FileMode) error {
	return f.publishErr
}

func TestCreateScaffoldDoesNotClaimDirectoryWonByConcurrentCreator(t *testing.T) {
	directory := t.TempDir()
	dirInfo, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(directory, "config")
	if err := os.WriteFile(configFile, []byte("config"), 0o644); err != nil {
		t.Fatal(err)
	}
	configInfo, err := os.Lstat(configFile)
	if err != nil {
		t.Fatal(err)
	}
	filesystem := &scaffoldFaultFilesystem{
		links:    []scaffoldLinkResult{{err: fs.ErrNotExist}, {info: dirInfo}, {info: configInfo}},
		mkdirErr: fs.ErrExist,
	}
	scaffold, err := createScaffold(filesystem, []byte("config"))
	if err != nil {
		t.Fatal(err)
	}
	if scaffold.createdDir || !scaffold.configCommitted {
		t.Fatalf("scaffold ownership = %#v; want only config committed", scaffold)
	}
}

func TestCreateScaffoldRetainsIdentityReturnedByDirectoryCreation(t *testing.T) {
	createdDirectory := t.TempDir()
	createdInfo, err := os.Lstat(createdDirectory)
	if err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(createdDirectory, "config")
	if err := os.WriteFile(configFile, []byte("config"), 0o644); err != nil {
		t.Fatal(err)
	}
	configInfo, err := os.Lstat(configFile)
	if err != nil {
		t.Fatal(err)
	}
	h, err := filesystem.Open(createdDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	dirIdentity, err := h.ExpectedIdentity(".")
	if err != nil {
		t.Fatal(err)
	}
	configIdentity, err := h.ExpectedIdentity("config")
	if err != nil {
		t.Fatal(err)
	}
	filesystem := &scaffoldFaultFilesystem{
		links:     []scaffoldLinkResult{{err: fs.ErrNotExist}, {info: configInfo}},
		mkdirInfo: createdInfo, mkdirIdentity: dirIdentity, expectedIdentity: configIdentity,
	}
	scaffold, err := createScaffold(filesystem, []byte("config"))
	if err != nil || !scaffold.createdDir || !scaffold.dirInfo.SameFile(createdInfo) {
		t.Fatalf("scaffold creation identity = %#v, %v", scaffold, err)
	}
}

func TestCreateScaffoldPreservesExclusiveDirectoryCreationFailure(t *testing.T) {
	want := errors.New("mkdir failed")
	scaffold, err := createScaffold(&scaffoldFaultFilesystem{
		links: []scaffoldLinkResult{{err: fs.ErrNotExist}}, mkdirErr: want,
	}, []byte("config"))
	if !errors.Is(err, want) || scaffold.committed() {
		t.Fatalf("exclusive mkdir outcome = %#v, %v; want unchanged failure", scaffold, err)
	}
}

func TestCreateScaffoldPreservesConcurrentDirectoryObservationFailure(t *testing.T) {
	want := errors.New("observe concurrent directory winner")
	scaffold, err := createScaffold(&scaffoldFaultFilesystem{
		links:    []scaffoldLinkResult{{err: fs.ErrNotExist}, {err: want}},
		mkdirErr: fs.ErrExist,
	}, []byte("config"))
	if !errors.Is(err, fs.ErrExist) || !errors.Is(err, want) || scaffold.committed() {
		t.Fatalf("concurrent directory observation = %#v, %v; want unchanged joined failure", scaffold, err)
	}
}

func TestCreateScaffoldReportsEveryPostDirectoryAndPublicationEffect(t *testing.T) {
	dir := t.TempDir()
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected scaffold failure")
	for _, tc := range []struct {
		name          string
		filesystem    *scaffoldFaultFilesystem
		wantConfig    bool
		wantDirectory bool
		wantResidue   int
	}{
		{
			name: "post-directory-publication",
			filesystem: &scaffoldFaultFilesystem{
				links:      []scaffoldLinkResult{{err: fs.ErrNotExist}},
				mkdirInfo:  dirInfo,
				publishErr: failure,
			},
			wantDirectory: true,
		},
		{
			name: "committed-publication-cleanup",
			filesystem: &scaffoldFaultFilesystem{
				links: []scaffoldLinkResult{{info: dirInfo}},
				publishErr: &filepublication.CommittedCleanupError{
					DestinationPath: ".awf/config.yaml", ResiduePath: ".awf/.config-residue", Cause: failure,
				},
			},
			wantConfig: true, wantResidue: 1,
		},
		{
			name:       "post-publish-observation",
			filesystem: &scaffoldFaultFilesystem{links: []scaffoldLinkResult{{info: dirInfo}, {err: failure}}},
			wantConfig: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scaffold, err := createScaffold(tc.filesystem, []byte("config"))
			if err == nil {
				t.Fatal("createScaffold succeeded")
			}
			if scaffold.configCommitted != tc.wantConfig || scaffold.createdDir != tc.wantDirectory || len(scaffold.residue) != tc.wantResidue {
				t.Fatalf("scaffold = %#v", scaffold)
			}
			outcome, partialErr := scaffoldPartialOutcome("config", scaffold.configCommitted, scaffold.createdDir, scaffold.residue, err)
			var partial *PartialError
			if !errors.As(partialErr, &partial) || outcome.ConfigPath == "" || len(outcome.Sync.Changes) != 1 {
				t.Fatalf("partial outcome = %#v, %v", outcome, partialErr)
			}
			wantValues := 0
			if tc.wantConfig {
				wantValues++
			}
			if tc.wantDirectory {
				wantValues++
			}
			wantValues += tc.wantResidue
			if got := len(outcome.Sync.Changes[0].Values); got != wantValues {
				t.Fatalf("effect values = %d, want %d", got, wantValues)
			}
		})
	}
}

func TestRollbackScaffoldRestoresOwnedTreeOrReportsChangedIdentity(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "changed"}[changed], func(t *testing.T) {
			root := t.TempDir()
			cfgPath := config.ConfigPath(root)
			if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cfgPath, []byte("owned"), 0o644); err != nil {
				t.Fatal(err)
			}
			h, err := filesystem.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			configIdentity, err := h.ExpectedIdentity(".awf/config.yaml")
			if err != nil {
				t.Fatal(err)
			}
			dirIdentity, err := h.ExpectedIdentity(".awf")
			if err != nil {
				t.Fatal(err)
			}
			_ = h.Close()
			if changed {
				if err := os.Remove(cfgPath); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cfgPath, []byte("winner"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want := errors.New("later failure")
			outcome, gotErr := rollbackScaffold(root, cfgPath, scaffoldCommit{configCommitted: true, createdDir: true, configInfo: configIdentity, dirInfo: dirIdentity}, want)
			if !errors.Is(gotErr, want) {
				t.Fatalf("rollback error = %v, want %v", gotErr, want)
			}
			if changed {
				var partial *PartialError
				if !errors.As(gotErr, &partial) || outcome.ConfigPath != cfgPath {
					t.Fatalf("changed rollback = %#v, %v; want typed partial", outcome, gotErr)
				}
				if got, err := os.ReadFile(cfgPath); err != nil || string(got) != "winner" {
					t.Fatalf("winner = %q, %v", got, err)
				}
				return
			}
			if outcome.ConfigPath != "" {
				t.Fatalf("successful rollback outcome = %#v", outcome)
			}
			if _, err := os.Stat(filepath.Join(root, config.DirName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("owned scaffold survived rollback: %v", err)
			}
		})
	}
}

func TestRunPostPublicationFailuresPresentCompletePublisherAndScaffoldEffects(t *testing.T) {
	for _, releaseFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "advisory", true: "lease-release"}[releaseFailure], func(t *testing.T) {
			root := t.TempDir()
			loader := func(string) (*project.Loader, error) {
				return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(_ context.Context, selected string) string { return selected }), nil
			}
			want := errors.New("post-publication failure")
			advisory := func(state *project.ProjectState, cfg *config.Config, prepared publisher.Preparation) ([]string, error) {
				if !releaseFailure {
					return nil, want
				}
				return project.AdvisoryNotes(state, cfg, prepared.Plan(), projectSemantics(prepared))
			}
			release := func(lease *filesystem.Lease) error {
				err := lease.Release()
				if releaseFailure {
					return errors.Join(err, want)
				}
				return err
			}
			outcome, err := runWithDependencies(context.Background(), Input{Root: root, Force: true}, loader, func(context.Context, string) error { return nil }, advisory, release)
			var partial *PartialError
			if !errors.Is(err, want) || !errors.As(err, &partial) {
				t.Fatalf("post-publication outcome = %#v, %v; want typed partial", outcome, err)
			}
			document, renderErr := outcome.Document()
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			var rendered bytes.Buffer
			if err := presentation.Render(&rendered, document); err != nil {
				t.Fatal(err)
			}
			got := rendered.String()
			for _, wantEffect := range []string{"config-created .awf/config.yaml", "directory-created .awf", "lock-replaced .awf/awf.lock", "recovery:"} {
				if !strings.Contains(got, wantEffect) {
					t.Fatalf("partial output missing %q:\n%s", wantEffect, got)
				}
			}
		})
	}
}

func TestRunPropagatesLoaderOpenFailure(t *testing.T) {
	root := t.TempDir()
	want := errors.New("open project")
	_, err := Run(context.Background(), Input{Root: root, Force: true}, func(string) (*project.Loader, error) {
		return project.NewLoaderWithoutRepository(
			func(string) (*config.Config, error) { return nil, want },
			catalog.Standard,
			func(_ context.Context, selected string) string { return selected },
		), nil
	}, func(context.Context, string) error {
		t.Fatal("gate called after loader open failure")
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("Run error = %v, want %v", err, want)
	}
	if _, statErr := os.Stat(filepath.Join(root, config.DirName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("loader failure left scaffold residue: %v", statErr)
	}
}

func TestRunGateFailureRollsBackOwnedScaffold(t *testing.T) {
	root := t.TempDir()
	want := errors.New("gate failed")
	loader := func(string) (*project.Loader, error) {
		return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(_ context.Context, selected string) string { return selected }), nil
	}
	_, err := Run(context.Background(), Input{Root: root, Force: true}, loader, func(context.Context, string) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("Run error = %v, want %v", err, want)
	}
	if _, statErr := os.Stat(filepath.Join(root, config.DirName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("gate failure left scaffold residue: %v", statErr)
	}
}

func TestProbeCollisionsPropagatesLoaderConstructionFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(config.ConfigPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := errors.New("loader failed")
	_, err := probeCollisions(context.Background(), root, func(string) (*project.Loader, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("probe error = %v, want %v", err, want)
	}
}
