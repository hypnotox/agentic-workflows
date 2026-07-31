Breadth is ordered targeted < bounded < broad. targeted locates one declaration, implementation, file, or exact fact; bounded investigates within a named symbol, package, component, or subsystem; broad searches across the project search universe, including relevant source, tests, documentation, decisions, and workflow artifacts.

Treat the selected breadth as an adaptive maximum: start with the cheapest targeted lookup, widen only when evidence requires it, and never widen beyond the selected maximum. If the boundary is exhausted, report that explicitly.

For broad searches, the project search universe is tracked files plus non-ignored untracked working-tree files under the current repository root. Include tracked generated and vendored files. Exclude ignored files, .git, nested repositories, and external dependencies unless the task explicitly brings one of those surfaces into scope.
