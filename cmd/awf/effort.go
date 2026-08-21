package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// effortComposition is the wiring one effort command runs against: the resident
// authority and the managed-topology manager, both bound to the same resolved
// control roots.
var (
	activitySlugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	activityOwnerPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type effortComposition struct {
	service *effort.Service
	manager *worktree.Manager
}

// composeEffort is how runEffort obtains that wiring. It is a parameter rather
// than a package fixture so a test names the composition it is exercising
// instead of replacing one behind the handler's back.
type composeEffort func(ctx context.Context, root string) (effortComposition, error)

// openEffortComposition is the production composition root for the effort
// command group. It resolves the control roots once and binds the seam's
// operations to the two consumers' own contracts: the effort service asks three
// questions of a handle opened on the invoking checkout, while the worktree
// manager receives the opener itself, because it reasons about the invoking and
// the managed checkout together and so opens a handle per checkout it touches.
// That is why the invoking root is opened here and again through the opener:
// a handle is pinned to one root, so the manager cannot borrow this one.
func openEffortComposition(ctx context.Context, root string) (effortComposition, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, root)
	if err != nil {
		return effortComposition{}, err
	}
	repo, err := awfgit.Open(roots.InvokingRoot)
	if err != nil { // coverage-ignore: ResolveControlRoots just proved this path is a checkout; a failed open requires a concurrent repository-identity race
		return effortComposition{}, err
	}
	archiveMarker := func() ([]byte, error) {
		projectState, cfg, _, err := openProjectOperation(ctx, root)
		if err != nil { // coverage-ignore: the gated command already loaded this same project; failure requires a concurrent config-tree race
			return nil, err
		}
		rendered, err := project.RenderResidentMarker(projectState, cfg, string(awfgit.ResidentEffortArchive))
		if err != nil { // coverage-ignore: the gate already built the same closed output plan; failure requires a concurrent config-tree race
			return nil, err
		}
		return []byte(rendered.Content), nil
	}
	service, err := effort.Open(roots, effort.Dependencies{
		Clock:                 time.Now,
		UUID:                  effort.RandomUUIDv4,
		Worktrees:             repo.WorktreeList,
		BranchExists:          repo.BranchExists,
		ValidateRef:           repo.ValidateRefName,
		RemoveTree:            os.RemoveAll,
		ExpectedArchiveMarker: archiveMarker,
	})
	if err != nil { // coverage-ignore: the complete literal dependency set and already-resolved roots make service composition infallible
		return effortComposition{}, err
	}
	manager, err := worktree.Open(roots, openCheckout, service)
	if err != nil { // coverage-ignore: openCheckout just opened this same root above; a second failure requires a concurrent repository-identity race
		return effortComposition{}, err
	}
	return effortComposition{service: service, manager: manager}, nil
}

// openCheckout satisfies the worktree manager's checkout contract directly with
// the Git seam's handle: no adapter stands between them, so the manager's
// contract is exactly a subset of the handle's surface.
func openCheckout(root string) (worktree.Runner, error) { return awfgit.Open(root) }

