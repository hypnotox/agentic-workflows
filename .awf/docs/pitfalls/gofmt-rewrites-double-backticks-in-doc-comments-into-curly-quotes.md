---
title: "gofmt rewrites double backticks in doc comments into curly quotes"
domains: ["tooling"]
---
Go's doc-comment normalization (gofmt since Go 1.19) treats a literal double-backtick pair in
a doc comment as the old quoting convention and rewrites it to a left curly quote (U+201C);
so a comment trying to *depict* markdown double-backtick spans gets silently mangled into
wrong typography, and restoring the backticks verbatim just re-triggers the rewrite (hit
twice on 2026-07-09 while landing ADR-0080's sweep). In a doc-comment position, spell the
construct out in words ("a double-backtick quoting span"); literal backtick pairs are only
safe inside non-doc comments or raw strings.
