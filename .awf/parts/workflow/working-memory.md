{{=awf:sectionDefault}}

Working memory is optional. A file is created only when the effort warrants durable checkpoint state, and then carries an exact `Effort: <active-effort-id>` line. A Pi handoff requires that validated file and a matching active association. Checkpoint-less Pi work continues in the current session or uses `/awf-resume-effort <effort-id>` for explicit fresh-session continuation; the runtime never mines prose or filenames for identity.

Effort identity is one-way: do not create a memory file merely to satisfy telemetry or handoff, and the ledger never stores or infers a memory path. Completed efforts must be reopened separately, and abandoned or pruned efforts cannot be resumed.
