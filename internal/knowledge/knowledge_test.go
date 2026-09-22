package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConceptContract(t *testing.T) {
	for _, test := range []struct {
		name, document, want string
	}{
		{"minimal", "---\ntype: Custom Kind\ndescription: A useful summary.\n---\n", ""},
		{"optional fields opaque", "---\ntype: Attested Computation\ndescription: Summary\nstatus: unusual\nverified: false\nstale_after: someday\nsources: anything\nextra: {nested: [1, true]}\n---\n[Missing](/missing.md)\n", ""},
		{"CRLF", "---\r\ntype: Custom\r\ndescription: Summary\r\n---\r\nBody\r\n", ""},
		{"multiline description", "---\ntype: Custom\ndescription: >\n  A useful\n  summary.\n---\n", ""},
		{"no frontmatter", "# Heading\n", "leading YAML"},
		{"unterminated frontmatter", "---\ntype: Custom\n", "leading YAML"},
		{"invalid YAML", "---\ntype: [broken\n---\n", "parse frontmatter"},
		{"sequence not mapping", "---\n- type\n---\n", "parse frontmatter"},
		{"duplicate key", "---\ntype: Custom\ntype: Other\ndescription: Summary\n---\n", "parse frontmatter"},
		{"missing type", "---\ndescription: Summary\n---\n", "OKF type"},
		{"missing description", "---\ntype: Custom\n---\n", "AWF description"},
		{"invalid body UTF8", "---\ntype: Custom\ndescription: Summary\n---\n\xff", "UTF-8"},
	} {
		t.Run(test.name, func(t *testing.T) {
			messages := validate("docs/nested/concept.md", []byte(test.document))
			if test.want == "" && len(messages) != 0 {
				t.Fatalf("valid concept rejected: %v", messages)
			}
			if test.want != "" && !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("findings = %v, want %q", messages, test.want)
			}
		})
	}
	for _, field := range []string{"type", "description"} {
		for _, invalid := range []string{"null", "''", "'  '", "42", "true", "[]", "{}", "2026-01-01"} {
			t.Run(field+"="+invalid, func(t *testing.T) {
				header := map[string]string{"type": "Custom", "description": "Summary"}
				header[field] = invalid
				content := "---\ntype: " + header["type"] + "\ndescription: " + header["description"] + "\n---\n"
				if got := strings.Join(validate("docs/concept.md", []byte(content)), "\n"); !strings.Contains(got, field+" must be a non-empty string") {
					t.Fatalf("invalid %s accepted: %s", field, got)
				}
			})
		}
	}
}

func TestADRStatePairsAreLocationBoundAndExplicit(t *testing.T) {
	for _, decision := range []string{"pending", "accepted", "active", "retired", "", "42"} {
		for _, status := range []string{"draft", "stable", "deprecated", "pending", "", "42"} {
			t.Run(decision+"/"+status, func(t *testing.T) {
				content := "---\ntype: Architecture Decision\ndescription: Summary\n"
				if decision != "" {
					content += "decision_status: " + decision + "\n"
				}
				if status != "" {
					content += "status: " + status + "\n"
				}
				content += "---\nBody\n"
				valid := (decision == "pending" || decision == "accepted") && status == "draft" || decision == "active" && status == "stable"
				messages := validate("docs/decisions/nested/record.md", []byte(content))
				if (len(messages) == 0) != valid {
					t.Fatalf("valid=%v; findings: %v", valid, messages)
				}
				custom := strings.Replace(content, "Architecture Decision", "Custom Decision", 1)
				if messages := validate("docs/decisions/custom.md", []byte(custom)); (len(messages) == 0) != valid {
					t.Fatalf("custom type changed location-bound ADR checking: %v", messages)
				}
				if messages := validate("docs/other/record.md", []byte(content)); len(messages) != 0 {
					t.Fatalf("ADR state machine escaped docs/decisions: %v", messages)
				}
			})
		}
	}
}

