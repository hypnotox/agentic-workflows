{{=awf:sectionDefault}}

awf itself keeps the `bootstrap` singleton disabled but enables the `runner` singleton with a from-source `awfInvokeCmd`, so `./awf` is the rendered awf wrapper while `./x` stays the hand-written project runner; the `examples/sundial` adopter demonstrates the same split. The rendered git-hook payloads under `.awf/hooks/` are enabled here; the executable `.githooks/` stubs delegate to them, and awf never activates hooks itself.
