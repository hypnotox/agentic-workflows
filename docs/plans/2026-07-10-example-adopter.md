# 2026-07-10 — In-repo example adopter (`examples/sundial/`)

**Goal:** implement [ADR-0090](../decisions/0090-in-repo-example-adopter-as-onboarding-artifact-and-rendered-output-quality-oracle.md): a committed, full-surface example adopter at `examples/sundial/` — cold-start onboarding artifact and deterministic rendered-output quality oracle — wired into `./x sync` and `./x check`.

**Architecture summary:** see the ADR; the plan adds no design. The example is a fictional Go CLI (module `example.com/sundial`, own `go.mod` → invisible to every enclosing `./...` sweep) carrying a complete `.awf/` adoption with everything enabled and all rendered output committed. `./x sync` re-renders it with a source-built binary; `./x check` gates it (drift + invariants + zero advisory notes + `go test`). A wiring test in `internal/project` pins the mechanism and backs the ADR's three invariant slugs.

**Tech stack:** Go 1.26 (example module is stdlib-only); bash (`x`); awf run from source.

**File structure:**

- Created: `examples/sundial/{go.mod, x, README.md, cmd/sundial/main.go, internal/almanac/{almanac.go,almanac_test.go}, internal/schedule/{schedule.go,schedule_test.go}, .githooks/{pre-commit,commit-msg,pre-push}}`; the example's `.awf/` tree (init-scaffolded + hand-authored sidecars/parts listed per task) and all rendered output; `examples/sundial/docs/decisions/000{1,2,3}-*.md`, `examples/sundial/docs/plans/2026-07-07-decimal-degrees-cli.md`; `internal/project/example_wiring_test.go`; `.awf/parts/working-with-awf/overview.md`.
- Modified: `x`, `README.md`, `changelog/CHANGELOG.md`, `.awf/agents-doc.yaml`, `.awf/docs/parts/testing/tiers.md`, `.awf/docs/parts/development/command-runner.md`, `.awf/docs/parts/architecture/components.md`, `.awf/domains/parts/{tooling,rendering}/current-state.md`, `docs/decisions/0090-*.md` (status flip), plus re-rendered repo docs via `./x sync`.
- Deleted: none.

**Conventions for every phase:** run commands from the repo root unless a task says otherwise. Every phase ends with `./x gate` green and a commit (scope `tooling` unless stated). `EX=examples/sundial` abbreviates paths in prose; write real paths in files.

---

## Phase 1 — fictional project scenery

- [ ] Create `examples/sundial/go.mod`:

  ```
  module example.com/sundial

  go 1.26
  ```

- [ ] Create `examples/sundial/cmd/sundial/main.go`:

  ```go
  // Command sundial prints this week's approximate sunrise and sunset times
  // for a location given as decimal degrees.
  package main

  import (
  	"fmt"
  	"os"
  	"strconv"
  	"time"

  	"example.com/sundial/internal/almanac"
  	"example.com/sundial/internal/schedule"
  )

  func main() {
  	if len(os.Args) != 3 {
  		fmt.Fprintln(os.Stderr, "usage: sundial <latitude> <longitude>")
  		os.Exit(2)
  	}
  	lat, latErr := strconv.ParseFloat(os.Args[1], 64)
  	lon, lonErr := strconv.ParseFloat(os.Args[2], 64)
  	if latErr != nil || lonErr != nil {
  		fmt.Fprintln(os.Stderr, "sundial: latitude and longitude must be decimal degrees")
  		os.Exit(2)
  	}
  	fmt.Print(schedule.Week(almanac.Location{Latitude: lat, Longitude: lon}, time.Now()))
  }
  ```

- [ ] Create `examples/sundial/internal/almanac/almanac.go`:

  ```go
  // Package almanac approximates sun events from a location and a date. The
  // cosine day-length model (ADR-0001) trades accuracy for zero dependencies:
  // good enough to plan a walk, wrong for navigation.
  package almanac

  import (
  	"math"
  	"time"
  )

  // A Location is a point on Earth in decimal degrees.
  type Location struct {
  	Latitude  float64
  	Longitude float64
  }

  // Day describes the sun events of one calendar day at a location.
  type Day struct {
  	Date    time.Time
  	Sunrise time.Time
  	Sunset  time.Time
  }

  // Sun returns the approximate sunrise and sunset for the location on the
  // given date. Polar day and night collapse to a full- or zero-length day
  // rather than an error (ADR-0001).
  func Sun(loc Location, date time.Time) Day {
  	daylight := dayLength(clampLatitude(loc.Latitude), date.YearDay())
  	noon := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, date.Location()).
  		Add(-time.Duration(loc.Longitude * 4 * float64(time.Minute)))
  	half := daylight / 2
  	return Day{Date: date, Sunrise: noon.Add(-half), Sunset: noon.Add(half)}
  }

  // clampLatitude bounds latitude to [-90, 90] so the model never leaves the
  // domain of math.Acos; out-of-range input degrades to the pole (ADR-0001).
  // invariant: almanac-clamped-latitude
  func clampLatitude(lat float64) float64 {
  	return math.Max(-90, math.Min(90, lat))
  }

  // dayLength approximates daylight duration via the cosine model: the
  // day/night terminator angle follows the seasonal solar declination.
  func dayLength(lat float64, yearDay int) time.Duration {
  	decl := -23.44 * math.Cos(2*math.Pi*float64(yearDay+10)/365)
  	x := -math.Tan(lat*math.Pi/180) * math.Tan(decl*math.Pi/180)
  	switch {
  	case x <= -1:
  		return 24 * time.Hour
  	case x >= 1:
  		return 0
  	}
  	return time.Duration(24 * math.Acos(x) / math.Pi * float64(time.Hour))
  }
  ```

