# Reference packages

Two Zarf packages that declare their cluster prerequisites in the [`colonel-byte.dev/v1alpha1` `PackagePrerequisites`](../../docs/guides/package-prerequisites.md) format, as a provider and a consumer of the same capability.

| Package                                                        | Role     | What it shows                                                                                        |
| -------------------------------------------------------------- | -------- | ---------------------------------------------------------------------------------------------------- |
| [`app-fluxcd`](app-fluxcd)                                     | Provider | A `provides:` section, and the generated kustomization that makes its version readable off a cluster |
| [`operator-external-secrets`](operator-external-secrets)       | Consumer | A `requires:` section with the Flux version, controllers and CRDs a HelmRelease needs reconciled     |

Neither is built or deployed by anything in this repository -- `zarf package create` on either reaches the network. They are here to be read, linted and inspected:

```sh
go run ./magefiles/core examples:lint
```

`app-fluxcd` is packaged from the upstream flux2 release manifests, vendored from the tag pinned in its [`resources.json`](app-fluxcd/resources.json) by `examples:vendor`, with its image lists discovered by `examples:images`. `operator-external-secrets` ships its own manifests and vendors nothing. See [the mage reference](../../docs/dev/mage.md) for those targets.

Their `docs/prerequisites.yaml` documents are checked, though. `TestExampleDocumentsMatchTheTypes` in `magefiles/pkg/gen/schema` decodes each one into the Go types the JSON Schema is generated from, with unknown fields rejected, so an example written against an older shape of the format fails rather than quietly becoming the thing readers copy.

The [`colonel_byte.zarf.prerequisites`](../../docs/ansible/role_prerequisites.md) role is what consumes a document like these at deployment time, and [`ansible/examples/playbooks/zarf-prerequisites.yml`](../../ansible/examples/playbooks/zarf-prerequisites.yml) is a playbook that does.
