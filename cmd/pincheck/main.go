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

	"gopkg.in/yaml.v3"
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

// checkFile parses workflow structure so a version belongs only to the same
// jobs.*.steps[] mapping as its goreleaser action.
func checkFile(name, content string, stderr io.Writer) int {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		fmt.Fprintf(stderr, "pincheck: %s: malformed YAML: %v\n", name, err)
		return 1
	}
	if err := uniqueMappingKeys(&root); err != nil {
		fmt.Fprintf(stderr, "pincheck: %s: malformed YAML: %v\n", name, err)
		return 1
	}
	fails := 0
	for _, step := range workflowSteps(&root) {
		uses, usesLine, hasUses := mappingValue(step, "uses")
		if !hasUses || uses.Kind != yaml.ScalarNode {
			continue
		}
		ref := uses.Value
		if bad := usesViolation(ref); bad != "" {
			fmt.Fprintf(stderr, "pincheck: %s:%d: %s: %s\n", name, usesLine, bad, ref)
			fails++
		}
		if !strings.HasPrefix(ref, "goreleaser/goreleaser-action@") {
			continue
		}
		with, _, hasWith := mappingValue(step, "with")
		var version *yaml.Node
		line := usesLine
		if hasWith && with.Kind == yaml.MappingNode {
			version, line, _ = mappingValue(with, "version")
		}
		if version == nil || version.Kind != yaml.ScalarNode {
			fmt.Fprintf(stderr, "pincheck: %s:%d: goreleaser-action step has no version: input; the tool would float to latest\n", name, usesLine)
			fails++
			continue
		}
		if !exactSemver.MatchString(version.Value) {
			fmt.Fprintf(stderr, "pincheck: %s:%d: goreleaser version must be an exact vX.Y.Z, got: %s\n", name, line, version.Value)
			fails++
		}
	}
	return fails
}

func uniqueMappingKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if seen[key] {
				return fmt.Errorf("duplicate key %q at line %d", key, node.Content[i].Line)
			}
			seen[key] = true
		}
	}
	for _, child := range node.Content {
		if err := uniqueMappingKeys(child); err != nil {
			return err
		}
	}
	return nil
}

func mappingValue(mapping *yaml.Node, key string) (*yaml.Node, int, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, 0, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], mapping.Content[i].Line, true
		}
	}
	return nil, 0, false
}

func workflowSteps(root *yaml.Node) []*yaml.Node {
	if root == nil || len(root.Content) != 1 {
		return nil
	}
	jobs, _, ok := mappingValue(root.Content[0], "jobs")
	if !ok || jobs.Kind != yaml.MappingNode {
		return nil
	}
	var steps []*yaml.Node
	for i := 1; i < len(jobs.Content); i += 2 {
		sequence, _, ok := mappingValue(jobs.Content[i], "steps")
		if !ok || sequence.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range sequence.Content {
			if step.Kind == yaml.MappingNode {
				steps = append(steps, step)
			}
		}
	}
	return steps
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
