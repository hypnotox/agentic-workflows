{{=awf:sectionDefault}}

The rendered `commit-msg` payload makes older-format ADR merge authorization definitive only after Git exposes the assembled index, incoming parents, and final message. A refusal preserves the merge for trailer correction and `git commit` retry; `pre-merge-commit` continues to check only its earlier staged evidence.

A complete worked example lives at
[`examples/sundial/`](../examples/sundial/README.md): a fictional adopter with the
full catalog enabled and every rendered file committed, kept in sync from source
by this repository's own checks (ADR-0090).
