# TODO: skip an item that names no components

`colonel_byte.zarf.deploy_packages` decides whether to deploy an item by comparing the components the item asks for against the components `zarf package list` reports for that package. An item that names no `components` is deployed every run, because there is nothing to compare.

This is written down rather than fixed because the fix is not the small one it looks like, and because the way it is currently wrong is the safe way: an item that deploys when it did not need to wastes time, and an item that skips when it was needed leaves a cluster the operator believes is converged and is not. Anything done here has to keep that asymmetry.

## What this page said before, and why it was wrong

The first version of this page said an item naming no components "is always deployed". It was not: it **killed the play**, part-way through the stack, with the earlier items already deployed.

Both the init and the deploy task rendered `components` as

```yaml
{{ pkg_item.components | join(',') if (pkg_item.components is iterable and pkg_item.components is not string)
   else (pkg_item.components | default(omit)) }}
```

and the shape test ran before the `| default(omit)` that was supposed to cover an absent key. Ansible's undefined marker raises `UndefinedError` from `is iterable` rather than returning false, because Jinja's iterable test only catches `TypeError`, so the error escaped the conditional entirely:

```
Error while resolving value for 'msg': object of type 'dict' has no attribute 'components'
```

The fix is `pkg_item.components is defined` leading the test, and Jinja's `and` short-circuits so nothing after it is reached. What is worth keeping from the episode is why it survived review: every item in `ansible/examples/playbooks/zarf-deploy-packages.yml` and in `test/e2e/cluster/testdata/deploy-packages-role.yml` names components, so no test ever rendered the branch, and the page documenting the limitation described a behaviour nobody had run.

**The lesson for whoever picks this up:** the omitted-`components` path needs a test before it needs a feature. A check-mode run of the role against the noncluster stub exercises every expression in it in seconds, with no cluster and no network -- that is where this belonged.

## Where the behaviour is

`ansible/colonel_byte/zarf/roles/deploy_packages/tasks/deploy_package.yml`, the `Determine if deployment should be skipped` task:

```yaml
_components_already_present: >-
  {{ (_required_components | length > 0) and
     (_required_components | difference(_deployed_components) | length == 0) and
     (_excluded_components | intersect(_deployed_components) | length == 0) }}
```

`_required_components` is the item's `components` with the `-`-prefixed exclusions removed. The `| length > 0` guard is the whole of this issue: with no components named, the expression is false and the item deploys.

The `_deployed_version` clause beside it is a separate rule and not part of this issue: it refuses to skip when the cluster's version of the package differs from the one the item carries, so that an upgraded tarball whose component names did not change is not mistaken for what is already deployed.

## What a fix has to work out

The question is what zarf would deploy for a package with no `--components`, and that is `selectOptionalComponent` and the loop above it in `vendor/github.com/zarf-dev/zarf/src/pkg/packager/filters/deploy.go`. Non-interactively -- which is what the modules are, they pass `--confirm` -- the selected set is:

- every component that is not optional, and
- every optional component carrying `default: true`, and
- for each component *group* of more than one, the member carrying `default: true`; a group with no default is an error rather than an empty selection.

Three things make that hard to rebuild from the inspected definition:

| Obstacle                                                     | Why it bites                                                                                                                                                                                                                                                                                                                                                                    |
| ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The two package schemas invert the field                     | `v1alpha1` components carry `required *bool`, and absent means **optional** -- see `IsRequired()` in `vendor/github.com/zarf-dev/zarf/src/api/v1alpha1/component.go`. `v1beta1` components carry `optional bool`, and absent means **required** -- `vendor/github.com/zarf-dev/zarf/src/api/v1beta1/component.go`. A rule written for one schema silently inverts on the other. |
| `zarf package inspect definition` emits the authored version | The role reads `_inspect_res.definition`, which is whichever apiVersion the package was written in, so both shapes reach the role and it would have to branch on `definition.apiVersion`.                                                                                                                                                                                       |
| `default: true` and groups                                   | An optional component is selected when it is `default`, so "the required ones" is not the answer either. Groups need the default-member rule on top.                                                                                                                                                                                                                            |

A Jinja expression that got any of those wrong would report a package as fully deployed when it was not, which is the failure direction this must not have.

## If it is picked up

Do not write it as one more expression in `_components_already_present`. Two routes, in order of preference:

1. **Ask zarf.** The honest answer is the one zarf itself computes. There is no `zarf package deploy --dry-run` that prints the selection today; if one appears, or if the filter is exposed through a library call the wrapper binary could make, that is the fix -- the wrapper already links zarf in, so a module parameter that returned the selection would put the computation in the one place that cannot drift from it.
2. **Resolve it in the inspect step, in Go.** `zarf_package_inspect` is a Python action plugin today, so this would mean a new parameter on a wrapper module rather than on it. The same objection applies to anything that reimplements the filter -- it has to be the filter, not a copy of it.

Either way it needs a test per schema version, over a package with an optional-and-default component and over one with a group. `test/e2e/noncluster/` is where that belongs: the stub serves `inspect`, so no cluster is needed.

Until then: **naming the components in the item is what turns drift detection on**, and that is what `docs/ansible/roles.md` and the role's `argument_specs.yml` say.
