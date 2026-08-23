package project

import (
	"strings"
	"testing"
)

func TestBugfixTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd":     "./x gate",
			"gateCmdFull": "./x gate full",
		},
		"data":   map[string]any{},
		"skills": map[string]bool{"tdd": true, "debugging": true, "reviewing-impl": true},
	}

	out := renderSkillGolden(t, "bugfix", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-bugfix") {
		t.Errorf("expected 'name: example-bugfix' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to bugfix
	loadBearing := []string{
		"regression test",
		"root-cause fix",
		"example-reviewing-impl",
		"example-tdd",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestTddTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"testCmd": "go test ./...",
			"gateCmd": "./x gate",
		},
		"data":   map[string]any{},
		"skills": map[string]bool{},
	}

	out := renderSkillGolden(t, "tdd", data)

	if !strings.Contains(out, "name: example-tdd") {
		t.Errorf("expected 'name: example-tdd' in output:\n%s", out)
	}

	loadBearing := []string{
		"strongest practical durable oracle",
		"confirm it fails for the right reason: `go test ./...`",
		"record the concrete reason automated red-first is impractical",
		"Run the gate: `./x gate`",
		"a test never observed failing proves nothing.",
		"Fix the code, not the oracle.",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestDebuggingTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd":     "./x gate",
			"gateCmdFull": "./x gate full",
		},
		"data":   map[string]any{},
		"skills": map[string]bool{"tdd": true, "bugfix": true, "brainstorming": true, "exploring": true},
	}

	out := renderSkillGolden(t, "debugging", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-debugging") {
		t.Errorf("expected 'name: example-debugging' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to debugging
	loadBearing := []string{
		"falsifiable hypothesis",
		"unfixed behaviour",
		"root cause",
		"strongest practical durable oracle",
		"example-bugfix",
		"example-brainstorming",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	ordered := []string{
		"**Form one falsifiable hypothesis.**",
		"Invoke `example-exploring`",
		"Pick the cheapest oracle",
		"**Establish the strongest practical durable oracle.**",
	}
	position := -1
	for _, phrase := range ordered {
		next := strings.Index(out, phrase)
		if next <= position {
			t.Fatalf("debugging order violation at %q: positions must increase in %v", phrase, ordered)
		}
		position = next
	}
}

func TestExploringTemplate(t *testing.T) {
	pi := renderSkillGolden(t, "exploring", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
		"skills": map[string]bool{}, "targetSubagentTools": true,
	})
	fallback := renderSkillGolden(t, "exploring", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{},
	})
	for label, body := range map[string]string{"pi": pi, "fallback": fallback} {
		for _, want := range []string{
			"location is unknown and inline search would pollute the parent context",
			"exact-known-file", "genuinely trivial",
			"one self-contained task per child", "fan them out as sibling calls",
			"breadth, detail, and tier independently per child", "large analysis child with small targeted paths or summary children",
			"refinements that depend on an earlier result stay sequential",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s exploring render missing %q:\n%s", label, want, body)
			}
		}
	}
	for _, want := range []string{"subagent_explore", "required task, breadth, and detail"} {
		if !strings.Contains(pi, want) {
			t.Errorf("Pi exploring render missing %q:\n%s", want, pi)
		}
	}
	if !strings.Contains(fallback, "target-native fresh-context exploration subagent") || strings.Contains(fallback, "subagent_explore") {
		t.Errorf("fallback exploring dispatch is not generic:\n%s", fallback)
	}
}
