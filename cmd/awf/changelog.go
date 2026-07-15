package main

import (
	"fmt"
	"io"
	"io/fs"
	"strings"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
	"github.com/hypnotox/agentic-workflows/internal/changelog"
)

// runChangelog prints the embedded CHANGELOG.md, or a version/since/range-filtered
// slice of it. version/since/rng are mutually exclusive; checkArgs has already
// validated the flag names and zero positional arity, but not this mutual
// exclusivity or --range's "from..to" shape.
// touches-invariant: changelog-flags-exclusive - mutually-exclusive changelog flag check; proof in changelog_test.go
func runChangelog(version, since, rng string, stdout io.Writer) error {
	set := 0
	for _, v := range []string{version, since, rng} {
		if v != "" {
			set++
		}
	}
	if set > 1 {
		return &usageErr{"awf changelog: --version, --since, and --range are mutually exclusive"}
	}
	switch {
	case version != "":
		entries, err := changelog.Load(changelogfs.FS)
		if err != nil { // coverage-ignore: changelog.Load over the embedded FS cannot fail at runtime
			return err
		}
		e, err := changelog.Version(entries, version)
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, e.Raw)
	case since != "":
		entries, err := changelog.Load(changelogfs.FS)
		if err != nil { // coverage-ignore: changelog.Load over the embedded FS cannot fail at runtime
			return err
		}
		matched, err := changelog.Since(entries, since)
		if err != nil {
			return err
		}
		if len(matched) == 0 {
			fmt.Fprintf(stdout, "no releases since %s\n", since)
			return nil
		}
		for _, e := range matched {
			fmt.Fprintln(stdout, e.Raw)
		}
	case rng != "":
		from, to, ok := strings.Cut(rng, "..")
		if !ok {
			return &usageErr{fmt.Sprintf("awf changelog: --range must be <from>..<to>, got %q", rng)}
		}
		entries, err := changelog.Load(changelogfs.FS)
		if err != nil { // coverage-ignore: changelog.Load over the embedded FS cannot fail at runtime
			return err
		}
		matched, err := changelog.Range(entries, from, to)
		if err != nil {
			return err
		}
		for _, e := range matched {
			fmt.Fprintln(stdout, e.Raw)
		}
	default:
		b, err := fs.ReadFile(changelogfs.FS, "CHANGELOG.md")
		if err != nil { // coverage-ignore: same embedded-asset guarantee as changelog.Load above
			return err
		}
		fmt.Fprint(stdout, string(b))
	}
	return nil
}
