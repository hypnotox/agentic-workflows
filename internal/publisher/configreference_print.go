package publisher

import (
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/configspec"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// StaticConfigReference projects configspec (plus catalog-wide potential
// consumers) into the same typed collections the live model uses, minus live
// project state - the pre-adoption fallback `awf config` prints outside an
// adopted tree.
func StaticConfigReference() (ConfigReference, error) {
	potential, err := potentialVarConsumersForCatalog()
	if err != nil { // coverage-ignore: potentialVarConsumersForCatalog reads only embedded templates
		return ConfigReference{}, err
	}
	var ref ConfigReference
	for _, e := range configspec.Keys() {
		row := ConfigKeyRow{Path: e.Path, Type: e.Type, Default: e.Default, Description: e.Description, Availability: e.Availability}
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
		ref.VarEntries = append(ref.VarEntries, VarRow{Key: v.Key, Description: v.Description, Availability: v.Availability, Consumers: consumers})
	}
	for _, d := range configspec.DataKeys() {
		artifact := strings.TrimSuffix(d.Kind, "s") + " " + d.Artifact
		if d.Artifact == "agents-doc" {
			artifact = "agents-doc"
		}
		ref.DataKeys = append(ref.DataKeys, DataKeyRow{Artifact: artifact, Key: d.Key, Description: d.Description})
	}
	return ref, nil
}

// ConfigReferencePresentation maps the typed reference into Collection. The
// reference model remains project-owned; presentation owns only the grammar.
func ConfigReferencePresentation(key string, model *ConfigReference, status string) (presentation.Document, error) {
	ref := model
	if ref == nil {
		static, err := StaticConfigReference()
		if err != nil { // coverage-ignore: embedded configspec catalog decoding is validated at build and package-test time
			return presentation.Document{}, err
		}
		ref = &static
	}
	categories := []presentation.CollectionCategory{}
	addConfigRows := func(label string, rows []ConfigKeyRow) error {
		records := make([]presentation.Record, 0, len(rows))
		for _, row := range rows {
			if key != "" && row.Path != key {
				continue
			}
			current := row.Current
			if current == "" {
				current = "none"
			}
			values := []string{row.Path, row.Type, row.Default, row.Description, row.Availability, current}
			record, err := configReferenceRecord(values...)
			if err != nil {
				return err
			}
			records = append(records, record)
		}
		if len(records) > 0 {
			categories = append(categories, presentation.CollectionCategory{Label: label, Schema: []string{"path", "type", "default", "description", "availability", "current"}, Records: records})
		}
		return nil
	}
	if err := addConfigRows("config keys", ref.ConfigKeys); err != nil {
		return presentation.Document{}, err
	}
	varRows := make([]presentation.Record, 0, len(ref.VarEntries))
	for _, row := range ref.VarEntries {
		if key != "" && row.Key != key {
			continue
		}
		state := row.State
		if state == "" {
			state = "none"
		}
		values := []string{row.Key, row.Description, row.Availability, row.Consumers, state}
		record, err := configReferenceRecord(values...)
		if err != nil {
			return presentation.Document{}, err
		}
		varRows = append(varRows, record)
	}
	if len(varRows) > 0 {
		categories = append(categories, presentation.CollectionCategory{Label: "vars", Schema: []string{"key", "description", "availability", "consumers", "state"}, Records: varRows})
	}
	if err := addConfigRows("sidecar fields", ref.SidecarFields); err != nil {
		return presentation.Document{}, err
	}
	dataRows := make([]presentation.Record, 0, len(ref.DataKeys))
	for _, row := range ref.DataKeys {
		if key != "" && row.Key != key {
			continue
		}
		state := row.State
		if state == "" {
			state = "none"
		}
		values := []string{row.Artifact, row.Key, row.Description, state}
		record, err := configReferenceRecord(values...)
		if err != nil {
			return presentation.Document{}, err
		}
		dataRows = append(dataRows, record)
	}
	if len(dataRows) > 0 {
		categories = append(categories, presentation.CollectionCategory{Label: "data keys", Schema: []string{"artifact", "key", "description", "state"}, Records: dataRows})
	}
	if len(categories) == 0 {
		return presentation.Document{}, fmt.Errorf("unknown key or var %q; run `awf config` for the full reference", key)
	}
	return (presentation.Collection{Status: status, Categories: categories}).Document()
}

func configReferenceRecord(fields ...string) (presentation.Record, error) {
	values := make([]presentation.Value, len(fields))
	for i, field := range fields {
		value, err := presentation.Prose(field)
		if err != nil {
			return presentation.Record{}, err
		}
		values[i] = value
	}
	return presentation.NewRecord(values...)
}
