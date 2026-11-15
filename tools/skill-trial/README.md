# Skill observation trial

A small, explicitly run harness that shows how a headless Claude Code agent behaves with AWF's workflow skills on a few ordinary tasks. It exists to compare skill versions, such as the current skills against a rewrite, and is not part of `./x gate`, `./x test`, or CI.

## Prerequisites

- Claude Code CLI, authenticated, with network access. Record its version: compare runs only from the same version.
- The user-level `agentic-skills` plugin installed and enabled. Leave the plugin set unchanged between runs you compare; `run-info.txt` records it.
- `go`, `git`, `jq`, and GNU `timeout`.

## Run

From any shell, in this repository:

```sh
tools/skill-trial/run.sh -r main -l baseline-main            # all tasks
tools/skill-trial/run.sh -l rewrite a-bugfix d-integrate     # HEAD, two tasks
```

- `-r <ref>`: git ref whose `awf` is built and installed in the fixture (default `HEAD`). The build comes from `git archive`, so uncommitted changes are never included; commit what you want to trial.
- `-l <label>`: results directory name (default `<ref>-<short commit>`). Rerunning a label replaces its task results.
- Task names are the files in `tasks/` without `.md`.
- Environment: `TRIAL_MODEL` (default `claude-sonnet-5-5`), `TRIAL_MAX_TURNS` (default 80), `TRIAL_TIMEOUT` seconds per task (default 900), `TRIAL_RESULTS` (default `${XDG_CACHE_HOME:-$HOME/.cache}/awf-skill-trial`).

Each task runs for a few minutes and costs real API usage.

## What it does

1. Builds `awf` from the ref into `<results>/<label>/bin/awf`.
2. Assembles the fixture from `fixture/` (every `*.in` file with the suffix stripped, so nothing here is compiled or loaded as instructions): `tally`, a tiny Go word-count CLI with tests, an author-owned `AGENTS.md` following the `awf docs agents` example, `CLAUDE.md` importing it, and two topics. It runs `awf init`, `go test`, makes one initial commit on `main`, and confirms `./awf check` passes.
3. Makes `./awf` run the built binary offline without changing generated files: the generated bootstrap uses a cached binary at `$XDG_CACHE_HOME/awf/<version>/awf` first, so the harness seeds a private cache there and sets `XDG_CACHE_HOME` for the agent. `GOCACHE` keeps pointing at the user's Go cache.
4. For each task, copies the fixture into a fresh temporary directory and runs `claude -p` with the task prompt, a fixed model, a turn cap, a time cap, `stream-json` output, `acceptEdits` permissions with a scoped tool allowlist (file tools, skills, subagents, and `git`, `go`, `./awf`, and common read-only shell commands), no prompts (anything else is denied and recorded), no MCP servers, and no session persistence. No permission bypass is used.
5. Saves results to `<results>/<label>/<task>/`:
   - `prompt.md`, `transcript.jsonl` (the full stream), `stderr.log`, `exit-status` (124 means the time cap hit);
   - `git-state.txt` (log of all branches, status, worktrees, diff stats), `diff.patch` against the initial commit, `final-fixture.tar.gz` (the whole temporary directory, including any sibling effort worktrees);
   - `summary.md`: available skills, turns, cost, denied tools, every tool call in order (subagent calls indented), and the final message.

   `<results>/<label>/run-info.txt` records ref, commit, awf, CLI and model versions, caps, and installed plugins.

`summarize.sh <task results dir>` regenerates a summary from an existing transcript. Results are outside every repository; do not commit them.

## Tasks

The prompts read as ordinary user requests and do not mention AWF or what is observed:

- `a-bugfix`: a small routine fix (tab-separated words are miscounted).
- `b-design-choice`: a feature with a real user-visible choice (directory input: recursion, hidden files, symlinks, output shape).
- `c-multi-stage`: work spread over sessions that should leave continuity for a later pickup.
- `d-integrate`: a small feature on a branch, ending with a request to finish and merge into `main`.

## What to compare

Between two labels, task by task, read `summary.md` and the transcript where needed:

- Which skills loaded (`Skill` calls and `./awf docs` reads), in what order, and at which point in the work.
- Whether `./awf resolve` ran, with which paths, and whether the returned topics were read before editing.
- Whether the agent brought the direction or a material choice back to the user before planning or implementing (`b-design-choice` especially; in headless mode a question ends the run).
- Whether an effort was created when continuity mattered (`c-multi-stage`), and whether a worktree was used.
- Review behavior: whether an independent review ran, by which agent, and on what.
- Retrospective behavior: whether one ran and where its findings went.
- Commit behavior: whether and when it committed, and message style.
- Whether integration waited for or used the request's authorization (`d-integrate`), and whether it pushed or published anything.

## What it does not establish

Results are directional. Each label is one run per task: a small, nondeterministic sample from one harness, one model, and one fixture. Headless mode cannot answer questions, so a run that asks the user simply ends. The user-level instructions, plugins, and settings of the machine running it apply to every run and may differ elsewhere. The allowlist deliberately excludes interpreters such as `python3`, shell redirection writes, and paths outside the run directory; agents often try these first, and the denials, recorded in `summary.md`, add friction that is the same across runs but not part of the skills. A difference between two runs may be noise; rerun a task before drawing conclusions from a single divergence.
