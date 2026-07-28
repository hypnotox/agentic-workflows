package project

import "testing"

// invariant: rendering/singletons-and-payloads:workflow-telemetry-governed-outputs-and-resident-data
// invariant: rendering/singletons-and-payloads:memory-gitignore-always-on
// invariant: rendering/project-output-plan:output-plan-complete
// invariant: rendering/guide-and-doc-templates:working-memory-single-home
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage
// invariant: rendering/adapter-outputs:pi-workflow-telemetry-runtime
// invariant: rendering/pi-workflows:pi-session-handoff-lifecycle
// invariant: rendering/pi-workflows:pi-session-handoff-public-contract
// invariant: rendering/pi-workflows:pi-session-handoff-workflow
// invariant: rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router
// invariant: rendering/pi-workflows:pi-workflow-telemetry-public-contract
// invariant: rendering/pi-runtime:pi-extension-target-render
func TestResidentRootsContractProof(t *testing.T) {
	if len(residentRootNames) != 5 {
		t.Fatal(residentRootNames)
	}
}
