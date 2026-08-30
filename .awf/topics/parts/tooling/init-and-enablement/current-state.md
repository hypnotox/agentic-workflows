How awf adopts and initializes a project with explicit answers, safe collision handling, and repository-fact defaults.

## Claims

### `invariant: explicit-answers-win`

A value supplied to awf init via --set or --answers is written verbatim into the scaffolded config and suppresses any prompt for that key, regardless of whether stdin is a terminal.
Backing: test

### `invariant: init-bootstrap-default-on`

`awf init` scaffolds `bootstrap.enabled: true`; no CLI command changes that repository fact.
Backing: test

### `invariant: init-collision-guard`

Before writing anything, `awf init` pre-flights every path it would create and, if any already exists, writes nothing and reports the offending paths; `awf init --force` backs up each colliding file to `<path>.awf-bak` and overwrites.
Backing: test

### `invariant: init-force-backs-up`

Running init with --force copies every colliding non-managed file to <path>.awf-bak before any managed output overwrites it, and reports the backup on stdout.
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
