package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJSON writes body to a temp file and returns its path.
func writeJSON(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mutants"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for missing arg, got %d", code)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("missing usage text: %q", errb.String())
	}
}

// ./x mutants pre-creates the report via mktemp, so a nonexistent path is a
// caller error (typo, changed script) - it must fail loudly, not read as a
// clean run. Only a present-but-empty file means "nothing to report".
// invariant: tooling/audit-commands:mutants-missing-report-errors (TestRunMissingFileErrors)
func TestRunMissingFileErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", filepath.Join(t.TempDir(), "nope.json")}, &out, &errb); code == 0 {
		t.Fatalf("expected non-zero exit for missing report file, got 0 (%s)", out.String())
	}
	if strings.Contains(out.String(), "no survived mutants") {
		t.Errorf("missing file must not read as a clean run: %q", out.String())
	}
}

func TestRunEmptyFile(t *testing.T) {
	for _, body := range []string{"", "   \n"} {
		var out, errb bytes.Buffer
		if code := run([]string{"mutants", writeJSON(t, body)}, &out, &errb); code != 0 {
			t.Fatalf("body %q: expected exit 0, got %d", body, code)
		}
		if !strings.Contains(out.String(), "no survived mutants") {
			t.Errorf("body %q: expected empty-run message, got %q", body, out.String())
		}
	}
}

func TestRunReadError(t *testing.T) {
	// A directory path is not IsNotExist but os.ReadFile still errors.
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", t.TempDir()}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on read error, got %d", code)
	}
	if !strings.Contains(errb.String(), "mutants:") {
		t.Errorf("expected error prefix, got %q", errb.String())
	}
}

func TestRunMalformed(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", writeJSON(t, "{not json")}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on malformed json, got %d", code)
	}
	if !strings.Contains(errb.String(), "parsing") {
		t.Errorf("expected parse error, got %q", errb.String())
	}
}

// invariant: tooling/quality-gates:mutants-timeout-untrusted (TestRunTimedOutIsUntrusted)
func TestRunTimedOutIsUntrusted(t *testing.T) {
	// A LIVED survivor is present, but the timeout makes the whole run untrustworthy.
	const j = `{"files":[{"file_name":"refs.go","mutations":[
		{"type":"CONDITIONALS_BOUNDARY","status":"LIVED","line":85},
		{"type":"ARITHMETIC_BASE","status":"TIMED OUT","line":92}]}]}`
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 when a mutant timed out, got %d", code)
	}
	if !strings.Contains(errb.String(), "timed out") {
		t.Errorf("expected timeout message, got %q", errb.String())
	}
}

func TestRunReportsOnlyLived(t *testing.T) {
	const j = `{"files":[{"file_name":"refs.go","mutations":[
		{"type":"ARITHMETIC_BASE","status":"KILLED","line":63},
		{"type":"CONDITIONALS_NEGATION","status":"NOT COVERED","line":60},
		{"type":"CONDITIONALS_BOUNDARY","status":"NOT VIABLE","line":70},
		{"type":"ARITHMETIC_BASE","status":"LIVED","line":92}]}]}`
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
	o := out.String()
	if !strings.Contains(o, "refs.go:92  ARITHMETIC_BASE") {
		t.Errorf("LIVED mutant not reported: %q", o)
	}
	if strings.Contains(o, "NOT COVERED") || strings.Contains(o, ":60") || strings.Contains(o, ":70") {
		t.Errorf("only LIVED should be reported, got %q", o)
	}
}

func TestRunNoSurvivors(t *testing.T) {
	const j = `{"files":[{"file_name":"refs.go","mutations":[
		{"type":"ARITHMETIC_BASE","status":"KILLED","line":63},
		{"type":"CONDITIONALS_NEGATION","status":"NOT COVERED","line":60}]}]}`
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "no survived mutants") {
		t.Errorf("expected no-survivors message, got %q", out.String())
	}
}

