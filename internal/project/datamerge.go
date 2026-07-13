package project

import "github.com/hypnotox/agentic-workflows/internal/config"

// withDefaultData overlays a sidecar onto an artifact's catalog default data:
// a key absent from the sidecar falls through to the default; a key present in
// the sidecar — even null or empty — replaces it (the explicit off-switch,
// ADR-0045). The merged sidecar feeds renderTarget AND artifactConfigHash, so
// catalog default data participates in the drift signal.
// touches-invariant: sidecar-key-overrides-default — sidecar-over-default data merge; proof in datamerge_test.go
func withDefaultData(sc config.Sidecar, defaults map[string]any) config.Sidecar {
	if len(defaults) == 0 {
		return sc
	}
	merged := make(map[string]any, len(defaults)+len(sc.Data))
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range sc.Data {
		merged[k] = v
	}
	sc.Data = merged
	return sc
}
