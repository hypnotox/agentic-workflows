package project

import (
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/configspec"
)

// StaticConfigReference projects configspec (plus catalog-wide potential
// consumers) into the same typed collections the live model uses, minus live
// project state - the pre-adoption fallback `awf config` prints outside an
// adopted tree.
func StaticConfigReference() (ConfigReference, error) {
	potential, err := PotentialVarConsumers()
	if err != nil { // coverage-ignore: PotentialVarConsumers reads only embedded templates
		return ConfigReference{}, err
	}
	var ref ConfigReference
	for _, e := range configspec.Keys() {
		row := ConfigKeyRow{
			Path: e.Path, Type: e.Type, Default: e.Default,
			Description: e.Description, Availability: e.Availability,
		}
		if strings.HasPrefix(e.Path, "sidecar.") {
			ref.SidecarFields = append(ref.SidecarFields, row)
		} else {
			ref.ConfigKeys = append(ref.ConfigKeys, row)
		}
	}
	for _, v := range configspec.VarEntries() {
		consumers := "No catalog artifact references it."
		if c := potential[v.Key]; len(c) > 0 {
			consumers = "Catalog consumers: " + strings.Join(c, ", ") + "."
		}
		ref.VarEntries = append(ref.VarEntries, VarRow{
			Key: v.Key, Description: v.Description, Availability: v.Availability,
			Consumers: consumers,
		})
	}
	for _, d := range configspec.DataKeys() {
		artifact := strings.TrimSuffix(d.Kind, "s") + " " + d.Artifact
		switch d.Artifact {
		case "agents-doc":
			artifact = "agents-doc"
		case "_base": // internal token - adopters know these as their local artifacts
			artifact = "local " + d.Kind
		}
		ref.DataKeys = append(ref.DataKeys, DataKeyRow{Artifact: artifact, Key: d.Key, Description: d.Description})
	}
	return ref, nil
}

// PrintConfigReference prints the model (or the static catalog reference when
// model is nil): every section, or the single entry matching key. An unknown
// key is an error (exit 1 - the CLI shape was valid). It is the typed
// counterpart to the removed map[string]any renderer: every field access
// below is a struct field, so a renamed field is a compile error, never a
// silently empty render.
func PrintConfigReference(stdout io.Writer, key string, model *ConfigReference, header string) error {
	ref := model
	if ref == nil {
		m, err := StaticConfigReference()
		if err != nil { // coverage-ignore: StaticConfigReference fails only on embedded-FS faults
			return err
		}
		ref = &m
	}
	printKeyRow := func(row ConfigKeyRow) {
		fmt.Fprintf(stdout, "%s (%s)\n", row.Path, row.Type)
		fmt.Fprintf(stdout, "  default: %s\n", row.Default)
		if row.Current != "" {
			fmt.Fprintf(stdout, "  current: %s\n", row.Current)
		}
		fmt.Fprintf(stdout, "  %s %s\n", row.Description, row.Availability)
	}
	printVarRow := func(row VarRow) {
		fmt.Fprintf(stdout, "%s (var)\n", row.Key)
		if row.State != "" {
			fmt.Fprintf(stdout, "  state: %s\n", row.State)
		}
		fmt.Fprintf(stdout, "  %s %s\n  %s\n", row.Description, row.Availability, row.Consumers)
	}
	printDataRow := func(row DataKeyRow) {
		fmt.Fprintf(stdout, "%s · data.%s%s\n  %s\n", row.Artifact, row.Key, row.State, row.Description)
	}

	if key != "" {
		found := false
		for _, row := range ref.ConfigKeys {
			if row.Path == key {
				printKeyRow(row)
				found = true
			}
		}
		for _, row := range ref.VarEntries {
			if row.Key == key {
				printVarRow(row)
				found = true
			}
		}
		for _, row := range ref.SidecarFields {
			if row.Path == key {
				printKeyRow(row)
				found = true
			}
		}
		for _, row := range ref.DataKeys {
			if row.Key == key {
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
	for _, row := range ref.ConfigKeys {
		printKeyRow(row)
	}
	fmt.Fprintln(stdout, "\n## Vars")
	for _, row := range ref.VarEntries {
		printVarRow(row)
	}
	fmt.Fprintln(stdout, "\n## Sidecar fields")
	for _, row := range ref.SidecarFields {
		printKeyRow(row)
	}
	fmt.Fprintln(stdout, "\n## Per-artifact data keys")
	for _, row := range ref.DataKeys {
		printDataRow(row)
	}
	return nil
}
