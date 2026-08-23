package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/changelog"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// runChangelog selects one focused changelog query and writes its closed
// authored payload without routing it through structured presentation.
func runChangelog(version, since, rng string, stdout io.Writer) error {
	set := 0
	for _, value := range []string{version, since, rng} {
		if value != "" {
			set++
		}
	}
	if set > 1 {
		return &usageErr{"awf changelog: --version, --since, and --range are mutually exclusive"}
	}
	switch {
	case version != "":
		payload, err := changelog.EmbeddedVersion(version)
		if err != nil {
			return err
		}
		return writeChangelogPayload(stdout, payload)
	case since != "":
		payloads, err := changelog.EmbeddedSince(since)
		if err != nil {
			return err
		}
		if len(payloads) == 0 {
			return writeStatus(stdout, "no releases since "+since)
		}
		return writeChangelogPayloads(stdout, payloads)
	case rng != "":
		from, to, err := awfgit.ParseRange(rng, false)
		if err != nil {
			return &usageErr{fmt.Sprintf("awf changelog: --range %v", err)}
		}
		payloads, err := changelog.EmbeddedRange(from, to)
		if err != nil {
			return err
		}
		return writeChangelogPayloads(stdout, payloads)
	default:
		payload, err := changelog.Embedded()
		if err != nil { // coverage-ignore: the compiled embedded changelog asset is fixed
			return err
		}
		return writeChangelogPayload(stdout, payload)
	}
}

func writeChangelogPayloads(stdout io.Writer, payloads []string) error {
	for _, payload := range payloads {
		if err := writeChangelogPayload(stdout, payload+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// writeChangelogPayload writes authored changelog bytes unchanged. It is a
// deliberately closed successful payload bypass.
func writeChangelogPayload(stdout io.Writer, payload string) error {
	_, err := io.WriteString(stdout, payload)
	return err
}
