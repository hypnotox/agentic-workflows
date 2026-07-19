package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestSnapshotMarkerAndCutoffFacts(t *testing.T) {
	cutoff, gaps := cutoffFacts([]string{"0001", "0003"})
	if cutoff != 4 || len(gaps) != 1 || gaps[0] != 2 {
		t.Fatalf("%d %v", cutoff, gaps)
	}
	if err := validateHeadMarkers(t.TempDir(), nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"x.md"}, Marker: "<!--", Close: "-->"}}}
	blob := awfgit.HeadBlob{Path: "x.md", Bytes: []byte("<!-- ordinary\n<!-- invariant: x -->\n<!-- touches-invariant: x - note -->\n<!-- invariant: retired -->\n")}
	mapping := []Mapping{{Key: "ADR-0001#x", Destination: "core/x:x"}}
	after := []byte("<!-- invariant: core/x:x -->\n<!-- touches-state: core/x:x - note -->\n")
	if err := validateHeadMarkers(root, cfg, []awfgit.HeadBlob{blob}, mapping, []Mutation{{Path: "x.md", AfterPresent: true, After: after}}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "x.md"), after, 0o644)
	if err := validateHeadMarkers(root, cfg, []awfgit.HeadBlob{blob}, mapping, nil); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(root, "x.md"))
	if err := validateHeadMarkers(root, cfg, []awfgit.HeadBlob{blob}, mapping, nil); err == nil || !strings.Contains(err.Error(), "disappeared") {
		t.Fatalf("%v", err)
	}
	if err := validateHeadMarkers(root, cfg, []awfgit.HeadBlob{blob}, mapping, []Mutation{{Path: "x.md", AfterPresent: true, After: []byte("wrong")}}); err == nil || !strings.Contains(err.Error(), "not preserved") {
		t.Fatalf("%v", err)
	}
}

func TestValidateLegacySnapshotErrors(t *testing.T) {
	if err := ValidateLegacySnapshot(t.TempDir(), adr.NewCorpus(nil), Inventory{}, nil, nil); err == nil {
		t.Fatal("non-repository accepted")
	}
	root := validBridgeProject(t)
	os.Remove(filepath.Join(root, ".awf", "config.yaml"))
	runGit(t, root, "add", "-u")
	runGit(t, root, "commit", "-qm", "no config")
	if err := ValidateLegacySnapshot(root, adr.NewCorpus(nil), Inventory{}, nil, nil); err == nil || !strings.Contains(err.Error(), "lacks") {
		t.Fatalf("%v", err)
	}
	root = validBridgeProject(t)
	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), []byte("bad: [\n"), 0o644)
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "bad config")
	if err := ValidateLegacySnapshot(root, adr.NewCorpus(nil), Inventory{}, nil, nil); err == nil {
		t.Fatal("bad HEAD config accepted")
	}
}

func TestValidateLegacySnapshotCorpusAndInventoryErrors(t *testing.T) {
	root := validBridgeProject(t)
	mustWrite(t, filepath.Join(root, "docs", "decisions", "0001-bad.md"), []byte("---\nstatus: [\n---\n"), 0o644)
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "bad adr")
	if err := ValidateLegacySnapshot(root, adr.NewCorpus(nil), Inventory{}, nil, nil); err == nil {
		t.Fatal("bad HEAD ADR accepted")
	}
	root = validBridgeProject(t)
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "- `invariant: x` - x.", "1. x.", "\n## Migration history\n\n- 2026-07-19: retired invariant `ADR-0001#x`; basis: encoded\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "bad history")
	if err := ValidateLegacySnapshot(root, adr.NewCorpus(nil), Inventory{}, nil, nil); err == nil {
		t.Fatal("bad HEAD inventory accepted")
	}
	root = validBridgeProject(t)
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "- `invariant: x` - x.", "1. x.", "")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "adr")
	corpus := bridgeCorpus(t, filepath.Join(root, "docs", "decisions"))
	inventory, err := BuildInventory(corpus)
	if err != nil {
		t.Fatal(err)
	}
	inventory.Entries[0].Backing = "unbacked"
	if err := ValidateLegacySnapshot(root, corpus, inventory, nil, nil); err == nil || !strings.Contains(err.Error(), "mismatch at") {
		t.Fatalf("%v", err)
	}
}

func TestValidateLegacySnapshotStatusMismatch(t *testing.T) {
	root := validBridgeProject(t)
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "- `invariant: x` - x.", "1. x.", "")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "adr")
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Superseded", "- `invariant: x` - x.", "1. x.", "")
	corpus := bridgeCorpus(t, filepath.Join(root, "docs", "decisions"))
	inventory, err := BuildInventory(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLegacySnapshot(root, corpus, inventory, nil, nil); err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("%v", err)
	}
}

func TestValidateLegacySnapshot(t *testing.T) {
	root := validBridgeProject(t)
	inventory, err := BuildInventory(bridgeCorpus(t, filepath.Join(root, "docs", "decisions")))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLegacySnapshot(root, bridgeCorpus(t, filepath.Join(root, "docs", "decisions")), inventory, nil, nil); err != nil {
		t.Fatal(err)
	}
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "- `invariant: x` - x.", "1. x.", "")
	inventory, err = BuildInventory(bridgeCorpus(t, filepath.Join(root, "docs", "decisions")))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLegacySnapshot(root, bridgeCorpus(t, filepath.Join(root, "docs", "decisions")), inventory, nil, nil); err == nil || (!strings.Contains(err.Error(), "entries") && !strings.Contains(err.Error(), "identity")) {
		t.Fatalf("expected mismatch, got %v", err)
	}
}
