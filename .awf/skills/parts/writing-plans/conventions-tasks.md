{{=awf:sectionDefault}}
- **Assert a method, never a count.** A step's expected output is a command reaching a terminal
  state (zero findings, empty diff, clean drift), not a literal figure. A count written into a plan
  is a measurement of the corpus at authoring time, and the corpus moves: a rebase, a re-render, a
  sibling ADR, or the plan's own earlier phases all shift it. When it drifts the plan does not fail
  loudly, it fails misleadingly, because an executor who reads "expected 42" and sees 47 cannot tell
  a stale plan from a broken step, and the safest-looking response (make the number match) is the
  wrong one. This bites hardest where it looks safest: a plan that opens by counting its own work
  and then derives every later figure from that total propagates one stale measurement through every
  phase. Prefer "the finding count reaches zero", "this grep returns no output", "`awf check` is
  clean". Where a magnitude genuinely helps a reader plan the work, mark it as indicative and keep
  it out of the verification step. The same rule governs prose about the repo's own shape: see the
  pitfalls entry on hard-coded counts drifting.
