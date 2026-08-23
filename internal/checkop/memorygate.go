package checkop

import (
	"errors"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/memorycite"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

func memoryCheckResult(cfg *config.Config, tree *snapshot.Tree) (checkresult.Result, error) {
	findings := memoryFindings(cfg, tree)
	result, err := memorycite.Result(findings)
	if err != nil { // coverage-ignore: Scan constructs every finding with fixed nonempty evidence
		return checkresult.Result{}, err
	}
	return result, memoryFindingError(findings)
}

func memoryFindings(cfg *config.Config, tree *snapshot.Tree) []memorycite.Finding {
	var configured []config.MemoryExemption
	if cfg.MemoryCite != nil {
		configured = cfg.MemoryCite.Exemptions
	}
	exemptions := make([]memorycite.Exemption, 0, len(configured))
	for _, e := range configured {
		exemptions = append(exemptions, memorycite.Exemption{Path: e.Path, Count: e.Count})
	}
	prefixes := []string{config.DocsDir + "/decisions/", config.DocsDir + "/plans/"}
	var files []memorycite.File
	for _, blob := range tree.List() {
		if !blob.Scannable() {
			continue
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(blob.Path, prefix) {
				files = append(files, memorycite.File{Path: blob.Path, Bytes: blob.Bytes})
				break
			}
		}
	}
	return memorycite.Scan(files, exemptions)
}

func memoryFindingError(findings []memorycite.Finding) error {
	if len(findings) == 0 {
		return nil
	}
	return producedCheckFailure{errors.New("check repo memory: remove the concrete effort-owned memory citation, name the bare .awf/efforts/ directory, use an angle-bracket slug placeholder, or exempt the path in memoryCite.exemptions")}
}
