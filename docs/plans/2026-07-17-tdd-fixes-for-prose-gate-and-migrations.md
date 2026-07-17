---
date: 2026-07-17
adrs: [10, 119, 120]
status: Proposed
---
# Plan: TDD fixes for prose gate and migrations

## Goal

Make `awf prose-gate` validate the exact staged policy and text that a commit carries, and make the schema-9 and schema-10 migrations preserve adopter content and ADR structure. Non-goals: changing prose-gate's configured punctuation set, adding a new migration generation, or changing ADR-0119/ADR-0120 policy beyond repairing their existing contracts.

## Architecture summary

Add a staged-regular-blob reader in `internal/git`; the prose-gate command selects the staged `.awf/config.yaml` from that snapshot, parses it through the config decoder, and sends the same snapshot's blobs to the scanner. The scanner skips and reports non-UTF-8 blobs as binary, but never lets a pinned exemption evade its zero-count comparison.

Replace the pitfalls migration's hand-built YAML with validated typed YAML serialization before deleting legacy input. Make ADR's Markdown section/item/token parser fence-aware while retaining raw section offsets; use that parser for both normal checks and the retirement migration's insertion position.

## File structure

- **Created:** none.
- **Modified:** `internal/git/git.go`, `internal/git/git_test.go`, `internal/config/config.go`, `internal/config/config_test.go`, `cmd/awf/prosegate.go`, `cmd/awf/prosegate_test.go`, `internal/prosegate/prosegate.go`, `internal/prosegate/prosegate_test.go`, `internal/migrate/pitfalls.go`, `internal/migrate/pitfalls_test.go`, `internal/adr/adr.go`, `internal/adr/adr_test.go`, `internal/migrate/retirementtokens.go`, `internal/migrate/retirementtokens_test.go`.
- **Deleted:** none.

## Phase 1: Scan the staged policy and staged regular blobs

- [ ] **Task 1.1: Add failing staged-snapshot regressions.** In `internal/git/git_test.go`, add a fixture whose index contains an ordinary file, an executable file, a symlink, a gitlink/submodule entry, and an unsupported unmerged or content-less entry; assert the new staged-blob API returns the ordinary and executable blobs with their exact index bytes and rejects ambiguous/non-committable index state rather than choosing an arbitrary stage. In `cmd/awf/prosegate_test.go`, add regressions that (a) stage a banned codepoint then clean the worktree copy without restaging, (b) remove a staged-clean file from the worktree without staging its deletion, (c) alter the worktree `proseGate.enabled` or exemption after staging a different `config.yaml`, and (d) include a staged gitlink while a regular staged prose file remains clean. Assert the first case reports the banned codepoint, the second succeeds from staged bytes, the staged config controls enabled/exemption behavior, and the gitlink does not block scanning. Run:
  ```sh
  go test ./internal/git ./cmd/awf -run 'Test(Index|ProseGate)'
  ```
  Expected: at least the new staged-content assertions fail against the worktree-reading implementation.

- [ ] **Task 1.2: Implement a single staged input path.** In `internal/git/git.go`, replace or supplement `IndexPaths` with an API returning sorted `{Path, Bytes}` entries from stage-0, ordinary or executable index blobs. Exclude symlinks and gitlinks; return a named error for unmerged stages or an entry without a readable blob. In `internal/config/config.go`, extract the strict YAML decoding/defaulting currently in `Load` into a byte-taking helper that accepts the owning `.awf` directory, preserves the supplied bytes in `Config.Source()`, and lets `Load` remain its filesystem wrapper. In `cmd/awf/prosegate.go`, obtain the staged blobs once, locate `.awf/config.yaml`, parse it through the new config helper, preserve the existing disabled-knob no-op, and pass all staged blobs to prose-gate. Refuse clearly when the staged snapshot has no config rather than falling back to worktree policy. Change `internal/prosegate/prosegate.go` only as needed to accept `{Path, Bytes}` scan inputs without importing `internal/git`. Run:
  ```sh
  go test ./internal/git ./internal/config ./internal/prosegate ./cmd/awf
  ```
  Expected: `ok` for all four packages.