func completeReport(statuses ...string) []byte {
	mutations := make([]string, 0, len(statuses))
	killed, lived, notViable, notCovered := 0, 0, 0, 0
	for i, status := range statuses {
		mutations = append(mutations, fmt.Sprintf(`{"type":"ARITHMETIC_BASE","status":%q,"line":%d,"column":13}`, status, i+1))
		switch status {
		case "KILLED":
			killed++
		case "LIVED":
			lived++
		case "NOT VIABLE":
			notViable++
		case "NOT COVERED":
			notCovered++
		}
	}
	total := killed + lived + notViable
	return []byte(fmt.Sprintf(`{"go_module":"github.com/hypnotox/agentic-workflows","files":[{"file_name":"cmd/covercheck/main.go","mutations":[%s]}],"test_efficacy":100,"mutations_coverage":100,"mutants_total":%d,"mutants_killed":%d,"mutants_lived":%d,"mutants_not_viable":%d,"mutants_not_covered":%d,"elapsed_time":12,"mutator_statistics":{"arithmetic_base":%d}}`, strings.Join(mutations, ","), total, killed, lived, notViable, notCovered, len(statuses)))
}

func TestBlockingReportTrustContract(t *testing.T) {
	dry := completeReport("RUNNABLE", "RUNNABLE")
	actual := completeReport("KILLED", "KILLED")
	if _, err := validateDryActual(dry, actual, nil, "cmd/covercheck"); err != nil {
		t.Fatalf("complete reports: %v", err)
	}
	equivalent := mutationIdentity{File: "cmd/covercheck/main.go", Line: 1, Column: 13, Mutator: "ARITHMETIC_BASE"}
	if _, err := validateDryActual(dry, completeReport("LIVED", "KILLED"), map[mutationIdentity]struct{}{equivalent: {}}, "cmd/covercheck"); err != nil {
		t.Fatalf("equivalent LIVED: %v", err)
	}

	cases := map[string][]byte{
		"duplicate": []byte(strings.Replace(string(actual), `"line":2`, `"line":1`, 1)),
		"missing":   completeReport("KILLED"), "extra": completeReport("KILLED", "KILLED", "KILLED"),
		"empty": {}, "malformed": []byte(`{`), "incomplete": []byte(`{"files":[]}`),
		"not covered": completeReport("NOT COVERED", "KILLED"), "not viable": completeReport("NOT VIABLE", "KILLED"),
		"skipped": completeReport("SKIPPED", "KILLED"), "timed out": completeReport("TIMED OUT", "KILLED"),
		"unknown":       completeReport("UNKNOWN", "KILLED"),
		"over budget":   []byte(strings.Replace(string(actual), `"elapsed_time":12`, `"elapsed_time":901`, 1)),
		"absolute file": []byte(strings.Replace(string(actual), `"cmd/covercheck/main.go`, `/cmd/covercheck/main.go`, 1)),
	}

	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := validateDryActual(dry, report, nil, "cmd/covercheck"); err == nil {
				t.Fatal("untrusted report accepted")
			}
		})
	}
}

func TestBlockingReportAcceptsGremlinsDryTotalsAndNormalizesTargetRoot(t *testing.T) {
	dry := []byte(`{"go_module":"github.com/hypnotox/agentic-workflows","files":[{"file_name":"main.go","mutations":[{"type":"CONDITIONALS_NEGATION","status":"RUNNABLE","line":20,"column":15}]}],"test_efficacy":0,"mutations_coverage":100,"mutants_total":0,"mutants_killed":0,"mutants_lived":0,"mutants_not_viable":0,"mutants_not_covered":0,"elapsed_time":1.2,"mutator_statistics":{"conditionals_negation":1}}`)
	actual := []byte(`{"go_module":"github.com/hypnotox/agentic-workflows","files":[{"file_name":"main.go","mutations":[{"type":"CONDITIONALS_NEGATION","status":"KILLED","line":20,"column":15}]}],"test_efficacy":100,"mutations_coverage":100,"mutants_total":1,"mutants_killed":1,"mutants_lived":0,"mutants_not_viable":0,"mutants_not_covered":0,"elapsed_time":2.3,"mutator_statistics":{"conditionals_negation":1}}`)
	report, err := validateDryActual(dry, actual, nil, "cmd/covercheck")
	if err != nil {
		t.Fatalf("real Gremlins shape: %v", err)
	}
	identity := mutationIdentity{File: "cmd/covercheck/main.go", Line: 20, Column: 15, Mutator: "CONDITIONALS_NEGATION"}
	if got := report.statuses[identity]; got != "KILLED" {
		t.Fatalf("normalized status = %q, want KILLED; all=%#v", got, report.statuses)
	}
}

