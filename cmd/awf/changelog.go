package main

import (
	"fmt"
	"io"
	"io/fs"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
	"github.com/hypnotox/agentic-workflows/internal/changelog"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// runChangelog prints the embedded CHANGELOG.md, or a version/since/range-filtered
// slice of it. version/since/rng are mutually exclusive; checkArgs has already
// validated the flag names and zero positional arity, but not this mutual
// exclusivity or --range's "from..to" shape.
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
		return writeChangelogPayload(stdout, e.Raw)
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
			return writeStatus(stdout, "no releases since "+since)
		}
		for _, e := range matched {
			if err := writeChangelogPayload(stdout, e.Raw+"\n"); err != nil {
				return err
			}
		}
	case rng != "":
		from, to, perr := awfgit.ParseRange(rng, false)
		if perr != nil {
			return &usageErr{fmt.Sprintf("awf changelog: --range %v", perr)}
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
			if err := writeChangelogPayload(stdout, e.Raw+"\n"); err != nil {
				return err
			}
		}
	default:
		b, err := fs.ReadFile(changelogfs.FS, "CHANGELOG.md")
		if err != nil { // coverage-ignore: same embedded-asset guarantee as changelog.Load above
			return err
		}
		return writeChangelogPayload(stdout, string(b))
	}
	return nil
}

// writeChangelogPayload writes authored changelog bytes unchanged. It is a
// deliberately closed successful payload bypass.
func writeChangelogPayload(stdout io.Writer, payload string) error {
	_, err := io.WriteString(stdout, payload)
	return err
}