- [ ] Create `examples/sundial/internal/almanac/almanac_test.go`:

  ```go
  package almanac

  import (
  	"testing"
  	"time"
  )

  func TestClampLatitude(t *testing.T) {
  	for _, tc := range []struct{ in, want float64 }{
  		{in: 91, want: 90},
  		{in: -120, want: -90},
  		{in: 52.5, want: 52.5},
  	} {
  		if got := clampLatitude(tc.in); got != tc.want {
  			t.Errorf("clampLatitude(%v) = %v, want %v", tc.in, got, tc.want)
  		}
  	}
  }

  func TestSunPolarNightCollapses(t *testing.T) {
  	winter := time.Date(2026, time.December, 21, 0, 0, 0, 0, time.UTC)
  	day := Sun(Location{Latitude: 89}, winter)
  	if !day.Sunrise.Equal(day.Sunset) {
  		t.Errorf("polar night must collapse to a zero-length day, got %v-%v", day.Sunrise, day.Sunset)
  	}
  }
  ```

- [ ] Create `examples/sundial/internal/schedule/schedule.go`:

  ```go
  // Package schedule renders almanac days as a plain-text weekly table.
  package schedule

  import (
  	"fmt"
  	"strings"
  	"time"

  	"example.com/sundial/internal/almanac"
  )

  // Week renders seven days of sun events starting at from, one row per day.
  func Week(loc almanac.Location, from time.Time) string {
  	var b strings.Builder
  	for i := 0; i < 7; i++ {
  		day := almanac.Sun(loc, from.AddDate(0, 0, i))
  		fmt.Fprintf(&b, "%s  rise %s  set %s\n",
  			day.Date.Format("Mon 2006-01-02"),
  			day.Sunrise.Format("15:04"),
  			day.Sunset.Format("15:04"))
  	}
  	return b.String()
  }
  ```

- [ ] Create `examples/sundial/internal/schedule/schedule_test.go`:

  ```go
  package schedule

  import (
  	"strings"
  	"testing"
  	"time"

  	"example.com/sundial/internal/almanac"
  )

  func TestWeekHasSevenRows(t *testing.T) {
  	out := Week(almanac.Location{Latitude: 52.5, Longitude: 13.4},
  		time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC))
  	if got := strings.Count(out, "\n"); got != 7 {
  		t.Errorf("Week rendered %d rows, want 7", got)
  	}
  }
  ```

- [ ] Create `examples/sundial/x` and `chmod +x` it:

  ```bash
  #!/usr/bin/env bash
  # Command runner for sundial — the single entry point for repo tasks.
  # The awf verbs run the pinned release: .awf/bootstrap.sh resolves (or
  # fetches and verifies) it and prints the binary path on stdout.
  set -euo pipefail

  cmd="${1:-}"
  shift || true

  awf_bin() { bash .awf/bootstrap.sh; }

  case "$cmd" in
    gate)
      # `full` is accepted for pre-push hook compatibility; no slower tier exists.
      go test ./...
      go vet ./...
      ;;
    test)
      go test ./... "$@"
      ;;
    sync | check | invariants | audit | commit-gate | new)
      "$(awf_bin)" "$cmd" "$@"
      ;;
    *)
      echo "usage: ./x <gate [full]|test|sync|check|invariants|audit|commit-gate|new>" >&2
      exit 2
      ;;
  esac
  ```

