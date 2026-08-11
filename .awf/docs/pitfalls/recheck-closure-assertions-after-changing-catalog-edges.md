---
title: "Recheck closure assertions after changing catalog edges"
domains: ["rendering"]
tags: ["kind-descriptor", "catalog-derived-tests", "verification-discipline"]
---
After adding a structural catalog edge such as `RequiresAgent`, follow every fixture and
closure-test failure and re-express the affected expectation without weakening what the test
was meant to prove. A passing open-path fixture does not settle closure semantics.
