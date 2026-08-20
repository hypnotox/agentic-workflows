**Authority-guided review remediation.**

**Rule.** Apply mechanical corrections directly. Apply reasoned corrections autonomously with a concise rationale. The review spine is the single semantic home of finding classification; this workflow routes that classification without redefining it.

**Flexible details.** Ambiguity, competing clean options, severity, structural character, or survival after an earlier correction describes a finding. None transfers the choice to the user.

{{if eq .profile "core"}}**Stop when.** A finding is a user decision only when every viable correct remediation would contradict or change the approved boundary; cite the affected authority. Route a new material decision or changed boundary through brainstorming and wait at its outline approval boundary. Resolve competing clean options inside the approved boundary as implementation detail.

**Required evidence.** After a reasoned fix or user-approved ruling, retain exactly one fresh verify-pass dispatch. Diagnose residual findings under the same boundary, apply authority-preserving corrections, run applicable verification, and report each disposition. Do not dispatch another same-artifact review loop. A consensus deviation remains a user decision.{{else}}**Stop when.** A finding is a user decision only when every viable correct remediation would contradict or change a settled user-approved design or durable decision, or would require an unauthorized change to an active project rule; cite the affected authority.

A new material decision or changed approved boundary follows the brainstorming route before ADR mutation and pauses at brainstorming's pre-artifact outline approval boundary. Competing clean options inside approved durable boundaries remain implementation detail for this workflow to resolve; they are not the unresolved design fork named by an implementation stop condition.

**Required evidence.** After a reasoned fix or user-approved ruling, retain exactly one fresh verify-pass dispatch. Diagnose every residual finding under the same boundary, apply authority-preserving mechanical and reasoned corrections, run applicable verification, and report each disposition. Do not dispatch another same-artifact review loop.

A plan correction that would contradict linked ADR authority returns to ADR amendment and independent review before ordinary plan review starts again. A consensus deviation remains a user decision.{{end}}
