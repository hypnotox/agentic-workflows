package knowledge

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

// starterDescriptionSource holds the description prompt each create-only
// starter writes. Starters render from it and the checker rejects concepts that
// still carry one, so both read this single source.
//
//go:embed starter-descriptions.yaml
var starterDescriptionSource []byte

var starterDescriptions = func() map[string]string {
	var descriptions map[string]string
	if err := yaml.Unmarshal(starterDescriptionSource, &descriptions); err != nil {
		panic("parse starter descriptions: " + err.Error())
	}
	return descriptions
}()

// StarterDescription returns the description prompt written by the named
// starter (intent, spec, plan, adr, or topic).
func StarterDescription(starter string) (string, bool) {
	description, ok := starterDescriptions[starter]
	return description, ok
}

func isStarterDescription(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	for _, description := range starterDescriptions {
		if text == description {
			return true
		}
	}
	return false
}
