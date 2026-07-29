---
format: current-state-v2
status: Proposed
date: 2026-07-29
---
# ADR-0176: Unambiguous Pi Subagent Model References

## Context

ADR-0173 bounded every Pi subagent model reference at "at most 256 characters" and the
implementation matches that wording: `parseExactModelReference` measures
`Array.from(value).length`, and `MODEL_REFERENCE_SCHEMA` carries `maxLength: 256` alongside the
form pattern `^[^/\s]+/[^\s]+$`. No gap exists between that decision, its plan, the current-state
claim, and the shipped code. This ADR is a new decision, not a correction.

The problem is that "characters" is not one measure. For non-ASCII input, code points, UTF-16 code
units, UTF-8 bytes, and grapheme clusters all differ, and the two validating layers need not agree
on which they use. Those layers are genuinely independent: the four subagent tool schemas validate
call arguments through `MODEL_REFERENCE_SCHEMA`, while `parseExactModelReference` validates
user-global and project-local preference values and the wizard's writes, and it never consults the
schema pattern. One is a JSON Schema keyword whose measure is fixed by the validator library; the
other is hand-written TypeScript. The regression suite pins one reading with an astral case:
`p/` followed by 254 emoji is accepted at exactly 256 code points, which is 1024 UTF-8 bytes.

ADR-0173 decision point 14 requires the tool schemas, runtime validation, extension guidance, and
regression tests to enforce the bound consistently. Today that consistency holds because two
independent implementations happen to agree on a measure, and because a library's internal choice
of measure happens to match the hand-written one. Nothing structural keeps them aligned, and a
validator upgrade that changed its `maxLength` measure would silently split the two layers apart
while every existing test stayed green.

Separately, the extension displays the literal string `inherit parent` as the model label whenever
the `model` argument is omitted, while `inherit parent` is simultaneously one of the three sentinel
values that are rejected with an omit-the-field repair. The same string therefore names both the
supported omission case and a forbidden argument. The ADR-0173 plan recorded this as unresolved and
stated a preference for `configured/default`, conditioned on tests proving the string is never
accepted or emitted as a tool argument. Those tests do not exist.

## Decision

1. Restrict both segments of a model reference to printable ASCII excluding space, the byte range
   `0x21` through `0x7E` inclusive. DEL and every character outside that range are rejected. The
   existing form requirement is unchanged: exactly one separating slash, a non-empty provider, and
   a non-empty model id.

2. Apply that restriction in `MODEL_REFERENCE_SCHEMA`'s pattern and in
   `parseExactModelReference`. Both layers must carry it, because `parseExactModelReference` is the
   sole authority for configured preference values and wizard writes and does not consult the
   schema.

3. Keep the bound at 256 and keep `maxLength: 256`. Within printable ASCII, code points, UTF-16
   code units, UTF-8 bytes, and grapheme clusters coincide exactly, so the bound becomes
   measure-independent and the schema keyword becomes an exact bound rather than one that agrees
   with the runtime by coincidence. No layer needs to state which measure it uses, because after
   this decision there is only one.

4. Reject a non-ASCII reference as `malformed`, reusing the existing rejection vocabulary and the
   existing omit-the-field repair message. Do not add a new reason code, and never normalize or
   transliterate a rejected reference.

5. Change the omitted-model display label from `inherit parent` to `configured/default`, so no
   rejected sentinel value doubles as the name of a supported case.

6. Add the test coverage the ADR-0173 plan required before that label could change: prove
   `inherit parent` is never accepted as a `model` argument by any of the four subagent tool
   schemas and never emitted as one. Convert the astral acceptance case into a rejection case, and
   pin both ends of the ASCII range so `0x21` and `0x7E` are accepted while space and DEL are not.

7. Carry the change through the authored templates to every rendered output, including the Sundial
   adopter, in the same transaction, and update the changelog entry that currently describes the
   bound as a character limit.

## State changes

- update `rendering/pi-workflows:pi-subagent-model-routing`

## Consequences

The bound stops depending on any measure, so the two validating layers agree by construction rather
than by coincidence. A future validator upgrade that changed how `maxLength` counts can no longer
split the schema layer from the runtime layer, because within the permitted charset every measure
returns the same number.

Non-ASCII model references are now rejected outright rather than merely bounded in size. This is
strictly broader than a size rule and is the real cost of the decision. It is accepted because every
model reference in use is ASCII, including all eight values in the recommended preset, and because a
`provider/model-id` is an identifier rather than human-facing prose. If a registry ever ships a
non-ASCII model id, this decision blocks it and a later ADR must revisit the charset; that is a
deliberate trade of future flexibility for present unambiguity.

Removing the `inherit parent` label removes a genuine ambiguity in which one string named both a
supported case and a forbidden argument, and the new tests close the gap the ADR-0173 plan left
open. The label is TUI-facing metadata, so no caller contract changes.

The rejection vocabulary and repair message are unchanged, so no adopter-facing error handling
changes shape. Preference files that already validate continue to validate.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep character counting and restate the measure in prose | Leaves two independent implementations agreeing by coincidence, and documents the hazard instead of removing it |
| Bound UTF-8 bytes at both layers, dropping `maxLength` | JSON Schema cannot express a byte bound, so tool-argument validation would lose its declarative length check and overlong input would only fail after schema validation passed |
| Two-stage bound: `maxLength` as a coarse character pre-filter, runtime bytes as the authority | The layers would then legitimately disagree for multi-byte input, so a reference could pass the schema and fail the runtime, which is exactly the inconsistency ADR-0173 point 14 forbids |
| Split the label change into its own record | It touches the same claim's diagnostics behaviour and the same module, and the required tests overlap; a separate record would duplicate the context for a one-string change |

## Status history

- 2026-07-29: Proposed