- [ ] **Task 1.3: Verify and commit.** Run:
  ```sh
  ./x gate
  git add internal/git/git.go internal/git/git_test.go internal/config/config.go internal/config/config_test.go cmd/awf/prosegate.go cmd/awf/prosegate_test.go internal/prosegate/prosegate.go internal/prosegate/prosegate_test.go
  git commit -m "fix(tooling): scan staged prose gate inputs"
  ```
  Expected: gate exits 0 and the commit succeeds.
  ```commit
  fix(tooling): scan staged prose gate inputs
  ```

## Phase 2: Make prose-gate exclusions and pins honest

- [ ] **Task 2.1: Add failing scanner regressions.** In `internal/prosegate/prosegate_test.go`, add a staged-input fixture with an invalid-UTF-8 blob and assert `Scan` produces no punctuation finding but returns one deterministic skipped-binary path for command reporting. Add pinned-exemption cases where the configured path is present but clean and where it is absent from the staged blob set; both must produce a finding with actual count zero and the configured pin. Preserve assertions that an unpinned exemption permits any observed count. In `cmd/awf/prosegate_test.go`, assert one `skipped binary: <path>` line is printed in sorted order and does not make an otherwise-clean command fail. Run:
  ```sh
  go test ./internal/prosegate ./cmd/awf -run 'Test(Scan|ProseGate)'
  ```
  Expected: the new binary-reporting and zero-count-pin assertions fail.

- [ ] **Task 2.2: Report binary exclusions and evaluate every pinned exemption.** In `internal/prosegate/prosegate.go`, classify invalid UTF-8 regular blobs as binary, return their sorted paths separately from sorted findings, and do not scan their contents. Build actual per-path/per-rune counts before evaluating exemptions, then iterate every pinned exemption so missing/clean paths compare as count zero; keep unpinned exemptions permissive. In `cmd/awf/prosegate.go`, print `skipped binary: <path>` for each returned path before findings and retain a zero exit when no findings exist. Update existing scan tests from worktree-file inputs to the staged-input form introduced in Phase 1. Run:
  ```sh
  go test ./internal/prosegate ./cmd/awf
  ```
  Expected: `ok` for both packages.

- [ ] **Task 2.3: Verify and commit.** Run:
  ```sh
  ./x gate
  git add internal/prosegate/prosegate.go internal/prosegate/prosegate_test.go cmd/awf/prosegate.go cmd/awf/prosegate_test.go
  git commit -m "fix(tooling): harden prose gate exclusions"
  ```
  Expected: gate exits 0 and the commit succeeds.
  ```commit
  fix(tooling): harden prose gate exclusions
  ```

## Phase 3: Preserve pitfalls migration content before deletion

- [ ] **Task 3.1: Add failing lossless-migration regressions.** In `internal/migrate/pitfalls_test.go`, add one legacy entry whose body starts with an indented Markdown code block and later has column-zero prose, and another whose full body is indented code. Assert the generated sidecar parses, its decoded title/body exactly equal the split input, and the rendered indentation remains intact. Add a failure-seam test for invalid generated output/validation failure and assert `entries.md` remains present with byte-identical content. Run:
  ```sh
  go test ./internal/migrate -run TestPitfallsData
  ```
  Expected: the new indentation round-trip and source-preservation assertions fail.

- [ ] **Task 3.2: Serialize and validate before removing the legacy part.** In `internal/migrate/pitfalls.go`, replace `renderPitfallsSidecar`'s manual literal-scalar construction with a typed `yaml.v3` sidecar model. Marshal the split entries, unmarshal the result into the same model, and compare title/body values before `manifest.WriteFileAtomic`. Only after that successful validation and atomic write may `applyPitfallsData` remove `entries.md` and its empty parent directory. Keep the absent-part no-op and provenance output, but remove wording that accepts content loss when no headings are found: return a named migration error and preserve the source instead. Run:
  ```sh
  go test ./internal/migrate -run TestPitfallsData
  ```
  Expected: `ok   github.com/hypnotox/agentic-workflows/internal/migrate`.

