package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/configspec"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runConfig prints the configuration reference: full or single-entry, with
// live state inside an adopted tree and a static catalog-wide fallback
// outside one (pre-adoption discovery is a supported audience).
func runConfig(cwd, key string, stdout io.Writer) error {
	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
		// invariant: config-command-static-fallback
		return printConfigReference(stdout, key, nil, "config reference (static — not inside an awf project; live state appears inside one)")
	}
	if err := gate(cwd); err != nil {
		return err
	}
	p, err := project.Open(cwd)
	if err != nil {
		return err
	}
	model, err := p.ConfigReferenceModel()
	if err != nil {
		return err
	}
	return printConfigReference(stdout, key, model, "config reference — live state for this project")
}

// staticModel projects configspec (plus catalog-wide potential consumers)
// into the same collection shape the live model uses, minus live state.
func staticModel() (map[string]any, error) {
	potential, err := project.PotentialVarConsumers()
	if err != nil { // coverage-ignore: PotentialVarConsumers reads only embedded templates
		return nil, err
	}
	var configKeys, sidecarFields []map[string]any
	for _, e := range configspec.Keys() {
		row := map[string]any{
			"path": e.Path, "type": e.Type, "default": e.Default,
			"description": e.Description, "availability": e.Availability,
		}
		if strings.HasPrefix(e.Path, "sidecar.") {
			sidecarFields = append(sidecarFields, row)
		} else {
			configKeys = append(configKeys, row)
		}
	}
	var varEntries []map[string]any
	for _, v := range configspec.VarEntries() {
		consumers := "No catalog artifact references it."
		if c := potential[v.Key]; len(c) > 0 {
			consumers = "Catalog consumers: " + strings.Join(c, ", ") + "."
		}
		varEntries = append(varEntries, map[string]any{
			"key": v.Key, "description": v.Description, "availability": v.Availability,
			"consumers": consumers,
		})
	}
	var dataKeys []map[string]any
	for _, d := range configspec.DataKeys() {
		artifact := strings.TrimSuffix(d.Kind, "s") + " " + d.Artifact
		if d.Artifact == "agents-doc" {
			artifact = "agents-doc"
		}
		dataKeys = append(dataKeys, map[string]any{
			"artifact": artifact, "key": d.Key, "description": d.Description,
		})
	}
	return map[string]any{
		"configKeys": configKeys, "varEntries": varEntries,
		"sidecarFields": sidecarFields, "dataKeys": dataKeys,
	}, nil
}

// printConfigReference prints the model (or the static catalog reference when
// model is nil): every section, or the single entry matching key. An unknown
// key is an error (exit 1 — the CLI shape was valid).
func printConfigReference(stdout io.Writer, key string, model map[string]any, header string) error {
	if model == nil {
		m, err := staticModel()
		if err != nil { // coverage-ignore: staticModel fails only on embedded-FS faults
			return err
		}
		model = m
	}
	rows := func(section string) []map[string]any {
		v, _ := model[section].([]map[string]any)
		return v
	}
	str := func(row map[string]any, field string) string {
		s, _ := row[field].(string)
		return s
	}
	printKeyRow := func(row map[string]any) {
		fmt.Fprintf(stdout, "%s (%s)\n", str(row, "path"), str(row, "type"))
		fmt.Fprintf(stdout, "  default: %s\n", str(row, "default"))
		if cur := str(row, "current"); cur != "" {
			fmt.Fprintf(stdout, "  current: %s\n", cur)
		}
		fmt.Fprintf(stdout, "  %s %s\n", str(row, "description"), str(row, "availability"))
	}
	printVarRow := func(row map[string]any) {
		fmt.Fprintf(stdout, "%s (var)\n", str(row, "key"))
		if state := str(row, "state"); state != "" {
			fmt.Fprintf(stdout, "  state: %s\n", state)
		}
		fmt.Fprintf(stdout, "  %s %s\n  %s\n", str(row, "description"), str(row, "availability"), str(row, "consumers"))
	}
	printDataRow := func(row map[string]any) {
		fmt.Fprintf(stdout, "%s · data.%s%s\n  %s\n", str(row, "artifact"), str(row, "key"), str(row, "state"), str(row, "description"))
	}

	if key != "" {
		found := false
		for _, row := range rows("configKeys") {
			if str(row, "path") == key {
				printKeyRow(row)
				found = true
			}
		}
		for _, row := range rows("varEntries") {
			if str(row, "key") == key {
				printVarRow(row)
				found = true
			}
		}
		for _, row := range rows("sidecarFields") {
			if str(row, "path") == key {
				printKeyRow(row)
				found = true
			}
		}
		for _, row := range rows("dataKeys") {
			if str(row, "key") == key {
				printDataRow(row)
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unknown key or var %q; run `awf config` for the full reference", key)
		}
		return nil
	}

	fmt.Fprintln(stdout, header)
	fmt.Fprintln(stdout, "\n## config.yaml keys")
	for _, row := range rows("configKeys") {
		printKeyRow(row)
	}
	fmt.Fprintln(stdout, "\n## Vars")
	for _, row := range rows("varEntries") {
		printVarRow(row)
	}
	fmt.Fprintln(stdout, "\n## Sidecar fields")
	for _, row := range rows("sidecarFields") {
		printKeyRow(row)
	}
	fmt.Fprintln(stdout, "\n## Per-artifact data keys")
	for _, row := range rows("dataKeys") {
		printDataRow(row)
	}
	return nil
}
