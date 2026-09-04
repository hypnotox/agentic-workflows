How awf adopts and initializes a project with explicit answers, safe collision handling, and repository-fact defaults.

## Claims

### `invariant: explicit-answers-win`

A value supplied to awf init via --set or --answers is written verbatim into the scaffolded config and suppresses any prompt for that key, regardless of whether stdin is a terminal.
Backing: test

### `invariant: init-bootstrap-default-on`

`awf init` scaffolds `bootstrap.enabled: true`; no CLI command changes that repository fact.
Backing: test

### `invariant: init-collision-guard`

`awf init` acquires its writer lease before reading mutable authority or planning destinations. Before first config creation it probes every planned output path and reports the complete collision set; any existing output refuses without mutation. Creation is exclusive and no-clobber, so a race at a preflighted destination also refuses rather than overwriting content. If an interrupted first adoption leaves a valid config without its lock, rerun uses that config unchanged, fully preflights all outputs, adopts only exact desired regular files, refuses differing content or mode, and publishes the permanent lock last.
Backing: test

### `invariant: init-noninteractive-default`

awf init with non-terminal stdin and no --set or --answers seeds every standard-footprint var empty and writes no invariants or profile selection.
Backing: test

### `invariant: init-prompts-enabled-vars`

Interactive awf init prompts for the governance footprint and vars referenced by that footprint's unconditional catalog and singleton templates; the seeded config carries that selected var union as empty keys.
Backing: test

### `invariant: init-unborn-head-supported`

Working-state assembly uses an empty committed baseline only when HEAD is specifically unborn, allowing init and check to consume eligible working files while every other repository, reference, and object error remains a failure.
Backing: test