func TestMutationOperatorValues(t *testing.T) {
	want := map[string]bool{"arithmetic-base": true, "conditionals-boundary": true, "conditionals-negation": true, "increment-decrement": true, "invert-negatives": true, "invert-assignments": false, "invert-bitwise": false, "invert-bwassign": false, "invert-logical": false, "invert-loopctrl": false, "remove-self-assignments": false}
	if got := mutationOperatorValues(); len(got) != len(want) {
		t.Fatalf("operator count = %d, want %d", len(got), len(want))
	} else {
		for name, enabled := range want {
			if got[name] != enabled {
				t.Errorf("%s = %t, want %t", name, got[name], enabled)
			}
		}
	}
}

func TestRunOperators(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", "operators"}, &out, &errb); code != 0 {
		t.Fatalf("operators = %d: %s", code, errb.String())
	}
	if got := strings.Count(strings.TrimSpace(out.String()), "\n") + 1; got != 11 {
		t.Errorf("operator lines = %d, want 11: %q", got, out.String())
	}
}

func TestRenewalRequiresThreeMatchingTrustedRunsWithinBudget(t *testing.T) {
	report, err := validateActual(completeReport("KILLED", "KILLED"), nil, "cmd/covercheck")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRenewal([]timedReport{{report, 500}, {report, 500}, {report, 500}}); err != nil {
		t.Fatalf("valid renewal: %v", err)
	}
	cases := map[string][]timedReport{
		"not three": {{report, 1}, {report, 1}}, "over run": {{report, 901}, {report, 1}, {report, 1}},
		"over total":         {{report, 501}, {report, 501}, {report, 501}},
		"different statuses": {{report, 1}, {report, 1}, {trustedReport{statuses: map[mutationIdentity]string{{File: "cmd/covercheck/main.go", Line: 1, Column: 13, Mutator: "ARITHMETIC_BASE"}: "LIVED"}}, 1}},
	}
	for name, runs := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validateRenewal(runs); err == nil {
				t.Fatal("invalid renewal accepted")
			}
		})
	}
}

func TestRunRenewal(t *testing.T) {
	baseline := writeJSON(t, `{"equivalentMutants":[]}`)
	actual := writeJSON(t, string(completeReport("KILLED", "KILLED")))
	args := []string{"mutants", "renewal", baseline, "cmd/covercheck", "500", actual, "500", actual, "500", actual}
	var out, errb bytes.Buffer
	if code := run(args, &out, &errb); code != 0 {
		t.Fatalf("renewal = %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "trusted mutation renewal") {
		t.Errorf("output = %q", out.String())
	}
}

func TestRunRenewalRejectsInvalidInputs(t *testing.T) {
	baseline := writeJSON(t, `{"equivalentMutants":[]}`)
	invalidBaseline := writeJSON(t, `{}`)
	actual := writeJSON(t, string(completeReport("KILLED")))
	invalidActual := writeJSON(t, `{}`)
	missing := filepath.Join(t.TempDir(), "missing.json")
	valid := []string{"mutants", "renewal", baseline, "cmd/covercheck", "1", actual, "1", actual, "1", actual}
	cases := map[string][]string{
		"wrong arity":      {"mutants", "renewal"},
		"missing baseline": append([]string(nil), valid...),
		"invalid baseline": append([]string(nil), valid...),
		"invalid elapsed":  append([]string(nil), valid...),
		"missing report":   append([]string(nil), valid...),
		"untrusted report": append([]string(nil), valid...),
		"over budget":      append([]string(nil), valid...),
	}
	cases["missing baseline"][2] = missing
	cases["invalid baseline"][2] = invalidBaseline
	cases["invalid elapsed"][4] = "bad"
	cases["missing report"][5] = missing
	cases["untrusted report"][5] = invalidActual
	cases["over budget"][4], cases["over budget"][6], cases["over budget"][8] = "501", "501", "501"
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(args, &out, &errb); code != 1 && name != "wrong arity" {
				t.Fatalf("code = %d, want 1: %s", code, errb.String())
			} else if name == "wrong arity" && code != 2 {
				t.Fatalf("code = %d, want 2: %s", code, errb.String())
			}
		})
	}
}

