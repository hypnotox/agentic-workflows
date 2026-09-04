package topicop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/topic"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func scaffoldFixture(t *testing.T) (string, *project.Loader) {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: make gate}\ndomains: [tooling]\n")
	return root, project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
}

func TestCreateExclusiveRerunConvergesAndDifferingCollisionRefuses(t *testing.T) {
	root, loader := scaffoldFixture(t)
	first, err := Create(context.Background(), root, "tooling", "Safety Model", loader, nil)
	if err != nil || len(first.Created) != 2 {
		t.Fatalf("first=%#v %v", first, err)
	}
	second, err := Create(context.Background(), root, "tooling", "Safety Model", loader, nil)
	if err != nil || len(second.Created) != 0 {
		t.Fatalf("rerun=%#v %v", second, err)
	}
	path := filepath.Join(root, filepath.FromSlash(first.Created[0]))
	if err := os.WriteFile(path, []byte("foreign"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(context.Background(), root, "tooling", "Safety Model", loader, nil); err == nil {
		t.Fatal("differing collision accepted")
	}
}

type failingScaffold struct {
	published int
	fault     error
	visible   map[string][]byte
}

func (f *failingScaffold) LinkInfo(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
func (f *failingScaffold) Read(string) ([]byte, error)          { return nil, os.ErrNotExist }
func (f *failingScaffold) MkdirAll(string, os.FileMode) error   { return nil }
func (f *failingScaffold) Publish(path string, content []byte, _ os.FileMode) error {
	f.published++
	if f.published == 2 {
		return f.fault
	}
	f.visible[path] = append([]byte(nil), content...)
	return nil
}

func TestLaterCreateFailureLeavesEarlierPathVisibleInOutcome(t *testing.T) {
	fault := errors.New("second create sentinel")
	files := &failingScaffold{fault: fault, visible: map[string][]byte{}}
	created, err := createPlanned(files, []topic.ScaffoldFile{{Path: "first", Content: []byte("one")}, {Path: "second", Content: []byte("two")}})
	if !errors.Is(err, fault) || len(created) != 1 || created[0] != "first" || string(files.visible["first"]) != "one" {
		t.Fatalf("created=%#v visible=%#v error=%v", created, files.visible, err)
	}
}

func TestCreateAcquiresTrackedLeaseBeforeAuthorityAndJoinsRelease(t *testing.T) {
	root, _ := scaffoldFixture(t)
	acquired := false
	observing := false
	loader := project.NewLoaderWithoutRepository(func(path string) (*config.Config, error) {
		if observing && !acquired {
			t.Fatal("read before lease")
		}
		return config.Load(path)
	}, catalog.Standard, func(context.Context, string) string { return root })
	fault := errors.New("release sentinel")
	acquire := func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		acquired = true
		lease, err := filesystem.AcquireTrackedLease(ctx, root)
		return lease, func() error { return errors.Join(lease.Release(), fault) }, err
	}
	observing = true
	outcome, err := Create(context.Background(), root, "tooling", "Lease", loader, acquire)
	if len(outcome.Created) != 2 || !errors.Is(err, fault) {
		t.Fatalf("outcome=%#v error=%v", outcome, err)
	}
}
