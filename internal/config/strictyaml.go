package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// decodeOneYAML accepts exactly one known-field document. Mutable authority
// must not silently ignore a valid second document or trailing content.
func decodeOneYAML(b []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple YAML documents")
		}
		return fmt.Errorf("trailing YAML input: %w", err)
	}
	return nil
}
