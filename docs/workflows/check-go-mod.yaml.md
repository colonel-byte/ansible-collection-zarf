# Check for go mod changes

Holds `go.mod`, `go.sum` and `vendor/` to each other. Runs on every pull request and on the merge queue.

The job runs `go mod tidy` and fails on a diff to `go.mod` or `go.sum`, then runs `go mod vendor` and fails on a diff to `vendor/`.

The second half is the one worth knowing about. The vendored tree is committed and every build reads from it, so a `go.mod` change that was not re-vendored leaves the two disagreeing -- and nothing local notices, because `go build` uses `vendor/` quite happily with a dependency list that no longer matches the module file. The failure only surfaces when somebody drops `vendor/` and gets a different build. This workflow is the only check that catches it, which is why a `go.mod` change is not finished until `go mod vendor` has run and the result is committed.

Dependabot's gomod updates trip it until the branch is re-vendored.
