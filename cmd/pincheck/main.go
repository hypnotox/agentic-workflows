// Command pincheck is the workflow supply-chain pin gate (ADR-0079). Every
// remote `uses:` reference under .github/workflows must pin a full 40-hex
// commit SHA (repo-local `./` references are exempt - they are repo code;
// `docker://` references must pin an image digest), and every
// goreleaser-action `version:` input must be an exact semver version, so
// neither a moved tag nor a re-floated tool range can inject unreviewed code
// into CI. ./x gate runs it on every commit.
package main

import (
	"errors"
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

// checkFile validates the GitHub workflow shape before inspecting every use site.
// Aliases and merges are refused because their effective values are not explicit at the
// security boundary.
func checkFile(name, content string, stderr io.Writer) int {
	var root yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(content))
	if err := decoder.Decode(&root); err != nil {
		fmt.Fprintf(stderr, "pincheck: %s: malformed YAML: %v\n", name, err)
		return 1
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("multiple YAML documents are not supported")
		}
		fmt.Fprintf(stderr, "pincheck: %s: malformed YAML: %v\n", name, err)
		return 1
	}
	if err := uniqueMappingKeys(&root); err != nil || hasAliasOrMerge(&root) {
		if err == nil {
			err = fmt.Errorf("aliases and merges are not supported")
		}
		fmt.Fprintf(stderr, "pincheck: %s: malformed YAML: %v\n", name, err)
		return 1
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		fmt.Fprintf(stderr, "pincheck: %s: malformed workflow root must be a mapping\n", name)
		return 1
	}
	jobs, _, ok := mappingValue(root.Content[0], "jobs")
	if !ok || jobs.Kind != yaml.MappingNode {
		fmt.Fprintf(stderr, "pincheck: %s: malformed workflow jobs must be a mapping\n", name)
		return 1
	}
	fails := 0
	for i := 1; i < len(jobs.Content); i += 2 {
		job := jobs.Content[i]
		if job.Kind != yaml.MappingNode {
			fmt.Fprintf(stderr, "pincheck: %s:%d: malformed job must be a mapping\n", name, job.Line)
			fails++
			continue
		}
		// Reusable workflows use jobs.<id>.uses and are subject to the same pin rule.
		if uses, line, found := mappingValue(job, "uses"); found {
			fails += checkUses(name, uses, line, stderr)
		}
		steps, _, found := mappingValue(job, "steps")
		if !found {
			continue
		}
		if steps.Kind != yaml.SequenceNode {
			fmt.Fprintf(stderr, "pincheck: %s:%d: malformed steps must be a sequence\n", name, steps.Line)
			fails++
			continue
		}
		for _, step := range steps.Content {
			if step.Kind != yaml.MappingNode {
				fmt.Fprintf(stderr, "pincheck: %s:%d: malformed step must be a mapping\n", name, step.Line)
				fails++
				continue
			}
			uses, line, hasUses := mappingValue(step, "uses")
			if !hasUses {
				continue
			}
			fails += checkUses(name, uses, line, stderr)
			if uses.Kind != yaml.ScalarNode || !strings.HasPrefix(uses.Value, "goreleaser/goreleaser-action@") {
				continue
			}
			with, _, hasWith := mappingValue(step, "with")
			if hasWith && with.Kind != yaml.MappingNode {
				fmt.Fprintf(stderr, "pincheck: %s:%d: malformed with must be a mapping\n", name, with.Line)
				fails++
				continue
			}
			version, versionLine, hasVersion := mappingValue(with, "version")
			if !hasVersion || version.Kind != yaml.ScalarNode {
				fmt.Fprintf(stderr, "pincheck: %s:%d: goreleaser-action step has no version: input or it is not scalar; the tool would float to latest\n", name, line)
				fails++
				continue
			}
			if !exactSemver.MatchString(version.Value) {
				fmt.Fprintf(stderr, "pincheck: %s:%d: goreleaser version must be an exact vX.Y.Z, got: %s\n", name, versionLine, version.Value)
				fails++
			}
		}
	}
	return fails
}

func checkUses(name string, uses *yaml.Node, line int, stderr io.Writer) int {
	if uses.Kind != yaml.ScalarNode {
		fmt.Fprintf(stderr, "pincheck: %s:%d: uses must be a scalar\n", name, line)
		return 1
	}
	if bad := usesViolation(uses.Value); bad != "" {
		fmt.Fprintf(stderr, "pincheck: %s:%d: %s: %s\n", name, line, bad, uses.Value)
		return 1
	}
	return 0
}

func uniqueMappingKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			for j := i + 2; j+1 < len(node.Content); j += 2 {
				if node.Content[i].Value == node.Content[j].Value {
					return fmt.Errorf("duplicate key %q at line %d", node.Content[i].Value, node.Content[j].Line)
				}
			}
		}
	}
	for _, child := range node.Content {
		if err := uniqueMappingKeys(child); err != nil {
			return err
		}
	}
	return nil
}
func hasAliasOrMerge(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" || node.Tag == "!!merge" {
		return true
	}
	for _, child := range node.Content {
		if hasAliasOrMerge(child) {
			return true
		}
	}
	return false
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
