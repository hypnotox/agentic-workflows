{{=awf:sectionDefault}}

   Then also run the repo-local audit — `./x audit-local ${baseSha}..${headSha}` (the repo's
   own `cmd/repoaudit`, ADR-0073) over the same session range. It mirrors this same finding
   contract: an `Error` finding — for example an adopter-facing change in the range with no
   `changelog/CHANGELOG.md` `[Unreleased]` entry — blocks the review from concluding, so
   resolve it or escalate it as a user-decision item; `Warning` findings are advisory. It is
   repo-specific dev tooling, deliberately not a rule in the shipped `awf audit`, and it does
   not run the gate.
