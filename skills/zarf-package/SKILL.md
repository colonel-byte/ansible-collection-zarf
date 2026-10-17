---
name: zarf-package
description: >
  Writes or changes a Zarf package definition -- a zarf.yaml and the manifests, charts,
  images and files its components deploy. Use when asked to create, scaffold, package,
  restructure or lint a Zarf package, when editing a zarf.yaml, or when a chart or
  manifest has to be delivered to an air-gapped cluster. For stating what a package
  needs of a cluster before it deploys, use zarf-package-prerequisites instead.
---

# Writing a Zarf package

A Zarf package is a directory holding a `zarf.yaml` and the files its components name. `zarf package create` builds that directory into a single `.tar.zst` carrying every manifest, chart and container image; `zarf package deploy` applies it to a cluster with nothing fetched from the network.

The authority on what may appear in a `zarf.yaml` is zarf's own schema and `zarf dev lint`, not this page. Put the schema line at the top of every package you write and lint before you claim it works.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/zarf-dev/zarf/main/zarf.schema.json
```

## Build it in this order

1. **Decide what the package is.** One package is one deployable thing with one version. A workload and the engine that reconciles it are two packages, because they are upgraded on different days by different people.
2. **Write `metadata`.** `name` is what `zarf package list` reports and what another package's prerequisites name it by, so it is an interface, not a label. `version` goes in the filename of the built package.
3. **Write the components.** Each one is a named, separately-skippable unit. `required: true` for the part without which the package is pointless; `required: false` for anything a cluster might not want.
4. **Name every image.** An image zarf does not know about is not in the package, and the deployment fails on an air-gapped cluster and nowhere else. `zarf dev find-images <dir>` discovers them from the charts and manifests -- run it and paste the result into each component's `images:` list rather than trusting discovery at create time.
5. **Lint**: `zarf dev lint <dir>`. It checks the schema and the practices, and it needs no cluster.
6. **Create**: `zarf package create <dir> --confirm -o <dir>`. This reaches the network for every image, so it is the step that fails on a machine that cannot.

## What a component may deploy

| Key              | For                                                                            |
| ---------------- | ------------------------------------------------------------------------------ |
| `manifests`      | Raw YAML files, or `kustomizations` zarf renders at create time                |
| `charts`         | Helm charts, from a repo, an OCI reference, a git URL or a path in the package |
| `images`         | Container images to carry into the package's registry                          |
| `repos`          | Git repositories to mirror into the package's git server                       |
| `files`          | Arbitrary files to place on a node, with an optional `shasum`                  |
| `actions`        | Commands to run at create, deploy or remove time                               |
| `dataInjections` | Files to inject into a running pod                                             |
| `only`           | Restricts the component to a cluster, a local OS or a flavor                   |

A `kustomizations` entry is rendered when the package is created, not when it is deployed, so the cluster never needs kustomize and the rendered output is what ships. Prefer it over a hand-flattened manifest when the upstream is a kustomize tree.

## The things that actually go wrong

- **An image nobody declared.** A chart's `values.yaml` names a sidecar, nothing lists it, and the package deploys everywhere except the air-gapped cluster it was built for. `zarf dev find-images` is the check, and a committed `images:` list is what makes a version bump reviewable.
- **A version that means nothing.** When a package *is* an upstream release, make `metadata.version` that release's tag. A package reading `1.0.0` while deploying Flux `v2.9.5` carries the one number nobody can act on.
- **A mutable upstream.** A chart without a `version`, or an image tagged `latest`, builds a different package every time. Pin both.
- **Comments lost to a round trip.** A `zarf.yaml` is read by people. If you generate or rewrite part of one, rewrite the lines rather than parsing and re-emitting the document, which drops every comment.
- **Namespaces.** A `manifests` entry takes a `namespace`, and a manifest carrying its own is not overridden silently -- decide which one owns it.

## Documentation that travels with the package

Zarf's top-level `documentation:` key maps a name to a path inside the package's source tree, and every file it names is packaged into a documentation layer that `zarf package inspect documentation` extracts without deploying anything.

```yaml
documentation:
  readme: README.md
  prerequisites: docs/prerequisites.yaml
```

That is how a machine-readable statement about the package travels inside it instead of living in a README nothing can parse. For a package that has to say what a cluster must already be, use the **zarf-package-prerequisites** skill.

## Reading and signing a built package

```sh
zarf package inspect definition <package>       # the zarf.yaml it was built from
zarf package inspect documentation <package>    # extract the documentation layer
zarf package inspect images <package>           # every image actually carried, which is the check
zarf package inspect sbom <package>
zarf package create <dir> --signing-key <key>   # or sign an existing one: zarf package sign
zarf package verify <package> --key <pubkey>
```

A signature is only worth having if something rejects an unsigned package: zarf's default verification is `if-possible`, so a consumer that supplies a public key and does not also demand verification accepts an unsigned package quietly.
