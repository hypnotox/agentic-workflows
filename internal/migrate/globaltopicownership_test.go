package migrate

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestGlobalTopicPathOwnershipGeneration(t *testing.T) {
	if globalTopicPathOwnershipGeneration != 41 {
		t.Fatalf("global topic path ownership generation = %d, want 41", globalTopicPathOwnershipGeneration)
	}
	migration := registry[len(registry)-6]
	if migration.To != globalTopicPathOwnershipGeneration || migration.Name != "global-topic-path-ownership" {
		t.Fatalf("global topic path ownership migration = %#v", migration)
	}
}

func TestEffortArchiveRootGeneration(t *testing.T) {
	if effortArchiveGeneration != 42 || Current() != profileGeneration {
		t.Fatalf("effort archive generation = %d, current = %d; want current profile generation %d", effortArchiveGeneration, Current(), profileGeneration)
	}
	archive := registry[len(registry)-5]
	if archive.To != effortArchiveGeneration || archive.Name != "effort-archive-root" {
		t.Fatalf("archive migration = %#v", archive)
	}
}

func TestGlobalTopicPathOwnershipUpgradeOnlyStampsSchema(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: example\nintegrationBranch: main\nvars: {}\n")
	pathOnly := filepath.Join(root, ".awf", "topics", "metadata", "core", "scoped.yaml")
	globalOnly := filepath.Join(root, ".awf", "topics", "metadata", "core", "global.yaml")
	testsupport.WriteFile(t, pathOnly, "title: Scoped\nsummary: Scoped topic.\npaths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, globalOnly, "title: Global\nsummary: Global topic.\napplies: global\n")
	stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), globalTopicPathOwnershipGeneration-1)
	beforePath, err := os.ReadFile(pathOnly)
	if err != nil {
		t.Fatal(err)
	}
	beforeGlobal, err := os.ReadFile(globalOnly)
	if err != nil {
		t.Fatal(err)
	}
	applied, changes, err := upgradeLegacyForTest(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(applied, []string{"global-topic-path-ownership", "effort-archive-root", "pitfall-corpus", "template-source-root", "local-docs", "workflow-profile"}) || len(changes) != 2 || changes[0].Text != "workflow-profile: selected full for an existing repository" || changes[1].Text != "schema-stamp: updated awf.lock schema version" {
		t.Fatalf("upgrade = %v, %v", applied, changes)
	}
	for path, want := range map[string][]byte{pathOnly: beforePath, globalOnly: beforeGlobal} {
		got, err := os.ReadFile(path)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("metadata %s = %q, %v; want unchanged %q", path, got, err, want)
		}
	}
}
