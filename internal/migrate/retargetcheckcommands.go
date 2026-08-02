package migrate

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

const retargetCheckCommandsGeneration = 32

var checkCommandRe = regexp.MustCompile(`^(\S+)(\s+)(check)(\s+)(prose|memory|commit|invariants)(\s|$)`)

var retiredCheckCommandVars = []string{"proseGateCmd", "memoryGateCmd"}

func rewriteCheckCommand(value string) (string, bool, bool) {
	m := checkCommandRe.FindStringSubmatch(value)
	if m == nil || !awfInvocation(m[1]) {
		return value, false, false
	}
	if m[5] == "invariants" {
		return value, false, true
	}
	universe := "repo"
	if m[5] == "commit" {
		universe = "staged"
	}
	prefix := m[1] + m[2] + m[3] + m[4] + m[5]
	return m[1] + m[2] + m[3] + m[4] + universe + m[4] + m[5] + value[len(prefix):], true, false
}

func retargetCheckCommandBytes(src []byte) ([]byte, []string, error) {
	var parsed struct {
		Vars map[string]any `yaml:"vars"`
	}
	parsedOK := yaml.Unmarshal(src, &parsed) == nil
	if !parsedOK {
		return src, nil, nil
	}

	out := src
	var changes []string
	for _, key := range retiredCheckCommandVars {
		edited, err := config.RemoveMappingKey(out, "vars", key)
		if err != nil { // coverage-ignore: src parsed as a mapping above, and each prior edit returned validated YAML
			return nil, nil, err
		}
		if !bytes.Equal(edited, out) {
			changes = append(changes, "removed retired vars."+key)
			out = edited
		}
	}

	parsed.Vars = nil
	if err := yaml.Unmarshal(out, &parsed); err != nil { // coverage-ignore: the same valid YAML was changed only by config's validated node editor
		return nil, nil, err
	}
	keys := make([]string, 0, len(parsed.Vars))
	for key := range parsed.Vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, ok := parsed.Vars[key].(string)
		if !ok {
			continue
		}
		next, changed, removeVar := rewriteCheckCommand(value)
		if removeVar {
			edited, err := config.RemoveMappingKey(out, "vars", key)
			if err != nil { // coverage-ignore: out was parsed as a mapping above
				return nil, nil, err
			}
			if !bytes.Equal(edited, out) {
				changes = append(changes, fmt.Sprintf("removed vars.%s naming retired check invariants", key))
				out = edited
			}
			continue
		}
		if !changed {
			continue
		}
		edited, err := config.SetMappingString(out, "vars", key, next)
		if err != nil { // coverage-ignore: out was parsed as a mapping above
			return nil, nil, err
		}
		if !bytes.Equal(edited, out) {
			changes = append(changes, "retargeted vars."+key)
			out = edited
		}
	}
	return out, changes, nil
}

// applyRetargetCheckCommands ports schema 31 -> 32. It removes command vars
// whose rendered consumers no longer exist, retargets surviving awf check
// invocations to their universe paths, and clears values naming the removed
// invariants report. The schema-19 migration remains frozen historical input.
func applyRetargetCheckCommands(root string, out io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		edited, changes, err := retargetCheckCommandBytes(src)
		if err != nil { // coverage-ignore: editConfig supplies bytes already read intact, and the helper's editor faults are unreachable after its parse
			return nil, err
		}
		for _, change := range changes {
			fmt.Fprintf(out, "retarget-check-commands: %s\n", change)
		}
		return edited, nil
	})
}
