package plan

import (
	"strings"
	"testing"
)

const validTerminalRange = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa..bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func validTerminalNotes(paths string) string {
	return "### Terminal reconciliation\nImplementation range: " + validTerminalRange + "\nTouched paths:\n" + paths + "Material deviations:\n- none\n"
}

func TestParseTerminalReconciliation(t *testing.T) {
	parsed, err := ParseTerminalReconciliation(validTerminalNotes("- \"ordinary.go\"\n- \"leading space \"\n- \"line\\nbreak`\\xff\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ordinary.go", "leading space ", "line\nbreak`\xff"}
	if !slicesEqual(parsed.TouchedPaths, want) {
		t.Fatalf("paths = %#v, want %#v", parsed.TouchedPaths, want)
	}
	if got := parsed.MaterialDeviations; !slicesEqual(got, []string{"none"}) {
		t.Fatalf("deviations = %#v", got)
	}
}

func TestParseTerminalReconciliationRejectsMalformedBranches(t *testing.T) {
	cases := map[string]string{
		"duplicate heading":           "### Terminal reconciliation\nImplementation range: " + validTerminalRange + "\nTouched paths:\n- \"a\"\nMaterial deviations:\n- none\n### Terminal reconciliation\n",
		"missing fields":              "### Terminal reconciliation\nImplementation range: " + validTerminalRange + "\n",
		"symbolic range":              strings.Replace(validTerminalNotes("- \"a\"\n"), validTerminalRange, "main..HEAD", 1),
		"uppercase range":             strings.Replace(validTerminalNotes("- \"a\"\n"), validTerminalRange, strings.ToUpper(validTerminalRange), 1),
		"empty touched paths":         "### Terminal reconciliation\nImplementation range: " + validTerminalRange + "\nTouched paths:\nMaterial deviations:\n- none\n",
		"unquoted path":               validTerminalNotes("- a\n"),
		"noncanonical path":           validTerminalNotes("- \"\\x61\"\n"),
		"duplicate path":              validTerminalNotes("- \"a\"\n- \"a\"\n"),
		"empty path":                  validTerminalNotes("- \"\"\n"),
		"missing material deviations": "### Terminal reconciliation\nImplementation range: " + validTerminalRange + "\nTouched paths:\n- \"a\"\n",
		"empty material deviations":   validTerminalNotes("- \"a\"\n")[:len(validTerminalNotes("- \"a\"\n"))-7],
		"mixed none":                  strings.Replace(validTerminalNotes("- \"a\"\n"), "- none\n", "- none\n- actual deviation\n", 1),
		"trailing content":            validTerminalNotes("- \"a\"\n") + "\nnot allowed\n",
	}
	for name, notes := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTerminalReconciliation(notes); err == nil {
				t.Fatal("malformed reconciliation accepted")
			}
		})
	}
}

func TestParseTerminalReconciliationIgnoresFencedExamples(t *testing.T) {
	notes := "```markdown\n### Terminal reconciliation\nImplementation range: " + validTerminalRange + "\nTouched paths:\n- \"fake\"\nMaterial deviations:\n- none\n```\n"
	parsed, err := ParseTerminalReconciliation(notes)
	if err != nil || parsed != nil {
		t.Fatalf("fenced example = %#v, %v", parsed, err)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