func TestRunValidate(t *testing.T) {
	dry, actual := writeJSON(t, string(completeReport("RUNNABLE", "RUNNABLE"))), writeJSON(t, string(completeReport("KILLED", "KILLED")))
	baseline := writeJSON(t, `{"equivalentMutants":[]}`)
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", "validate", dry, actual, baseline, "cmd/covercheck"}, &out, &errb); code != 0 {
		t.Fatalf("validate = %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "trusted mutation reports") {
		t.Errorf("output = %q", out.String())
	}
}

func TestRunValidateRejectsUnreadableInputsAndInvalidBaseline(t *testing.T) {
	dry := writeJSON(t, string(completeReport("RUNNABLE")))
	actual := writeJSON(t, string(completeReport("KILLED")))
	baseline := writeJSON(t, `{"equivalentMutants":[]}`)
	missing := filepath.Join(t.TempDir(), "missing.json")
	for name, args := range map[string][]string{
		"wrong arity":  {"mutants", "validate"},
		"dry":          {"mutants", "validate", missing, actual, baseline, "cmd/covercheck"},
		"actual":       {"mutants", "validate", dry, missing, baseline, "cmd/covercheck"},
		"baseline":     {"mutants", "validate", dry, actual, missing, "cmd/covercheck"},
		"bad baseline": {"mutants", "validate", dry, actual, writeJSON(t, `{}`), "cmd/covercheck"},
	} {
		t.Run(name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(args, &out, &errb); code == 0 {
				t.Fatalf("invalid validate invocation succeeded: out=%q err=%q", out.String(), errb.String())
			}
		})
	}
}

func TestRunOperatorsRejectsArguments(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", "operators", "extra"}, &out, &errb); code != 2 {
		t.Fatalf("operators extra argument = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("missing usage: %q", errb.String())
	}
}

func TestTrustContractRejectsEveryMalformedBoundary(t *testing.T) {
	validDry := completeReport("RUNNABLE")
	validActual := completeReport("KILLED")
	assertRejected := func(t *testing.T, dry, actual []byte, root string) {
		t.Helper()
		if _, err := validateDryActual(dry, actual, nil, root); err == nil {
			t.Fatal("untrusted report accepted")
		}
	}
	for name, actual := range map[string][]byte{
		"invalid statistics":  []byte(strings.Replace(string(validActual), `{"arithmetic_base":1}`, `[]`, 1)),
		"empty module":        []byte(strings.Replace(string(validActual), `"go_module":"github.com/hypnotox/agentic-workflows"`, `"go_module":""`, 1)),
		"empty mutations":     []byte(strings.Replace(string(validActual), `"mutations":[{"type":"ARITHMETIC_BASE","status":"KILLED","line":1,"column":13}]`, `"mutations":[]`, 1)),
		"missing identity":    []byte(strings.Replace(string(validActual), `"column":13`, `"column":0`, 1)),
		"actual runnable":     completeReport("RUNNABLE"),
		"bad totals":          []byte(strings.Replace(string(validActual), `"mutants_total":1`, `"mutants_total":2`, 1)),
		"negative statistic":  []byte(strings.Replace(string(validActual), `{"arithmetic_base":1}`, `{"arithmetic_base":-1}`, 1)),
		"statistics mismatch": []byte(strings.Replace(string(validActual), `{"arithmetic_base":1}`, `{"arithmetic_base":2}`, 1)),
		"outside root":        []byte(strings.Replace(string(validActual), `cmd/covercheck/main.go`, `cmd/other/main.go`, 1)),
	} {
		t.Run(name, func(t *testing.T) { assertRejected(t, validDry, actual, "cmd/covercheck") })
	}
	for name, root := range map[string]string{"dot": ".", "absolute": "/cmd/covercheck", "parent": "../cmd/covercheck"} {
		t.Run("invalid root "+name, func(t *testing.T) { assertRejected(t, validDry, validActual, root) })
	}
	assertRejected(t, completeReport("KILLED"), validActual, "cmd/covercheck")
	otherDry := []byte(strings.Replace(string(validDry), `"line":1`, `"line":2`, 1))
	assertRejected(t, otherDry, validActual, "cmd/covercheck")
}

