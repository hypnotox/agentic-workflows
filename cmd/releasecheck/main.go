// Command releasecheck is the release-time changelog pin (ADR-0078). The every-commit
// gate only guarantees ordering (entries strictly descending, newest at or below
// project.Version); this check closes the exact match at the one moment it matters:
// the Release workflow runs it before GoReleaser, and the release runbook runs it
// locally as the pre-tag rehearsal. It fails unless the newest embedded changelog
// entry equals project.Version and a standing [Unreleased] section is present and
// empty modulo whitespace - so a tag can neither ship without its own release notes
// nor strand late entries under [Unreleased].
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
	"github.com/hypnotox/agentic-workflows/internal/changelog"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

func main() { os.Exit(run(changelogfs.FS, os.Stdout, os.Stderr, project.BridgeTrancheComplete)) } // coverage-ignore: os.Exit wrapper; run is unit-tested

// touches-invariant: release-changelog-pin - changelog ordering/pin enforcement site; proof in main_test.go
func run(fsys fs.FS, stdout, stderr io.Writer, bridgeTrancheComplete bool) int {
	if !bridgeTrancheComplete {
		fmt.Fprintln(stderr, "releasecheck: current-state bridge tranche is incomplete; Plans 1 and 2 must both land before release")
		return 1
	}
	raw, err := fs.ReadFile(fsys, "CHANGELOG.md")
	if err != nil {
		fmt.Fprintf(stderr, "releasecheck: read CHANGELOG.md: %v\n", err)
		return 1
	}
	entries, err := changelog.Parse(raw)
	if err != nil {
		fmt.Fprintf(stderr, "releasecheck: %v\n", err)
		return 1
	}
	fails := 0
	if entries[0].Version != project.Version {
		fmt.Fprintf(stderr, "releasecheck: newest changelog entry %s != project.Version %s; promote [Unreleased] before tagging\n",
			entries[0].Version, project.Version)
		fails++
	}
	// Ordering is the gate test's job (inv: changelog-monotonic), but release CI
	// runs no tests - re-check it here so a mis-sorted file cannot make a stray
	// newer entry pass as pinned merely because entries[0] matched.
	for i := 0; i+1 < len(entries); i++ {
		if semver.Compare("v"+entries[i].Version, "v"+entries[i+1].Version) <= 0 {
			fmt.Fprintf(stderr, "releasecheck: changelog entries out of order: %s is not strictly newer than %s\n",
				entries[i].Version, entries[i+1].Version)
			fails++
		}
	}
	switch body, found := unreleasedBody(string(raw)); {
	case !found:
		fmt.Fprintln(stderr, "releasecheck: no ## [Unreleased] section; restore the standing header (the changelog-unreleased audit rule keys on it)")
		fails++
	case strings.TrimSpace(body) != "":
		fmt.Fprintln(stderr, "releasecheck: [Unreleased] is not empty; fold its entries into the release section before tagging")
		fails++
	}
	if fails > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "releasecheck: changelog pins %s and [Unreleased] is empty\n", project.Version)
	return 0
}

// unreleasedBody returns the body between the "## [Unreleased]" header and the next
// top-level "## [" header (or EOF), and whether the header was found at all. The
// section walk deliberately duplicates repoaudit's git-bound unreleasedSection: the
// two read from different sources (embedded bytes vs `git show`), per ADR-0078.
func unreleasedBody(raw string) (string, bool) {
	var body []string
	in := false
	for _, ln := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(ln, "## [Unreleased]"):
			in = true
		case in && strings.HasPrefix(ln, "## ["):
			return strings.Join(body, "\n"), true
		case in:
			body = append(body, ln)
		}
	}
	if !in {
		return "", false
	}
	return strings.Join(body, "\n"), true
}