func runEffort(c *cmdCtx, compose composeEffort) error {
	if err := validateEffortGrammar(c); err != nil {
		if c.sub == "memory" || strings.HasPrefix(c.sub, "memory ") {
			return &usageErr{boundedMemoryCommandError(err).Error()}
		}
		return err
	}
	var editRequest *memoryEditRequest
	if c.sub == "memory edit" {
		request, err := decodeMemoryEditRequest(c.stdin)
		if err != nil {
			return boundedMemoryCommandError(err)
		}
		editRequest = &request
	}
	composed, err := compose(c.ctx, c.root)
	if err != nil {
		return err
	}
	service, manager := composed.service, composed.manager
	selected := firstPos(c.inv.positionals)
	switch c.sub {
	case "new":
		input := effort.NewInput{Slug: c.inv.values["--slug"], Title: selected}
		if c.inv.bools["--no-worktree"] {
			record, err := service.New(c.ctx, input)
			if err != nil {
				return err
			}
			absent := worktree.Result{Condition: "no managed worktree", ChangedTopology: false, NextAction: "continue the effort in " + service.InvokingRoot()}
			return writeEffortNew(c.stdout, record, absent)
		}
		record, result, err := manager.NewEffort(c.ctx, input, c.inv.values["--base"])
		if err != nil {
			return err
		}
		return writeEffortNew(c.stdout, record, result)
	case "list":
		records, err := service.List()
		if err != nil {
			return err
		}
		document, err := effort.ListDocument(records)
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return err
		}
		return presentation.Render(c.stdout, document)
	case "show":
		record, err := service.Show(selected)
		if err != nil {
			return err
		}
		return writeEffort(c.stdout, record)
	case "finish":
		result, err := service.Finish(c.ctx, selected)
		if err != nil {
			return err
		}
		mutation, err := result.FinishMutation(selected)
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return err
		}
		document, err := mutation.Document()
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return err
		}
		return presentation.Render(c.stdout, document)
	case "worktree":
		action, selected := c.inv.positionals[0], c.inv.positionals[1]
		var result worktree.Result
		var err error
		switch action {
		case "add":
			result, err = manager.Add(c.ctx, selected, c.inv.values["--base"])
		case "remove":
			result, err = manager.Remove(c.ctx, selected)
		default: // coverage-ignore: validateEffortGrammar accepts only add or remove before this closed dispatch
			return &usageErr{"usage: awf effort worktree <add|remove> <slug>"}
		}
		return writeWorktreeResult(c.stdout, result, err)
	case "integrate":
		gateCommand, err := integrationGateCommand(c.root)
		if err != nil {
			return err
		}
		result, err := manager.Integrate(c.ctx, selected, gateCommand)
		return writeWorktreeResult(c.stdout, result, err)
	case "memory read", "memory edit", "memory update":
		return runEffortMemory(c, service, editRequest)
	case "activity attach", "activity heartbeat", "activity detach":
		return writeActivityReply(c.stdout, runEffortActivity(c, service))
	default:
		return &usageErr{"usage: awf effort <new|list|show|finish|worktree|integrate|memory|activity>"}
	}
}

func effortValue(inv invocation, flag string) *string {
	value, ok := inv.values[flag]
	if !ok {
		return nil
	}
	return &value
}

func runEffortActivity(c *cmdCtx, service *effort.Service) effort.ActivityReply {
	slug := firstPos(c.inv.positionals)
	switch c.sub {
	case "activity attach":
		return service.AttachActivity(slug, c.inv.values["--owner"])
	case "activity heartbeat":
		return service.HeartbeatActivity(slug, c.inv.values["--owner"])
	case "activity detach":
		return service.DetachActivity(slug, c.inv.values["--owner"])
	default:
		panic("unreachable effort activity action")
	}
}
func writeActivityReply(out io.Writer, reply effort.ActivityReply) error {
	return writeEffortActivityProtocol(out, reply)
}

func integrationGateCommand(root string) (string, error) {
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	command, ok := cfg.Vars["gateCmd"].(string)
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(command), nil
}

func validateEffortGrammar(c *cmdCtx) error {
	if c.sub == "memory" {
		return &usageErr{"usage: awf effort memory <read|edit|update>"}
	}
	if c.sub == "activity" {
		return &usageErr{"usage: awf effort activity <attach|heartbeat|detach>"}
	}
	if strings.HasPrefix(c.sub, "memory ") {
		return validateEffortMemoryGrammar(c)
	}
	if strings.HasPrefix(c.sub, "activity ") {
		return validateEffortActivityGrammar(c)
	}
	if c.sub == "new" {
		if _, ok := c.inv.values["--slug"]; !ok {
			return &usageErr{"awf effort new: --slug is required"}
		}
		if c.inv.bools["--no-worktree"] && c.inv.values["--base"] != "" {
			return &usageErr{"awf effort new: --base is invalid with --no-worktree"}
		}
		return nil
	}
	if c.sub != "worktree" {
		return nil
	}
	if len(c.inv.positionals) != 2 {
		return &usageErr{"usage: awf effort worktree <add|remove> <slug>"}
	}
	switch c.inv.positionals[0] {
	case "add":
		return nil
	case "remove":
		if c.inv.values["--base"] != "" {
			return &usageErr{"awf effort worktree remove: --base is not allowed"}
		}
		return nil
	default:
		return &usageErr{"usage: awf effort worktree <add|remove> <slug>"}
	}
}

