// Command pincheck is the workflow supply-chain pin gate (ADR-0079). Every
// remote `uses:` reference under .github/workflows must pin a full 40-hex
// commit SHA (repo-local `./` references are exempt - they are repo code;
// `docker://` references must pin an image digest), and every
// goreleaser-action `version:` input must be an exact semver version, so
// neither a moved tag nor a re-floated tool range can inject unreviewed code
// into CI. ./x gate runs it on every commit.
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

func main() { os.Exit(run(os.DirFS(".github/workflows"), os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run is unit-tested

var (
	commitSHA   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	imageDigest = regexp.MustCompile(`@sha256:[0-9a-f]{64}$`)
	exactSemver = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
)

func run(fsys fs.FS, stdout, stderr io.Writer) int {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		fmt.Fprintf(stderr, "pincheck: read .github/workflows: %v\n", err)
		return 1
	}
	var files []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && (strings.HasSuffix(n, ".yml") || strings.HasSuffix(n, ".yaml")) {
			files = append(files, n)
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "pincheck: no workflow files found (run from the repo root)")
		return 1
	}
	fails := 0
	for _, name := range files {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			fmt.Fprintf(stderr, "pincheck: %s: %v\n", name, err)
			fails++
			continue
		}
		fails += checkFile(name, string(b), stderr)
	}
	if fails > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "pincheck: all workflow references pinned")
	return 0
}

// checkFile scans one workflow's lines and reports every violation. Line-based
// on purpose: the workflow YAML here is flat enough that `uses:`/`version:`
// key scans are exact, and a parser dependency would outweigh the rule.
func checkFile(name, content string, stderr io.Writer) int {
	fails := 0
	lastUses := ""
	// Line of a goreleaser-action uses: still awaiting its version: key - a step
	// without one floats the tool to latest, the same hole as a floated range.
	pendingVersion := 0
	flushPending := func() {
		if pendingVersion > 0 {
			fmt.Fprintf(stderr, "pincheck: %s:%d: goreleaser-action step has no version: input; the tool would float to latest\n", name, pendingVersion)
			fails++
			pendingVersion = 0
		}
	}
	for i, raw := range strings.Split(content, "\n") {
		ln := strings.TrimSpace(raw)
		if c := strings.Index(ln, " #"); c >= 0 {
			ln = strings.TrimSpace(ln[:c])
		}
		ln = strings.TrimPrefix(ln, "- ")
		switch {
		case strings.HasPrefix(ln, "uses:"):
			flushPending()
			ref := unquote(strings.TrimSpace(strings.TrimPrefix(ln, "uses:")))
			lastUses = ref
			if strings.HasPrefix(ref, "goreleaser/goreleaser-action@") {
				pendingVersion = i + 1
			}
			if bad := usesViolation(ref); bad != "" {
				fmt.Fprintf(stderr, "pincheck: %s:%d: %s: %s\n", name, i+1, bad, ref)
				fails++
			}
		case strings.HasPrefix(ln, "version:") && strings.HasPrefix(lastUses, "goreleaser/goreleaser-action@"):
			pendingVersion = 0
			v := unquote(strings.TrimSpace(strings.TrimPrefix(ln, "version:")))
			if !exactSemver.MatchString(v) {
				fmt.Fprintf(stderr, "pincheck: %s:%d: goreleaser version must be an exact vX.Y.Z, got: %s\n", name, i+1, v)
				fails++
			}
		}
	}
	flushPending()
	return fails
}

// usesViolation classifies a uses: reference; empty means acceptably pinned.
func usesViolation(ref string) string {
	switch {
	case strings.HasPrefix(ref, "./"):
		return "" // repo-local action: repo code, nothing to pin
	case strings.HasPrefix(ref, "docker://"):
		if imageDigest.MatchString(ref) {
			return ""
		}
		return "docker reference must pin an image digest"
	default:
		at := strings.LastIndex(ref, "@")
		if at >= 0 && commitSHA.MatchString(ref[at+1:]) {
			return ""
		}
		return "action must pin a full 40-hex commit SHA"
	}
}

func unquote(s string) string {
	return strings.Trim(s, `'"`)
}
