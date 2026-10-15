# Validate Lint

Runs golangci-lint over the tree, and hadolint over `containers/ansible/Dockerfile`. Runs on every pull request and on the merge queue.

The interesting part is how the version is chosen. `golangci-lint-action` installs the **latest release** when it is given no version, so CI once ran 2.14.0 against a tree whose hooks pinned 2.13.2 -- a lint failure that reproduced nowhere locally.

So the workflow reads the version out of `.pre-commit-config.yaml` instead of carrying a pin of its own. An `awk` program finds the `golangci/golangci-lint` repo block, takes its `rev:`, and exports it as a step output the action then consumes. Leaving that block without having seen a `rev` exits rather than reporting the next repo's pin back, and an empty result fails the step with a message.

Reading rather than restating is the point: Dependabot's `pre-commit` ecosystem bumps that `rev`, and its `github-actions` ecosystem updates `uses:` references, not inputs inside a `with:` block. A second pin here would be one nothing keeps in step with the first -- the same "every pin needs an owner" rule that [`magefiles/AGENTS.md`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/magefiles/AGENTS.md) states.

The `containers` job runs hadolint. Its path is listed explicitly rather than globbed: the action's `recursive` mode globs from the repository root, which would drag in the vendored Dockerfiles under `vendor/`. hadolint checks syntax and never builds the image -- the build runs in [`test-build-containers.yaml`](test-build-containers.yaml.md).
