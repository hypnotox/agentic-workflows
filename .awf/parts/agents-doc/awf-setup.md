{{=awf:sectionDefault}}

awf itself disables the `bootstrap` and `runner` singletons, building from source and keeping its from-source `./x`; the `examples/sundial` adopter demonstrates the runner artifact. The rendered git-hook payloads under `.awf/hooks/` are enabled here; the executable `.githooks/` stubs delegate to them, and awf never activates hooks itself.
