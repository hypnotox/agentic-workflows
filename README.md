# agentic-workflows

`awf` is a generic agentic-development-workflow tool: it renders a standardised suite of Claude
Code skills, review agents, git hooks, and docs into any project from shared templates plus a
per-project `.claude/awf.yaml`, and supplies the tooling to drift-check and enforce parts of the
standard. The awf tool is a Go binary; the standard it renders is language-agnostic.

## Install

    go install agentic-workflows/cmd/awf@latest

## Use

    cd your-project
    awf init      # scaffold .claude/awf.yaml + render + activate hooks
    awf setup     # activate git hooks (core.hooksPath); run once after cloning
    awf sync      # re-render after template or config changes
    awf check     # fail on stale or hand-edited rendered output
    awf list      # show standard skills and their per-project state
    awf add tdd   # enable a standard skill

Rendered files are committed. Drift is tracked in `.claude/awf.lock`; rendered bodies carry
no generator metadata.
