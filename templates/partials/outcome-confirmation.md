**Mandatory first-creation confirmation.** Discovery creates no effort. Analysis, exploration,
prioritization, option comparison, and selection remain discovery until one concrete non-minimal
outcome can be named. A direct concrete non-minimal request follows the same boundary. A minimal
simple fix remains effort-free. An existing effort resumes under its fixed identity and existing
validation rules without title reconfirmation only while work remains within its confirmed outcome;
a newly discovered outcome cannot silently reuse, rename, replace, or create beside that active
effort.

When no existing effort owns the outcome, propose a canonical short slug and present all three
fields:

`Outcome: <concrete non-minimal outcome>`
`Effort title: <proposed title>`
`Effort slug: <proposed-short-slug>`

Ask the user to confirm creation, then end the turn without creating an effort, memory, branch, or
managed worktree. Only a clear response in a later turn confirms all three fields and permits
`awf effort new --slug <confirmed-slug> "<confirmed-title>"`. Agreement before the three fields were
presented does not confirm them. A requested change to any field stays in discovery and receives a
revised three-field proposal; an ambiguous response receives a focused clarification about the
outcome, title, and slug.

If creation fails while the three-field proposal and its later confirming response remain available
in conversational context, report the concrete failure and recovery action and retry without another
confirmation. If context loss or session replacement makes that evidence unavailable, present and
confirm all three fields again before retrying creation.