func validateEffortMemoryGrammar(c *cmdCtx) error {
	usage := "usage: awf effort " + c.sub
	slug := firstPos(c.inv.positionals)
	if len(c.inv.positionals) != 1 || !activitySlugPattern.MatchString(slug) || len(slug) > 63 {
		return &usageErr{usage + " requires a canonical 1-63-byte slug"}
	}
	owner, hasOwner := c.inv.values["--owner"]
	hasJSON := c.inv.bools["--json"]
	if hasOwner != hasJSON {
		return &usageErr{usage + " requires --owner and --json together"}
	}
	if c.inv.bools["--preview"] && (!hasOwner || !hasJSON) {
		return &usageErr{usage + " preview requires --owner and --json"}
	}
	if hasOwner && !activityOwnerPattern.MatchString(owner) {
		return &usageErr{usage + " requires a lowercase UUIDv4 owner"}
	}
	if c.sub == "memory read" {
		for _, flag := range []string{"--offset", "--limit"} {
			if value, ok := c.inv.values[flag]; ok {
				parsed, err := strconv.Atoi(value)
				if err != nil || parsed < 1 {
					return &usageErr{usage + " requires " + flag + " to be a positive integer"}
				}
			}
		}
	}
	if c.sub == "memory update" {
		if _, phase := c.inv.values["--phase"]; !phase {
			if _, next := c.inv.values["--next"]; !next {
				return &usageErr{usage + " requires --phase or --next"}
			}
		}
	}
	return nil
}

const (
	maxMemoryEditRequestBytes       = 16 << 20
	maxMemoryCommandDiagnosticBytes = 50 << 10
)

func boundedMemoryCommandError(err error) error {
	const framing = "condition: awf: "
	limit := maxMemoryCommandDiagnosticBytes - len(framing) - len("\n")
	raw := []byte(strings.ToValidUTF8(err.Error(), "?"))
	if len(raw) > limit {
		raw = raw[:limit]
		for !utf8.Valid(raw) {
			raw = raw[:len(raw)-1]
		}
	}
	return errors.New(string(raw))
}

type memoryEditRequest struct {
	Edits []memoryEditItem `json:"edits"`
}

type memoryEditItem struct {
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok { // coverage-ignore: json.Decoder guarantees object member tokens are strings after More
					return errors.New("JSON object key is not a string")
				}
				if _, duplicate := seen[key]; duplicate {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default: // coverage-ignore: json.Decoder yields only object or array opening delimiters at a value position
			return fmt.Errorf("unexpected JSON delimiter %q", delim)
		}
	}
	return walk()
}

