# Scorecards supply-chain security

Recomputes the repository's [OpenSSF Scorecard](https://securityscorecards.dev/viewer/?uri=github.com/colonel-byte/ansible-collection-zarf) and uploads the result to the code scanning dashboard. Borrowed from [zarf's own workflow](https://github.com/zarf-dev/zarf/blob/main/.github/workflows/scorecard.yaml).

It runs on pushes to `main`, weekly on a schedule, and on a `branch_protection_rule` event. Only the default branch is supported, so there is no pull request trigger -- which is the thing to know about it: **a workflow change can move the score without anything failing in CI**, and the published score only refreshes after a merge.

Score the working tree instead, before opening the pull request, and name the checks you ran in the description:

```sh
ver=$(gh release view --repo ossf/scorecard --json tagName -q .tagName)
curl -fsSL "https://github.com/ossf/scorecard/releases/download/${ver}/scorecard_${ver#v}_linux_amd64.tar.gz" | tar xz scorecard
./scorecard --local . --checks=Token-Permissions,Dangerous-Workflow,Pinned-Dependencies --show-details
```

`--local` reads the working tree, so it scores a change that is not yet committed. Prefer the release binary over `gcr.io/openssf/scorecard`, which is not reliably pullable. For an honest before and after, take the baseline from a clean export -- `git archive HEAD | tar -x -C "$(mktemp -d)"` -- rather than by stashing.

[`.github/AGENTS.md`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.github/AGENTS.md) records the handful of checks this repository has actually tripped: granting a permission on the job rather than at the top of the file, never taking a checkout `ref:` from the event payload, pinning every action to a full commit SHA, and passing an untrusted value through `env:` rather than into `run:`.

The workflow itself declares `permissions: read-all` at the top and takes `security-events: write` on the job to upload the SARIF, plus `id-token: write` to receive the badge. Its checkout sets `persist-credentials: false`.