- [ ] **Task 3.3: Verify and commit.** Run:
  ```sh
  ./x gate
  git add internal/migrate/pitfalls.go internal/migrate/pitfalls_test.go
  git commit -m "fix(config): preserve pitfalls migration content"
  ```
  Expected: gate exits 0 and the commit succeeds.
  ```commit
  fix(config): preserve pitfalls migration content
  ```

## Phase 4: Ignore fenced ADR syntax during migration and checks

- [ ] **Task 4.1: Add failing fence regressions at the parser boundary.** In `internal/adr/adr_test.go`, replace the raw-fence expectations with cases proving that a level-two heading, numbered item, and supersession token inside either a backtick or tilde fenced block are inert; assert real syntax after the fence remains visible and ordered. Add a parser assertion that exposes the raw start/end offset of a Decision section ending at the next non-fenced level-two heading. In `internal/migrate/retirementtokens_test.go`, add a carrier whose Decision contains a fenced fake `##` heading and fake numbered item before real item 2; assert migration appends outside the fence, after item 2, as item 3, with the exact expected token and back-pointer. Run:
  ```sh
  go test ./internal/adr ./internal/migrate -run 'Test(DecisionItems|SupersessionRefExtraction|RetirementTokens)'
  ```
  Expected: the new fenced-syntax and bookkeeping-placement assertions fail.

- [ ] **Task 4.2: Centralize fence-aware ADR structure.** In `internal/adr/adr.go`, replace the fence-blind section splitter with a line walker that recognizes backtick and tilde fences, records raw byte offsets for non-fenced level-two sections, and derives section bodies, Decision item numbers, and `SupersessionRef`s exclusively from non-fenced lines. Keep column-zero item semantics and existing frontmatter behavior. In `internal/migrate/retirementtokens.go`, use the parsed raw Decision-section end offset and fence-aware `DecisionItems` to insert bookkeeping before the next real section or at the true document end, never inside a fence; retain frontmatter-only surgery and idempotency. Run:
  ```sh
  go test ./internal/adr ./internal/migrate ./internal/project
  ```
  Expected: `ok` for all three packages.

- [ ] **Task 4.3: Verify, commit, and freeze the plan.** Run:
  ```sh
  ./x gate
  git add internal/adr/adr.go internal/adr/adr_test.go internal/migrate/retirementtokens.go internal/migrate/retirementtokens_test.go docs/plans/2026-07-17-tdd-fixes-for-prose-gate-and-migrations.md
  git commit -m "fix(adr-system): ignore fenced ADR syntax"
  ```
  Expected: gate exits 0 and the commit succeeds. Change this plan's frontmatter from `status: Proposed` to `status: Implemented` before staging it.
  ```commit
  fix(adr-system): ignore fenced ADR syntax
  ```

## Verification

- `./x gate` exits 0 after every phase and on the final tree.
- `go test ./internal/git ./internal/config ./internal/prosegate ./cmd/awf ./internal/migrate ./internal/adr ./internal/project` exits 0.
- The final prose-gate tests prove configuration, paths, and bytes all originate from the staged snapshot; ordinary and executable staged blobs scan, while symlinks, gitlinks, and binary blobs do not create false failures.
- The final migration tests prove valid, lossless pitfalls YAML before source deletion and fence-safe, sequential retirement bookkeeping.

## Notes

- The staged-input design deliberately refuses an enabled prose-gate invocation when `.awf/config.yaml` is absent from the staged snapshot; falling back to the worktree would reintroduce policy mismatch.
- Direct byte scanning of non-UTF-8 files is out of scope by settled design: those blobs are binary, skipped, and reported without failing the gate.
- The parser correction also fixes the related `awf check` fence-blindness found during grounding; it is necessary for migration and checker semantics to agree.
