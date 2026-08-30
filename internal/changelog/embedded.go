package changelog

import (
	"io/fs"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
)

// Embedded returns the complete authored changelog payload.
func Embedded() (string, error) {
	raw, err := fs.ReadFile(changelogfs.FS, "CHANGELOG.md")
	return string(raw), err
}

// EmbeddedVersion returns one selected release payload.
func EmbeddedVersion(version string) (string, error) {
	entries, err := Load(changelogfs.FS)
	if err != nil {
		return "", err
	}
	entry, err := Version(entries, version)
	return entry.Raw, err
}

// EmbeddedSince returns release payloads newer than version.
func EmbeddedSince(version string) ([]string, error) {
	entries, err := Load(changelogfs.FS)
	if err != nil {
		return nil, err
	}
	matched, err := Since(entries, version)
	return rawPayloads(matched), err
}

// EmbeddedRange returns release payloads in the inclusive selected range.
func EmbeddedRange(from, to string) ([]string, error) {
	entries, err := Load(changelogfs.FS)
	if err != nil {
		return nil, err
	}
	matched, err := Range(entries, from, to)
	return rawPayloads(matched), err
}

func rawPayloads(entries []Entry) []string {
	payloads := make([]string, len(entries))
	for i, entry := range entries {
		payloads[i] = entry.Raw
	}
	return payloads
}
