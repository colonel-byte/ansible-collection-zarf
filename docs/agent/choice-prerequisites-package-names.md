# Why prerequisite packages accept `name` or `names`, and how `id` groups alternatives

`PackagePrerequisites` (`colonel-byte.dev/v1alpha1`) allows each entry under `requires.packages` to specify either a single package `name` or a list of acceptable `names`. This records why `names` exists beside `name`, why they are mutually exclusive, and how grouping by `id` extends this to alternative packages that carry distinct version constraints.

## Why `names` exists beside `name`

A prerequisite package often exists under multiple published names that provide identical cluster capability. The canonical example is upstream Zarf's `init` package versus rebuilds such as `init-slim`: a downstream package requiring an initialized cluster can run on either.

The expressive alternative -- stating capabilities in `provides` and requiring a capability -- cannot be evaluated on cluster because Kubernetes and `zarf package list` track package names, versions, and components, not `PackagePrerequisites` documents. The name is the only token `zarf_package_info` can read from the cluster without out-of-band package inspection.

Declaring separate `requires.packages` entries for each alias does not work:
- Multiple requirements would mean *all* of them must be present, when the author means *any* of them satisfies the requirement.
- Attaching version constraints across multiple alternative requirements causes duplicate constraints that drift when one is updated and another is forgotten. `names` keeps the version constraint bound to the requirement itself, applying whichever matching package is found on the cluster.

## Why `oneOf` mutual exclusivity, not precedence or `anyOf`

A requirement that carries both `name` and `names` creates ambiguity:
- Under `anyOf`, a document with both keys is valid schema, forcing consumers to invent precedence rules. If the role reads `names` first, an author writing `name: cluster-init` and `names: [init, init-slim]` would have `name` silently ignored.
- Under `oneOf`, a document declaring both keys or neither key is rejected immediately at schema validation time.

Because playbooks can pass prerequisite documents directly to the `prerequisites` role without running schema validation first, the role repeats this check explicitly in `roles/prerequisites/tasks/collect.yml`:
- It asserts that no package entry defines both `name` and `names`.
- It asserts that no package entry defines neither `name` nor `names`.
- It asserts that `names` is non-empty and `name` is non-empty (`minLength=1` and `minItems=1`).

## How the role evaluates them today

In `collect.yml`, the role immediately normalizes every entry into an internal `names` list:

```jinja2
'names': item.names | default([item.name | default('', true)], true) | select() | list
```

This normalization means downstream task logic (`check_resources.yml` and `check_versions.yml`) only handles a single list structure:
- Resource checks verify that at least one name in `names` appears in deployed packages on the cluster.
- Version checks apply the constraint against whichever matching package name is deployed.

## Grouping alternatives with distinct versions via `id`

The existing `names: [a, b]` mechanism binds a single `version` constraint across all names in that list. When alternatives have different version histories -- such as two distinct storage providers satisfying a common requirement:

```yaml
requires:
  packages:
    - name: csi-rook-ceph
      version: ">=1.21.0"
      id: storage
    - name: csi-longhorn
      version: ">=1.13.0"
      id: storage
```

A single `version` field under `names: [csi-rook-ceph, csi-longhorn]` cannot express `>=1.21.0` for Rook Ceph and `>=1.13.0` for Longhorn simultaneously.

### Why `id` instead of `requires.capabilities`

`provides.capabilities` exists in the format for bundlers and orchestrators that inspect package documents ahead of time. However, on a running cluster, `zarf package list` only reports deployed package names and versions; it does not read or query capability declarations. `id` solves this by grouping concrete package names that can be checked directly against the cluster's installed package inventory, while keeping separate version ranges per package.

### Semantics of `id` grouping

- **Local scope**: `id` is scoped strictly within `requires.packages`. It groups alternative entries in that list and does not collide with other requirement types.
- **Group satisfaction**: An `id` group represents a disjunction (OR). A group is satisfied if **at least one** branch in the group has its package deployed on the cluster **and** satisfies its specific version constraint.
- **Handling multiple installed packages**: If multiple packages from the same `id` group are deployed on the cluster (e.g. both Rook Ceph and Longhorn are present), the requirement is satisfied if at least one satisfies its constraint. An obsolete or failing sibling does not fail the group if another branch meets its constraint.
- **Ungrouped requirements**: Entries without an `id` remain independent requirements that must each be present on the cluster (the existing behavior).
- **Structure per entry**: Each entry in an `id` group may use either `name` or `names`. For example, an entry in the group can still use `names: [csi-rook-ceph, csi-ceph]` if a specific branch has aliases sharing a single version constraint.
- **Schema & Go types**: `ID string` (`minLength=1`) is an optional field on `PackageRequirement`. The existing `oneOf` between `name` and `names` remains intact.
- **Error reporting**: If no branch in an `id` group is satisfied, the failure is reported as a single grouped unmet prerequisite naming all candidate packages and their constraints, rather than separate missing package failures.