- [ ] Create `examples/sundial/README.md`:

  ```markdown
  # sundial — the awf example adopter

  `sundial` is a small fictional Go CLI that prints a week of approximate sunrise and
  sunset times:

      go run ./cmd/sundial 52.5 13.4

  The fiction is scenery. This directory's real purpose is to be a **complete,
  committed example of an awf adoption** — every catalog skill, agent, and doc
  enabled, domains with declared territories, authored convention parts, three ADRs,
  a plan, and every rendered file checked in. It doubles as the awf repository's
  rendered-output quality oracle (its ADR-0090): the enclosing `./x sync` re-renders
  this directory from awf's source on every template change, and the enclosing
  `./x check` fails on drift, invariant findings, or any advisory note — so what you
  see here is provably what awf produces.

  ## What is what

  - `.awf/` — the authored config tree: `config.yaml`, sidecars, convention parts.
    This is the input; everything marked GENERATED is rendered from it.
  - `AGENTS.md`, `CLAUDE.md`, `.claude/`, most of `docs/` — rendered output. Never
    edit these; change `.awf/` and run `./x sync`.
  - `docs/decisions/`, `docs/plans/` — the fiction's hand-written workflow artifacts
    (`ACTIVE.md` is generated).
  - `cmd/`, `internal/` — the fictional Go module (`example.com/sundial`),
    deliberately separate from the awf module.
  - `.githooks/` — illustrative hook wiring. This directory is not a git repository,
    so the stubs can never fire here; in a real adoption, activate them with
    `git config core.hooksPath .githooks`.
  - `x` — the fiction's command runner. Its awf verbs fetch the release pinned in
    `.awf/bootstrap.sh`; inside this repository, the enclosing `./x sync` and
    `./x check` use awf built from source instead.

  ## Regenerating

  From the repository root: `./x sync` re-renders this directory along with the
  repo's own tree, and `./x check` gates it — drift, invariants, zero advisory
  notes, and this module's `go test ./...`.
  ```

- [ ] Create the three `examples/sundial/.githooks/` stubs and `chmod +x` them. `pre-commit`:

  ```bash
  #!/usr/bin/env bash
  # Illustrative wiring: in a real adoption, activate with
  #   git config core.hooksPath .githooks
  # This example directory is not a git repository, so this stub never fires here.
  exec bash .awf/hooks/pre-commit.sh "$@"
  ```

  `commit-msg` and `pre-push`: identical apart from the payload name (`.awf/hooks/commit-msg.sh`, `.awf/hooks/pre-push.sh`).

- [ ] Verify: `(cd examples/sundial && go test ./... && go vet ./... && gofmt -l .)` — tests pass, vet clean, gofmt prints nothing. `go build ./...` at the repo root still succeeds and `! go list ./... | grep -q sundial` exits 0 (module isolation).
- [ ] `./x gate` green; commit: `feat(tooling): add the sundial example scenery` (body: first slice of ADR-0090 — the fictional module the example adoption wraps; adoption follows).

## Phase 2 — adopt awf in the example

- [ ] Build the source binary once for this phase: `go build -o /tmp/awf-0090 ./cmd/awf`.
- [ ] Write `/tmp/sundial-answers.yaml`:

  ```yaml
  gateCmd: ./x gate
  gateCmdFull: ./x gate full
  checkCmd: ./x check
  commitGateCmd: ./x commit-gate
  testCmd: ./x test
  commitScopes: almanac,schedule,cli,docs
  activeMdRegenCmd: ./x sync
  invariantTestPath: ./internal/...
  skills: adr-lifecycle,brainstorming,bugfix,debugging,executing-plans,proposing-adr,refactor-coupling-audit,retrospective,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,roadmap-graduation,subagent-driven-development,tdd,writing-plans
  docs: architecture,debugging,development,glossary,pitfalls,roadmap,testing
  ```

- [ ] `(cd examples/sundial && /tmp/awf-0090 init --answers /tmp/sundial-answers.yaml)` — expect `scaffolded …/.awf/config.yaml`, `awf sync: done`, and stub notes (they clear in Phase 3). The scaffolded config gets `prefix: sundial` from the directory name; `bootstrap.enabled: true` and `hooks.enabled: true` are init defaults.
- [ ] `(cd examples/sundial && /tmp/awf-0090 add domain almanac && /tmp/awf-0090 add domain cli)`.
- [ ] Create `examples/sundial/.awf/domains/almanac.yaml`:

  ```yaml
  paths:
    - internal/almanac/**
  ```

  and `examples/sundial/.awf/domains/cli.yaml`:

  ```yaml
  paths:
    - cmd/**
  ```