func TestReservedStructures(t *testing.T) {
	for _, test := range []struct {
		name, path, content, want string
	}{
		{"index", "docs/index.md", "# Contents\n\n* [Missing](missing.md)\n", ""},
		{"empty index listing", "docs/index.md", "# Contents\n", ""},
		{"empty log history", "docs/log.md", "# Update log\n", ""},
		{"nested index", "docs/topics/nested/index.md", "# Topics\n\n- [Topic](topic.md) - summary\n", ""},
		{"case insensitive", "docs/topics/INDEX.MD", "# Topics\n", ""},
		{"root version", "docs/index.md", "---\nokf_version: '0.2'\n---\n# Contents\n", ""},
		{"future version best effort", "docs/index.md", "---\nokf_version: '1.0'\n---\n# Contents\n", ""},
		{"setext and reference links", "docs/index.md", "Contents\n========\n\n1. [First][ref]\n2. [Second]\n\n[ref]: missing.md\n[Second]: other.md\n", ""},
		{"fenced examples", "docs/index.md", "# Contents\n\n~~~md\n- not an entry\n~~~\n\n> - not an entry either\n", ""},
		{"root concept metadata", "docs/index.md", "---\ntype: Project Topic\ndescription: Not an index\n---\n# Contents\n", "only a non-empty string okf_version"},
		{"numeric version", "docs/index.md", "---\nokf_version: 0.2\n---\n# Contents\n", "only a non-empty string okf_version"},
		{"version plus extra metadata", "docs/index.md", "---\nokf_version: '0.2'\nextra: true\n---\n# Contents\n", "only a non-empty string okf_version"},
		{"nested version", "docs/topics/index.md", "---\nokf_version: '0.2'\n---\n# Contents\n", "must not contain frontmatter"},
		{"unclosed frontmatter", "docs/index.md", "---\nokf_version: '0.2'\n# Contents\n", "unterminated"},
		{"no index heading", "docs/index.md", "- [Topic](topic.md)\n", "grouped under a heading"},
		{"no index link", "docs/index.md", "# Contents\n\n- Topic\n", "must contain Markdown links"},
		{"empty index", "docs/index.md", "", "requires a heading"},
		{"log", "docs/log.md", "# Update log\n\n## 2026-06-20\n- Added a topic\n\n## 2026-01-01\n1. Initialized\n", ""},
		{"log without title", "docs/log.md", "## 2026-06-20\n- Added a topic\n", ""},
		{"log closing hashes", "docs/log.md", "# Updates #\n\n## 2026-06-20 ##\n- Added a topic\n", ""},
		{"log alternate headings", "docs/log.md", "Update log\n==========\n\n2026-06-20\n----------\n\n+ Added a topic\n", ""},
		{"log examples", "docs/log.md", "# Update log\n\n```md\n## not a date\n- example\n```\n\n## 2026-01-01\n- Added example\n", ""},
		{"log frontmatter", "docs/log.md", "---\ntype: Log\n---\n# Log\n", "must not contain frontmatter"},
		{"log malformed date", "docs/log.md", "# Log\n\n## Yesterday\n- Added a topic\n", "YYYY-MM-DD"},
		{"log impossible date", "docs/log.md", "# Log\n\n## 2026-02-30\n- Added a topic\n", "YYYY-MM-DD"},
		{"log missing date", "docs/log.md", "# Log\n\n- Added a topic\n", "must follow"},
		{"log ascending dates", "docs/log.md", "# Log\n\n## 2026-01-01\n- First\n\n## 2026-01-02\n- Second\n", "newest first"},
		{"log nested entries", "docs/log.md", "# Log\n\n## 2026-01-01\n- Entry\n  - nested entry\n", "flat list"},
		{"reserved invalid UTF8", "docs/log.md", "# Log\n\xff", "UTF-8"},
	} {
		t.Run(test.name, func(t *testing.T) {
			messages := validate(test.path, []byte(test.content))
			if test.want == "" && len(messages) != 0 {
				t.Fatalf("valid reserved file rejected: %v", messages)
			}
			if test.want != "" && !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("findings = %v, want %q", messages, test.want)
			}
		})
	}
}

func TestCheckWalksWholeBundleReadOnly(t *testing.T) {
	root := t.TempDir()
	if findings, err := Check(root); err != nil || len(findings) != 0 {
		t.Fatalf("absent bundle: %v, %v", findings, err)
	}
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if findings, err := Check(root); err != nil || len(findings) != 0 {
		t.Fatalf("empty bundle: %v, %v", findings, err)
	}
	files := map[string]string{
		"README.md":                       "outside bundle\xff",
		"AGENTS.md":                       "outside bundle",
		"internal/docs/guide.md":          "outside bundle",
		".awf/efforts/work/memory.md":     "outside bundle",
		".pi/skills/test/SKILL.md":        "outside bundle",
		"docs/manual/nested/arbitrary.MD": "---\ntype: A custom type\ndescription: A summary\n---\n",
		"docs/.hidden/concept.md":         "no frontmatter",
		"docs/topics/index.md":            "# Contents\n",
		"docs/decisions/log.md":           "# Updates\n",
		"docs/ordinary.md":                "---\ntype: Reference\n---\n",
		"docs/blob.txt":                   "not markdown\xff",
	}
	for name, content := range files {
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	findings, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, finding := range findings {
		paths = append(paths, finding.Path)
	}
	if !reflect.DeepEqual(paths, []string{"docs/.hidden/concept.md", "docs/ordinary.md"}) {
		t.Fatalf("findings = %v", findings)
	}
	for name, original := range files {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(content) != original {
			t.Fatalf("check changed %s: %q, %v", name, content, err)
		}
	}
}

func TestCheckDoesNotFollowMarkdownSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "docs", "link.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	findings, err := Check(root)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].Message, "not a regular file") {
		t.Fatalf("symlink findings = %v, %v", findings, err)
	}
}
