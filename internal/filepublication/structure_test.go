package filepublication

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestExclusivePublicationHasOneReleasedPlatformHome)
func TestExclusivePublicationHasOneReleasedPlatformHome(t *testing.T) {
	var findings []string
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(path string, body []byte) {
		findings = append(findings, exclusivePublicationFindings(path, string(body))...)
	})
	if len(findings) != 0 {
		t.Fatalf("released-platform no-replace publication exists outside internal/filepublication:\n\t%s", strings.Join(findings, "\n\t"))
	}
}

func TestExclusivePublicationDetectorRejectsSecondHomes(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		body string
	}{
		{name: "Linux no-replace rename", path: "internal/other/linux.go", body: "unix.Renameat2(a, b, c, d, unix.RENAME_NOREPLACE)"},
		{name: "Darwin exclusive rename", path: "internal/other/darwin.go", body: "unix.RenamexNp(a, b, unix.RENAME_EXCL)"},
		{name: "Windows no-replace move", path: "internal/other/windows.go", body: "windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH)"},
		{name: "effort creation branch", path: "internal/effort/publication.go", body: "if expected == nil { return publishAtomic(a, b, nil) }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := exclusivePublicationFindings(test.path, test.body); len(got) == 0 {
				t.Fatal("duplicate implementation was not detected")
			}
		})
	}
}

func exclusivePublicationFindings(path, body string) []string {
	if strings.HasPrefix(path, "internal/filepublication/") {
		if strings.Contains(body, "internal/effort") {
			return []string{path + ": publication leaf depends outward on effort"}
		}
		return nil
	}

	var findings []string
	for _, token := range []string{"RENAME_NOREPLACE", "RENAME_EXCL"} {
		if strings.Contains(body, token) {
			findings = append(findings, path+": contains "+token)
		}
	}
	if strings.Contains(body, "MoveFileEx") && path != "internal/effort/publication_windows.go" {
		findings = append(findings, path+": contains MoveFileEx")
	}
	if strings.Contains(path, "publication") && strings.Contains(body, "expected == nil") && strings.Contains(body, "publishAtomic") {
		findings = append(findings, path+": retains an expected-absent effort publication branch")
	}
	return findings
}