- [ ] Append to `examples/sundial/.awf/config.yaml` (top-level block, matching the enclosing repo's shape):

  ```yaml
  invariants:
    sources:
      - globs:
          - "**/*.go"
        marker: //
  ```

- [ ] Create `examples/sundial/.awf/docs/glossary.yaml`:

  ```yaml
  data:
    terms:
      almanac: 'the sun-event model — approximates sunrise and sunset from latitude, longitude, and day of year (`internal/almanac`)'
      cosine day-length model: 'the zero-dependency daylight approximation ADR-0001 adopts; accurate to minutes at temperate latitudes, deliberately wrong for navigation'
      solar noon: 'the moment the sun crosses the local meridian; the model centres each day''s daylight on it, shifted four minutes per degree of longitude'
      sun table: 'the seven-row plain-text schedule `sundial` prints — one row per day, `rise`/`set` columns'
  ```

- [ ] Create `examples/sundial/.awf/agents-doc.yaml`:

  ```yaml
  data:
    commands:
      - cmd: go run ./cmd/sundial 52.5 13.4
        desc: print this week's sun table for Berlin
    docMap:
      - path: README.md
        desc: what sundial is and how this example repository is generated
    invariants:
      - ref: ADR-0001
        text: '**Clamped latitude.** `almanac.Sun` clamps latitude to [-90, 90] before the day-length model; out-of-range input degrades to the pole, never to a domain error.'
      - ref: ADR-0002
        text: '**Decimal degrees only.** The CLI accepts coordinates exclusively as decimal degrees; no DMS parsing exists.'
      - kind: scopes
  ```

- [ ] `(cd examples/sundial && /tmp/awf-0090 sync && /tmp/awf-0090 check)` — check exits 0; the remaining notes are exactly the seven stub-content lines (AGENTS.md identity; architecture overview/components/data-flow/dependencies; debugging surfaces/recipes; development setup/command-runner/dependencies; pitfalls entries; roadmap ideas/deferred; testing layout).
- [ ] Verify the rendered adapter tree is not gitignored (the root `.gitignore` negations must reach the example): `git check-ignore examples/sundial/CLAUDE.md examples/sundial/.claude/skills/sundial-tdd/SKILL.md` prints nothing and exits 1.
- [ ] Stage the whole example (`git add examples/sundial`), `./x gate` green; commit: `feat(tooling): adopt awf in the sundial example` (body: full-surface enabled set per ADR-0090 Decision 2; stub sections authored next). After the commit, `git ls-files examples/sundial/.claude | head -3` is non-empty.

## Phase 3 — author the full surface (zero notes)

Every part below carries its own `##` heading where the section renders one (mirror the enclosing repo's doc parts).

- [ ] Rebuild the source binary so the phase is self-sufficient: `go build -o /tmp/awf-0090 ./cmd/awf` (idempotent; mirrors Phase 2).

- [ ] Create `examples/sundial/.awf/parts/agents-doc/identity.md`:

  ```markdown
  `sundial` is a tiny Go CLI that prints a week of approximate sunrise and sunset
  times for a latitude/longitude pair. It is also the worked example for the awf
  standard: a complete, green adoption whose rendered files are kept in sync by the
  enclosing awf repository. The fiction is deliberately small — two internal packages
  and a `main` — so the workflow artifacts around it stay legible.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/architecture/overview.md`:

  ```markdown
  ## Overview

  `sundial` is a one-binary CLI: `cmd/sundial` parses a location, `internal/almanac`
  approximates the sun events, `internal/schedule` renders them as a table. No
  persistence, no network, no configuration files.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/architecture/components.md`:

  ```markdown
  ## Components

  - **`cmd/sundial/`** — argument parsing and output; exits 2 on usage errors.
  - **`internal/almanac/`** — the cosine day-length model (ADR-0001): `Sun(Location,
    date)` returns clamped, polar-safe sunrise/sunset pairs.
  - **`internal/schedule/`** — formats seven `almanac.Day` values as the plain-text
    sun table.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/architecture/data-flow.md`:

  ```markdown
  ## Data flow

  `main` → `schedule.Week(location, today)` → seven `almanac.Sun` calls → formatted
  table on stdout. Errors exist only at the argument boundary; the model itself is
  total — polar day and night collapse to full- or zero-length days (ADR-0001).
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/architecture/dependencies.md`:

  ```markdown
  ## Dependencies

  Standard library only (`math`, `time`, `strings`, `fmt`). Keeping the model
  dependency-free is the point of ADR-0001; adding an ephemeris library would be a
  new decision.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/debugging/surfaces.md`:

  ```markdown
  ## Surfaces

  - **CLI boundary** — wrong usage or non-numeric coordinates: exit 2 with a usage
    line on stderr.
  - **Model output** — implausible times: check the latitude clamp and the
    declination term in `internal/almanac` before suspecting formatting.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/debugging/recipes.md`:

  ```markdown
  ## Recipes

  - Reproduce a suspicious table with a fixed date: call `schedule.Week` from a test
    with a `time.Date` literal — never `time.Now()` — so the case is replayable.
  - Bisect model vs formatting by printing `almanac.Sun` directly for one day.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/development/setup.md`:

  ```markdown
  ## Setup

  Go 1.26 or newer, nothing else. Clone, then `go test ./...` to confirm a green
  baseline. The pinned awf binary is fetched on demand by `.awf/bootstrap.sh`.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/development/command-runner.md`:

  ```markdown
  ## Command runner

  `./x` wraps every repo task: `gate` (tests + vet; `gate full` is identical), `test`,
  and the awf verbs `sync`, `check`, `invariants`, `audit`, `commit-gate`, `new`,
  which run the release pinned in `.awf/bootstrap.sh`.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/development/dependencies.md`:

  ```markdown
  ## Dependencies

  None beyond the Go standard library. awf is a development-time tool, not a module
  dependency — it never appears in `go.mod`.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/pitfalls/entries.md`:

  ```markdown
  ## Entries

  - **`time.Now()` in tests.** The sun table depends on the date; a test that formats
    "today" goes red twice a year at the solstices. Fix the date with `time.Date`.
  - **Longitude sign confusion.** East is positive; a flipped sign shifts solar noon
    by minutes per degree and looks like a model bug (it isn't — check the input
    first).
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/roadmap/ideas.md`:

  ```markdown
  ## Ideas

  - Golden-hour rows in the sun table (ADR-0003's proposed cache makes them cheap).
  - A `--date` flag so scripts can render any week, not just the current one.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/roadmap/deferred.md`:

  ```markdown
  ## Deferred

  - Sub-minute accuracy: needs a real ephemeris dependency, which contradicts
    ADR-0001's zero-dependency stance; revisit only with a new decision.
  - Timezone-database lookups from coordinates; the CLI trusts the system timezone.
  ```

- [ ] Create `examples/sundial/.awf/docs/parts/testing/layout.md`:

  ```markdown
  ## Layout

  Tests live beside their package (`internal/almanac`, `internal/schedule`): model
  tests pin clamping and the polar collapse; schedule tests pin table shape.
  `./x gate` runs them all with `go vet`; the invariant-backing comments under
  `./internal/...` are checked by `./x invariants`.
  ```

- [ ] Overwrite `examples/sundial/.awf/domains/parts/almanac/current-state.md`:

  ```markdown
  The almanac domain is one package, `internal/almanac`, implementing the cosine
  day-length model ADR-0001 adopted: clamped latitude, polar-safe collapse, solar
  noon shifted four minutes per degree of longitude. Accuracy is minutes, not
  seconds — a deliberate ceiling. Anything touching the declination term or the
  clamp must keep `almanac-clamped-latitude` backed.
  ```

- [ ] Overwrite `examples/sundial/.awf/domains/parts/cli/current-state.md`:

  ```markdown
  The cli domain is `cmd/sundial`: decimal-degrees argument parsing (ADR-0002) and
  the week-table print. It owns every exit code (2 for usage) and no model logic;
  ADR-0003's proposed cache would slot between it and the almanac.
  ```

- [ ] Create `examples/sundial/.awf/skills/parts/brainstorming/example-clarifying-questions.md` (a plain body-replacement part):

  ```markdown
  2. **Ask clarifying questions, one at a time.** For sundial that usually means:
     which surface moves (model, schedule, CLI), does accuracy change (ADR-0001 caps
     it at minutes), and would a new dependency arrive (ADR-0001's zero-dependency
     stance says no). Prefer multiple-choice questions where your runtime supports
     them.
  ```

- [ ] Create `examples/sundial/.awf/skills/parts/tdd/notes.md` (a `sectionDefault`-extending part):

  ```markdown
  {{=awf:sectionDefault}}

  Sundial-specific: the almanac model is pure — every model change starts with a
  failing table-driven case in `internal/almanac`, no exceptions.
  ```

- [ ] Scaffold and fill the three fictional ADRs. For each: run `(cd examples/sundial && /tmp/awf-0090 new adr "<title>")`, then overwrite the scaffolded file with the exact content below (frontmatter included; keep the scaffolded filename).

  `docs/decisions/0001-approximate-sun-events-with-the-cosine-day-length-model.md` (title: `Approximate sun events with the cosine day-length model`):

  ```markdown
  ---
  status: Implemented
  date: 2026-07-06
  supersedes: []
  retires_invariants: []
  superseded_by: ""
  tags: [model]
  related: []
  domains: [almanac]
  ---
  # ADR-0001: Approximate sun events with the cosine day-length model

  ## Context

  sundial needs sunrise/sunset times good enough to plan a walk. Real ephemeris
  computation drags in a dependency and precision nobody asked for; the CLI's whole
  value is being small and instant.

  ## Decision

  1. Day length comes from the cosine model: seasonal solar declination drives the
     day/night terminator angle; solar noon shifts four minutes per degree of
     longitude.
  2. Latitude is clamped to [-90, 90] before the model runs; garbage input degrades
     to the pole.
  3. Polar day and night collapse to full- or zero-length days — never an error.

  ## Invariants

  - `inv: almanac-clamped-latitude` — latitude is clamped to [-90, 90] before the
    day-length model; out-of-range input degrades to the pole, never to a domain
    error.
  - Textual: `internal/almanac` stays standard-library-only.

  ## Consequences

  Minutes-level accuracy at temperate latitudes; wrong near the poles and useless
  for navigation — stated in the package doc. Zero dependencies. Sub-minute accuracy
  is out of scope unless a successor decision accepts an ephemeris dependency.

  ## Alternatives Considered

  | Alternative | Why not chosen |
  |---|---|
  | NOAA solar-position algorithm | An order of magnitude more code for accuracy the use case does not need. |
  | Ephemeris library | A dependency for a toy; contradicts the CLI's instant-and-small value. |
  ```

  `docs/decisions/0002-cli-accepts-coordinates-as-decimal-degrees-only.md` (title: `CLI accepts coordinates as decimal degrees only`):

  ```markdown
  ---
  status: Implemented
  date: 2026-07-07
  supersedes: []
  retires_invariants: []
  superseded_by: ""
  tags: [cli]
  related: [1]
  domains: [cli]
  ---
  # ADR-0002: CLI accepts coordinates as decimal degrees only

  ## Context

  Coordinates arrive in many formats (DMS, cardinal suffixes, geo URIs). Every
  format the CLI accepts is parsing surface to test and document, against ADR-0001's
  small-and-instant stance.

  ## Decision

  1. `sundial <latitude> <longitude>`, both decimal degrees, parsed with
     `strconv.ParseFloat`; anything else is exit 2 with a usage line.
  2. No DMS, no cardinal suffixes, no flags for alternate formats.

  ## Invariants

  - Textual: the CLI accepts coordinates exclusively as decimal degrees; no DMS
    parsing exists.

  ## Consequences

  One trivially testable input path. Users with DMS coordinates convert them first —
  accepted friction, documented in the README usage line.

  ## Alternatives Considered

  | Alternative | Why not chosen |
  |---|---|
  | Accept DMS too | Doubles the parse surface for a conversion any map app does. |
  | Named flags (`--lat`, `--lon`) | Two positional arguments are unambiguous; flags add ceremony. |
  ```

  `docs/decisions/0003-cache-almanac-tables-per-location.md` (title: `Cache almanac tables per location`):

  ```markdown
  ---
  status: Proposed
  date: 2026-07-09
  supersedes: []
  retires_invariants: []
  superseded_by: ""
  tags: [performance]
  related: [1]
  domains: [almanac, cli]
  ---
  # ADR-0003: Cache almanac tables per location

  ## Context

  Every invocation recomputes seven days from scratch. That is fast today, but the
  roadmap's golden-hour rows would triple the per-day work, and scripted use renders
  the same location repeatedly.

  ## Decision

  1. Cache computed `almanac.Day` values per (location, date) in a small in-process
     map behind the `schedule` package.
  2. No persistence: the cache lives and dies with the process.

  ## Invariants

  - Textual: cache hits and misses return byte-identical tables.

  ## Consequences

  Golden-hour rows become cheap. A new seam between `schedule` and `almanac` to keep
  honest — the cache must never change output, only cost.

  ## Alternatives Considered

  | Alternative | Why not chosen |
  |---|---|
  | On-disk cache | State and invalidation for a CLI that runs in microseconds. |
  | Precomputed year tables | Trades startup and memory for savings nobody measured yet. |
  ```

- [ ] Create `examples/sundial/docs/plans/2026-07-07-decimal-degrees-cli.md` (the fiction's executed plan for ADR-0002; frozen since that ADR is Implemented):

  ```markdown
  # 2026-07-07 — Decimal-degrees CLI

  **Goal:** implement [ADR-0002](../decisions/0002-cli-accepts-coordinates-as-decimal-degrees-only.md): positional decimal-degree parsing in `cmd/sundial`.

  **Architecture summary:** see the ADR. One file changes; the model is untouched.

  **Tech stack:** Go 1.26, stdlib only.

  **File structure:** modified `cmd/sundial/main.go`.

  ## Phase 1 — parse and gate

  - [x] Parse `os.Args[1]`/`os.Args[2]` with `strconv.ParseFloat`; on error print
        `sundial: latitude and longitude must be decimal degrees` to stderr, exit 2.
  - [x] Print the usage line and exit 2 when the argument count is not 2.
  - [x] `./x gate` green; commit: `feat(cli): parse coordinates as decimal degrees`.

  ## Phase 2 — record

  - [x] Flip ADR-0002 to Implemented; `./x sync` regenerates `docs/decisions/ACTIVE.md`.
  - [x] Commit: `docs(docs): flip ADR-0002 to Implemented`.
  ```

- [ ] `(cd examples/sundial && /tmp/awf-0090 sync && /tmp/awf-0090 check && /tmp/awf-0090 invariants && go test ./...)` — expected: sync regenerates `ACTIVE.md` and the two domain docs; check prints **no `note:` lines** and `awf check: clean`; invariants clean; tests pass. If any note remains, the listed section's part is missing or misnamed — fix before committing.
- [ ] Stage `examples/sundial`, `./x gate` green; commit: `feat(tooling): author the sundial example's full surface` (body: zero-notes bar of ADR-0090 Decision 4 reached; parts, ADRs, plan, glossary, guide data).

## Phase 4 — wire `./x` and pin the wiring

- [ ] In `x`, replace the `sync)` branch:

  ```bash
    sync)
      # Run awf from source so the dogfooded render always matches the tree.
      go run ./cmd/awf sync "$@"
      # ADR-0090: re-render the example adopter with the same source. The example
      # is its own Go module, so build once and run with the example as cwd.
      bindir="$(mktemp -d)"
      trap 'rm -rf "$bindir"' EXIT
      go build -o "$bindir/awf" ./cmd/awf
      (cd examples/sundial && "$bindir/awf" sync)
      ;;
  ```

  and the `check)` branch:

  ```bash
    check)
      go run ./cmd/awf check "$@"
      # ADR-0090: the example adopter must be drift-free, invariant-clean, free of
      # advisory notes (the model adopter has zero smells), and its scenery green.
      bindir="$(mktemp -d)"
      trap 'rm -rf "$bindir"' EXIT
      go build -o "$bindir/awf" ./cmd/awf
      if ! out="$(cd examples/sundial && "$bindir/awf" check)"; then
        printf '%s\n' "$out"
        exit 1
      fi
      printf '%s\n' "$out"
      if printf '%s\n' "$out" | grep -q '^note: '; then
        echo "check: the example adopter has advisory notes — author the missing content or clear the smell (ADR-0090)" >&2
        exit 1
      fi
      (cd examples/sundial && "$bindir/awf" invariants)
      (cd examples/sundial && go test ./...)
      ;;
  ```

- [ ] Create `internal/project/example_wiring_test.go`:

  ```go
  package project

  import (
  	"os"
  	"strings"
  	"testing"
  )

  // ADR-0090: the committed example adopter is kept deterministic through ./x —
  // sync re-renders it from source; check drift-, invariant-, note-, and
  // test-gates it. The example is its own Go module so the enclosing ./...
  // sweeps never see it; this test pins the wiring so it cannot be silently
  // dropped.
  //
  // invariant: example-adopter-checked
  // invariant: example-zero-notes
  // invariant: example-module-isolated
  func TestExampleAdopterWiring(t *testing.T) {
  	raw, err := os.ReadFile("../../x")
  	if err != nil {
  		t.Fatalf("read x: %v", err)
  	}
  	script := string(raw)
  	for _, want := range []string{
  		`(cd examples/sundial && "$bindir/awf" sync)`,
  		`out="$(cd examples/sundial && "$bindir/awf" check)"`,
  		`grep -q '^note: '`,
  		`(cd examples/sundial && "$bindir/awf" invariants)`,
  		`(cd examples/sundial && go test ./...)`,
  	} {
  		if !strings.Contains(script, want) {
  			t.Errorf("x lost the example-adopter step %q (ADR-0090)", want)
  		}
  	}
  	if _, err := os.Stat("../../examples/sundial/go.mod"); err != nil {
  		t.Errorf("examples/sundial must stay its own Go module (ADR-0090): %v", err)
  	}
  }
  ```

- [ ] Add to `changelog/CHANGELOG.md` under `## [Unreleased]`, section `### Others` (create the section if absent, after any Breaking/Features/Bug fixes):

  ```markdown
  - The repository now carries a committed example adopter (`examples/sundial/`) — a full-surface worked example of an awf adoption, browsable in the repo and kept render-synced from awf's source by the repo's own checks (its ADR-0090).
  ```

The behavior-documenting parts land in this phase's commit — docs travel with the change (ADR-0090 Decision 7).

- [ ] `.awf/agents-doc.yaml`: append to `data.invariants`:

  ```yaml
        - ref: ADR-0090
          text: '**Example adopter checked.** `./x sync` re-renders the committed example adopter `examples/sundial` with a source-built awf, and `./x check` fails on its drift or invariant findings.'
        - ref: ADR-0090
          text: '**Example zero-notes.** The example check step fails on any `note: ` line in the example''s `awf check` output — the model adopter has no smells.'
        - ref: ADR-0090
          text: '**Example module isolated.** `examples/sundial` is its own Go module; no enclosing `./...` sweep (test, coverage, vet, lint, deadcode) includes it.'
  ```

- [ ] `.awf/docs/parts/testing/tiers.md`: append:

  ```markdown

  `./x check` — beside the gate at every commit via the pre-commit payload — also
  gates the example adopter (ADR-0090): it re-checks `examples/sundial` with a
  source-built awf (drift, invariants, zero advisory notes) and runs that module's
  `go test ./...`, the only place the example's tests execute.
  ```

- [ ] `.awf/docs/parts/development/command-runner.md`: replace the `./x sync / check / …` table row with:

  ```markdown
  | `./x sync` / `./x check` / `./x invariants` / `./x audit` / `./x commit-gate` / `./x new` | The matching `awf` subcommand, run from source. `sync` additionally re-renders the example adopter `examples/sundial` with a source-built binary; `check` additionally gates it — drift, invariants, zero advisory notes, and its module's `go test ./...` (ADR-0090). |
  ```

- [ ] `.awf/docs/parts/architecture/components.md`: append:

  ```markdown
  - **`examples/sundial/`** — the committed example adopter (ADR-0090): a fictional Go
    module (own `go.mod`, invisible to the repo's `./...` sweeps) whose full rendered
    surface is the rendered-output quality oracle — re-rendered by `./x sync`, gated
    by `./x check`. Not part of the rendered standard.
  ```

- [ ] `.awf/domains/parts/tooling/current-state.md`: append as a new final paragraph:

  ```markdown

  ADR-0090 adds the committed example adopter `examples/sundial/` — its own Go module, invisible to every `./...` sweep — as the repo's worked example and rendered-output quality oracle: `./x sync` re-renders it with a source-built binary after the repo's own sync, and `./x check` additionally runs the example's `awf check` (failing on drift or any `note: ` line — the model adopter is smell-free), `awf invariants`, and `go test ./...`. `internal/project/example_wiring_test.go` pins the wiring; `awf audit` deliberately never runs there (no `.git` at the example root).
  ```

- [ ] `.awf/domains/parts/rendering/current-state.md`: append as a new final paragraph:

  ```markdown

  The rendered-output quality bar has a deterministic review surface (ADR-0090): the committed full-surface example adopter `examples/sundial/` re-renders on every `./x sync`, so a template change lands as a reviewable rendered diff over a realistic adoption in the same commit, and a schema bump must run `awf upgrade` there before `./x check` goes green — an in-repo migration rehearsal ahead of any external adopter.
  ```

- [ ] Verify: `./x sync` (both renders plus the repo-doc re-render, ends quiet), then `./x check` — expected tail: the repo's `awf check: clean`, the example's `awf check: clean`, the example's invariants clean, and `ok  	example.com/sundial/...` test lines. `go test ./internal/project -run TestExampleAdopterWiring` passes.
- [ ] `./x gate` green; commit: `feat(tooling): gate the example adopter through x sync and check` (body: ADR-0090 Decisions 3-4 — determinism wiring, zero-notes bar, wiring test backing the three invariant slugs; the behavior docs — guide invariants, testing/development/architecture parts, domain narratives — travel in this commit).

## Phase 5 — onboarding links, then the flip

- [ ] `README.md`: insert after the `## Quickstart` section (before `## Commands`):

  ```markdown
  ## Worked example

  A complete example adopter lives in [`examples/sundial/`](examples/sundial/README.md):
  a small fictional Go CLI with every catalog artifact enabled — authored parts,
  domains, ADRs, a plan — and every rendered file committed, kept in sync by this
  repository's own checks. Browse it to see exactly what an adoption looks like on
  disk.
  ```

- [ ] Create `.awf/parts/working-with-awf/overview.md` (singleton-doc parts live under `.awf/parts/`, per the rendered doc's own edit marker):

  ```markdown
  {{=awf:sectionDefault}}

  A complete worked example lives at
  [`examples/sundial/`](../examples/sundial/README.md): a fictional adopter with the
  full catalog enabled and every rendered file committed, kept in sync from source
  by this repository's own checks (ADR-0090).
  ```

- [ ] `./x sync && ./x check` — both trees clean; the dead-link scan accepts the two new example links.
- [ ] `./x gate` green; commit: `docs(tooling): link the example adopter from the repo docs` (body: ADR-0090 Decision 7 onboarding pointers — README worked-example section, working-with-awf overview part).
- [ ] Flip `docs/decisions/0090-*.md` frontmatter `status: Proposed` → `status: Implemented`; run `./x sync` (regenerates `ACTIVE.md` and the tooling/rendering domain docs); `./x invariants` clean (the three slugs are backed by `internal/project/example_wiring_test.go` since Phase 4).
- [ ] `./x gate` green; commit: `docs(adr): flip 0090 to Implemented — example adopter shipped`.
- [ ] Terminal step: invoke `awf-reviewing-impl` over the branch's commits.
