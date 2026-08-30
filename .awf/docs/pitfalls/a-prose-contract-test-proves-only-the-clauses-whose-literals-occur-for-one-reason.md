---
title: "A prose-contract test proves only the clauses whose literals occur for one reason"
domains: ["rendering", "invariants"]
---
A test that backs a current-state claim about generated prose asserts substrings, and a
substring is a proof only when the clause under test is the sole reason it appears. Three
shapes break that, and ADR-0245 shipped all three past a green gate and a semantic rendering
review before focused assertion perturbation found them.

First, a literal deliberately trimmed for portability stops proving the part it trimmed.
Two prose homes shared an enumeration whose leading word is capitalized in one and
mid-sentence in the other, so the pin started after that word; the claim still asserted it,
and rewording it in the spine left the suite green. Adding a bare-token check did not fix
that either, because one reviewer body names the same word elsewhere for unrelated reasons,
so the token survives the mutation that matters. Second, replacing a sentence and asserting
only that its predecessor is gone lets the replacement be degraded toward the old behavior
without turning anything red: absence of the retired text is not presence of the new
contract. Third, pinning a bare vocabulary token that a schema block or lens list already
contains lets the entire clause that token was meant to protect be deleted while the suite
stays green.

So when a test is the declared backing for a prose claim, walk the claim clause by clause
and, for each, name a literal that exists only because that clause does, with enough left
context to bind it to its own sentence. Then degrade exactly the clause each new pin names and
require the focused test to turn red. Absence assertions, bare vocabulary tokens, and literals
trimmed for case or wrapping portability are all worth writing, but none of them backs a
claim on its own.

Related decisions: [ADR-0245](../decisions/0245-authority-guided-review-remediation.md)
