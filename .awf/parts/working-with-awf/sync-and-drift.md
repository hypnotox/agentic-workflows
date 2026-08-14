{{=awf:sectionDefault}}

`awf check` also verifies that generated artifacts are indexed. Git ignore rules do not satisfy this requirement: if a global ignore hides a generated file, add it explicitly with `git add -f <path>`.
