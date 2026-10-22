# Why every Go module update arrives in one Dependabot pull request

[`.github/dependabot.yaml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.github/dependabot.yaml) puts the whole `gomod` ecosystem in a single group matching `*`. It used to carry seven groups by vendor family -- `k8s`, `opentelemetry`, `sigstore`, `golang`, `docker`, `cloud-kms`, `openapi` -- and anything outside them opened a pull request of its own. This is the record of why they were collapsed.

## The problem

`vendor/` is committed, and every build and CI job reads from it, because the management node this collection runs on is typically air-gapped. That makes the vendored tree the dependency list rather than a cache, and it is the reason [`check-go-mod.yaml`](../workflows/check-go-mod.yaml.md) re-vendors and fails on any diff.

Dependabot edits `go.mod` and `go.sum` and stops there. It has no notion of a vendored tree, and there is no setting that gives it one. So every `gomod` branch it opens is inconsistent on arrival, and the failures are not confined to the check that exists to catch it:

```
vendor/cloud.google.com/go/monitoring/apiv3/v2/auxiliary_go123.go:32:32: method AlertPolicyIterator.All already declared at vendor/.../auxiliary.go:31:32
vendor/github.com/containerd/typeurl/v2/types_gogo.go:24:2: cannot find module providing package github.com/gogo/protobuf/proto: import lookup disabled by -mod=vendor
```

Unit tests, both e2e suites, lint and CodeQL all fail the same way, because none of them can load a package that `vendor/` no longer holds. The fix is one command -- `go mod vendor`, committed to the branch -- and it is the same one command whether the branch moved one module or two hundred.

With seven groups, a daily schedule and a 50 pull request limit, that command had to be run by hand on up to a dozen branches a day. The queue of them is what the grouping removes.

## Why not the two alternatives

**Automating the re-vendor** means a workflow that pushes a commit to a Dependabot branch, which needs `contents: write` on a pull-request-triggered job -- a token grant that [`.github/AGENTS.md`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.github/AGENTS.md) keeps off anything a fork can trigger. A `workflow_dispatch` variant taking a pull request number avoids that, and remains the upgrade if one grouped branch a day still proves to be one too many.

**Dropping `vendor/`** is not available. The air-gapped build is the point of it.

## The cost

A grouped update is one pull request and therefore one revert. When a bump breaks something, `git revert` takes the other two hundred modules back with it, and finding which one was at fault means bisecting within the branch rather than reading the title. The previous layout bought that isolation, and it was paid for daily in re-vendoring; the grouped layout pays only when a bump actually breaks, which is rarer.

Security advisories are unaffected. Dependabot opens those outside the group regardless of this configuration, which is what the `open-pull-requests-limit: 5` headroom is for.