func TestEquivalentMutantsRejectsMalformedAndNormalizesValidEntries(t *testing.T) {
	for name, data := range map[string][]byte{
		"malformed": []byte(`{`), "missing": []byte(`{}`),
		"incomplete": []byte(`{"equivalentMutants":[{"file":"cmd/covercheck/main.go","line":1,"column":13,"mutator":"ARITHMETIC_BASE","reason":" "}]}`),
		"duplicate":  []byte(`{"equivalentMutants":[{"file":"cmd/covercheck/main.go","line":1,"column":13,"mutator":"ARITHMETIC_BASE","reason":"equivalent"},{"file":"cmd/covercheck/main.go","line":1,"column":13,"mutator":"ARITHMETIC_BASE","reason":"also equivalent"}]}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := equivalentMutants(data); err == nil {
				t.Fatal("invalid equivalent set accepted")
			}
		})
	}
	identity := mutationIdentity{File: "cmd/covercheck/main.go", Line: 1, Column: 13, Mutator: "ARITHMETIC_BASE"}
	got, err := equivalentMutants([]byte(`{"equivalentMutants":[{"file":"cmd/covercheck/main.go","line":1,"column":13,"mutator":"ARITHMETIC_BASE","reason":"equivalent expression"}]}`))
	if err != nil || len(got) != 1 {
		t.Fatalf("valid equivalents = %#v, %v", got, err)
	}
	if _, ok := got[identity]; !ok {
		t.Fatalf("missing identity: %#v", got)
	}
}

func TestSetComparisonsRejectEqualLengthDifferences(t *testing.T) {
	identity := mutationIdentity{File: "cmd/covercheck/main.go", Line: 1, Column: 13, Mutator: "ARITHMETIC_BASE"}
	left := map[mutationIdentity]string{identity: "KILLED"}
	right := map[mutationIdentity]string{{File: identity.File, Line: 2, Column: 13, Mutator: identity.Mutator}: "LIVED"}
	if sameIdentitySet(left, right) {
		t.Fatal("different identities accepted")
	}
	if sameStatusSet(left, map[mutationIdentity]string{identity: "LIVED"}) {
		t.Fatal("different statuses accepted")
	}
}

func TestRunValidateRejectsUntrustedReport(t *testing.T) {
	dry := writeJSON(t, string(completeReport("RUNNABLE")))
	actual := writeJSON(t, string(completeReport("LIVED")))
	baseline := writeJSON(t, `{"equivalentMutants":[]}`)
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", "validate", dry, actual, baseline, "cmd/covercheck"}, &out, &errb); code != 1 {
		t.Fatalf("untrusted report exit = %d, want 1; out=%q err=%q", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "untrusted report") {
		t.Errorf("missing trust failure: %q", errb.String())
	}
}

func TestParserRejectsTypeMismatchesAndUnreviewedLivedMutation(t *testing.T) {
	if _, err := validateActual([]byte(`{"files":1}`), nil, "cmd/covercheck"); err == nil {
		t.Fatal("wrong report field type accepted")
	}
	if _, err := validateActual(completeReport("LIVED"), nil, "cmd/covercheck"); err == nil {
		t.Fatal("unreviewed LIVED mutation accepted")
	}
	if _, err := mutationFileIdentity("/cmd/covercheck/main.go", "cmd/covercheck"); err == nil {
		t.Fatal("absolute mutation file accepted")
	}
}

func TestEquivalentMutantsRejectsWrongFieldType(t *testing.T) {
	if _, err := equivalentMutants([]byte(`{"equivalentMutants":1}`)); err == nil {
		t.Fatal("wrong equivalentMutants type accepted")
	}
}