func decodeMemoryEditRequest(input io.Reader) (memoryEditRequest, error) {
	limited := io.LimitReader(input, maxMemoryEditRequestBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return memoryEditRequest{}, fmt.Errorf("read memory edit request: %w", err)
	}
	if len(raw) > maxMemoryEditRequestBytes {
		return memoryEditRequest{}, errors.New("memory edit request exceeds 16 MiB")
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return memoryEditRequest{}, fmt.Errorf("decode memory edit request: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request memoryEditRequest
	if err := decoder.Decode(&request); err != nil {
		return memoryEditRequest{}, fmt.Errorf("decode memory edit request: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return memoryEditRequest{}, errors.New("memory edit request contains a trailing JSON value")
		}
		return memoryEditRequest{}, fmt.Errorf("decode memory edit request: %w", err)
	}
	if len(request.Edits) < 1 || len(request.Edits) > 128 {
		return memoryEditRequest{}, errors.New("memory edit request requires 1 through 128 edits")
	}
	for i, edit := range request.Edits {
		if edit.OldText == "" || len(edit.OldText) > 1<<20 || len(edit.NewText) > 1<<20 || !utf8.ValidString(edit.OldText) || !utf8.ValidString(edit.NewText) {
			return memoryEditRequest{}, fmt.Errorf("memory edit %d strings must be valid UTF-8, oldText must be nonempty, and each string must be at most 1 MiB", i)
		}
	}
	return request, nil
}

func runEffortMemory(c *cmdCtx, service *effort.Service, request *memoryEditRequest) error {
	slug := firstPos(c.inv.positionals)
	owner := c.inv.values["--owner"]
	var input effort.MemoryOperationInput
	switch c.sub {
	case "memory read":
		offset, limit := 0, 0
		if value, ok := c.inv.values["--offset"]; ok {
			offset, _ = strconv.Atoi(value)
		}
		if value, ok := c.inv.values["--limit"]; ok {
			limit, _ = strconv.Atoi(value)
		}
		input = effort.MemoryReadInput{Slug: slug, Owner: owner, Offset: offset, Limit: limit}
	case "memory edit":
		edits := make([]effort.MemoryReplacement, len(request.Edits))
		for i, edit := range request.Edits {
			edits[i] = effort.MemoryReplacement{OldText: edit.OldText, NewText: edit.NewText}
		}
		input = effort.MemoryEditInput{Slug: slug, Owner: owner, Edits: edits, Preview: c.inv.bools["--preview"]}
	case "memory update":
		input = effort.MemoryUpdateInput{Slug: slug, Owner: owner, Update: effort.MemoryUpdate{Phase: effortValue(c.inv, "--phase"), Next: effortValue(c.inv, "--next")}, Preview: c.inv.bools["--preview"]}
	default: // coverage-ignore: validateEffortGrammar and closed command dispatch admit only the three memory leaves
		return errors.New("unsupported memory command")
	}
	var result effort.MemoryOperationResult
	var err error
	if update, ok := input.(effort.MemoryUpdateInput); ok {
		result, err = service.UpdateMemory(update)
	} else {
		result, err = service.Memory(input)
	}
	if err != nil {
		return err
	}
	if c.inv.bools["--json"] {
		return writeEffortMemoryProtocol(c.stdout, result)
	}
	document, err := result.MemoryDocument()
	if err != nil { // coverage-ignore: service results satisfy the effort-owned presentation model
		return err
	}
	return presentation.Render(c.stdout, document)
}

func validateEffortActivityGrammar(c *cmdCtx) error {
	usage := "usage: awf effort " + c.sub
	if len(c.inv.positionals) != 1 || !activitySlugPattern.MatchString(firstPos(c.inv.positionals)) || len(firstPos(c.inv.positionals)) > 63 {
		return &usageErr{usage + " requires a canonical 1-63-byte slug"}
	}
	if !c.inv.bools["--json"] {
		return &usageErr{usage + " requires --json"}
	}
	for _, flag := range activityRequiredFlags(c.sub) {
		if _, ok := c.inv.values[flag]; !ok {
			return &usageErr{usage + " requires " + flag}
		}
	}
	if len(c.inv.values) != 1 {
		return &usageErr{usage + " accepts only --owner and --json"}
	}
	if !activityOwnerPattern.MatchString(c.inv.values["--owner"]) {
		return &usageErr{usage + " requires a lowercase UUIDv4 owner"}
	}
	return nil
}

func activityRequiredFlags(action string) []string {
	switch action {
	case "activity attach", "activity heartbeat", "activity detach":
		return []string{"--owner"}
	default:
		return nil
	}
}
func writeWorktreeResult(out io.Writer, result worktree.Result, operationErr error) error {
	if operationErr != nil {
		return operationErr
	}
	mutation, err := result.Mutation()
	if err != nil {
		return err
	}
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return presentation.Render(out, document)
}

func writeEffort(out io.Writer, record effort.Record) error {
	detail, err := record.Detail()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	document, err := detail.Document()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return presentation.Render(out, document)
}

func writeEffortNew(out io.Writer, record effort.Record, result worktree.Result) error {
	mutation, err := result.Mutation()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	mutation, err = record.NewEffortMutation(mutation)
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return presentation.Render(out, document)
}

type memoryProtocolMetadata struct {
	Effort  string `json:"effort"`
	Phase   string `json:"phase"`
	Next    string `json:"next"`
	Updated string `json:"updated"`
}

type memoryProtocolOutcome struct {
	Category      string   `json:"category"`
	Condition     string   `json:"condition"`
	ChangedMemory bool     `json:"changedMemory"`
	NextActions   []string `json:"nextActions"`
	Cause         string   `json:"cause,omitempty"`
}

type memoryProtocolRange struct {
	StartLine   int    `json:"startLine"`
	EndLine     int    `json:"endLine"`
	TotalLines  int    `json:"totalLines"`
	NextOffset  *int   `json:"nextOffset"`
	TruncatedBy string `json:"truncatedBy"`
}

type memoryProtocolDiff struct {
	Text             string `json:"text"`
	FirstChangedLine *int   `json:"firstChangedLine"`
	Truncated        bool   `json:"truncated"`
}

// writeEffortMemoryProtocol writes exactly one bounded protocol-1 envelope.
func writeEffortMemoryProtocol(out io.Writer, result effort.MemoryOperationResult) error {
	metadata := func(value *effort.MemoryMetadata) memoryProtocolMetadata {
		return memoryProtocolMetadata{Effort: value.Effort, Phase: value.Phase, Next: value.Next, Updated: value.Updated}
	}
	var envelope any
	switch result.Condition {
	case effort.MemoryRead:
		envelope = struct {
			SchemaVersion int                    `json:"schemaVersion"`
			Condition     effort.MemoryCondition `json:"condition"`
			Memory        memoryProtocolMetadata `json:"memory"`
			Content       string                 `json:"content"`
			Range         memoryProtocolRange    `json:"range"`
		}{1, result.Condition, metadata(result.Memory), result.Content, memoryProtocolRange{result.Range.StartLine, result.Range.EndLine, result.Range.TotalLines, result.Range.NextOffset, result.Range.TruncatedBy}}
	case effort.MemoryEdited:
		envelope = struct {
			SchemaVersion    int                    `json:"schemaVersion"`
			Condition        effort.MemoryCondition `json:"condition"`
			Memory           memoryProtocolMetadata `json:"memory"`
			ReplacementCount int                    `json:"replacementCount"`
			Diff             memoryProtocolDiff     `json:"diff"`
		}{1, result.Condition, metadata(result.Memory), result.ReplacementCount, memoryProtocolDiff{result.Diff.Text, result.Diff.FirstChangedLine, result.Diff.Truncated}}
	case effort.MemoryUpdated:
		envelope = struct {
			SchemaVersion int                    `json:"schemaVersion"`
			Condition     effort.MemoryCondition `json:"condition"`
			Memory        memoryProtocolMetadata `json:"memory"`
			Diff          memoryProtocolDiff     `json:"diff"`
		}{1, result.Condition, metadata(result.Memory), memoryProtocolDiff{result.Diff.Text, result.Diff.FirstChangedLine, result.Diff.Truncated}}
	case effort.MemoryPreviewed:
		if result.ReplacementCount > 0 {
			envelope = struct {
				SchemaVersion    int                    `json:"schemaVersion"`
				Condition        effort.MemoryCondition `json:"condition"`
				ReplacementCount int                    `json:"replacementCount"`
				Diff             memoryProtocolDiff     `json:"diff"`
			}{1, result.Condition, result.ReplacementCount, memoryProtocolDiff{result.Diff.Text, result.Diff.FirstChangedLine, result.Diff.Truncated}}
		} else {
			envelope = struct {
				SchemaVersion int                    `json:"schemaVersion"`
				Condition     effort.MemoryCondition `json:"condition"`
				Diff          memoryProtocolDiff     `json:"diff"`
			}{1, result.Condition, memoryProtocolDiff{result.Diff.Text, result.Diff.FirstChangedLine, result.Diff.Truncated}}
		}
	default:
		actions := make([]string, len(result.Outcome.NextActions))
		for i, action := range result.Outcome.NextActions {
			actions[i] = action.Text
		}
		outcome := memoryProtocolOutcome{Category: result.Outcome.Category, Condition: result.Outcome.Condition, ChangedMemory: result.Outcome.ChangedMemory, NextActions: actions}
		if result.Condition == effort.MemoryFailure {
			outcome.Cause = result.Outcome.Cause
		}
		switch result.Condition {
		case effort.MemoryOffsetOutOfRange:
			rangeFact := struct {
				Offset     int `json:"offset"`
				TotalLines int `json:"totalLines"`
			}{result.Offset.Offset, result.Offset.TotalLines}
			envelope = struct {
				SchemaVersion int                    `json:"schemaVersion"`
				Condition     effort.MemoryCondition `json:"condition"`
				Outcome       memoryProtocolOutcome  `json:"outcome"`
				Range         any                    `json:"range"`
			}{1, result.Condition, outcome, rangeFact}
		case effort.MemoryNoMatch, effort.MemoryAmbiguousMatch:
			editFact := struct {
				Index       int `json:"index"`
				Occurrences int `json:"occurrences,omitempty"`
			}{result.Edit.Index, result.Edit.Occurrences}
			envelope = struct {
				SchemaVersion int                    `json:"schemaVersion"`
				Condition     effort.MemoryCondition `json:"condition"`
				Outcome       memoryProtocolOutcome  `json:"outcome"`
				Edit          any                    `json:"edit"`
			}{1, result.Condition, outcome, editFact}
		case effort.MemoryOverlappingEdits:
			editsFact := struct {
				FirstIndex  int `json:"firstIndex"`
				SecondIndex int `json:"secondIndex"`
			}{result.Overlap.FirstIndex, result.Overlap.SecondIndex}
			envelope = struct {
				SchemaVersion int                    `json:"schemaVersion"`
				Condition     effort.MemoryCondition `json:"condition"`
				Outcome       memoryProtocolOutcome  `json:"outcome"`
				Edits         any                    `json:"edits"`
			}{1, result.Condition, outcome, editsFact}
		case effort.MemoryResultTooLarge:
			sizeFact := struct {
				Bytes    int `json:"bytes"`
				MaxBytes int `json:"maxBytes"`
			}{result.Size.Bytes, result.Size.MaxBytes}
			envelope = struct {
				SchemaVersion int                    `json:"schemaVersion"`
				Condition     effort.MemoryCondition `json:"condition"`
				Outcome       memoryProtocolOutcome  `json:"outcome"`
				Size          any                    `json:"size"`
			}{1, result.Condition, outcome, sizeFact}
		case effort.MemoryNotOwner, effort.MemoryMissing, effort.MemoryUnsafeActivity, effort.MemoryInvalid, effort.MemoryUnsafe, effort.MemoryFailure:
			envelope = struct {
				SchemaVersion int                    `json:"schemaVersion"`
				Condition     effort.MemoryCondition `json:"condition"`
				Outcome       memoryProtocolOutcome  `json:"outcome"`
			}{1, result.Condition, outcome}
		default:
			return fmt.Errorf("unsupported memory result condition %q", result.Condition)
		}
	}
	raw, err := json.Marshal(envelope)
	if err != nil { // coverage-ignore: the closed protocol structs contain only JSON-supported values
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > 1<<20 {
		return errors.New("memory protocol reply exceeds 1 MiB")
	}
	n, err := out.Write(raw)
	if err != nil {
		return err
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil
}

// writeEffortActivityProtocol writes the documented activity JSON protocol.
// It is a closed successful protocol bypass.
func writeEffortActivityProtocol(out io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil { // coverage-ignore: fixed protocol types cannot fail encoding; writer failures are covered at the shared output boundary
		return err
	}
	raw = append(raw, '\n')
	_, err = out.Write(raw)
	return err
}
