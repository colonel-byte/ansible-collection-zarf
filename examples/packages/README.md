# Reference packages

Three Zarf packages that declare their cluster prerequisites in the [`colonel-byte.dev/v1alpha1` `PackagePrerequisites`](../../docs/guides/package-prerequisites.md) format: two providers of the same capability, and a consumer of it.

| Package                                                  | Role     | What it shows                                                                                                                                                      |
| -------------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [`app-fluxcd`](app-fluxcd)                               | Provider | A `provides:` section, and the generated kustomization that makes its version readable off a cluster                                                               |
| [`app-argocd`](app-argocd)                               | Provider | The same capability from the other engine: a `StatefulSet` controller, its own namespace, and a vendoring source that is a tagged tree rather than a release asset |
| [`operator-external-secrets`](operator-external-secrets) | Consumer | A `requires:` section with the engine version, controllers and CRDs a HelmRelease needs reconciled                                                                 |

None of them is built or deployed by anything in this repository -- `zarf package create` on any of them reaches the network. They are here to be read, linted and inspected:

```sh
go run ./magefiles/core examples:lint
```

`app-fluxcd` and `app-argocd` are packaged from their upstreams' manifests, vendored from the tag pinned in each one's `resources.json` by `examples:vendor`, with their image lists discovered by `examples:images`. `operator-external-secrets` ships its own manifests and vendors nothing. See [the mage reference](../../docs/dev/mage.md) for those targets.

The two providers vendor from different kinds of source, which is why both are here. Flux attaches `manifests.tar.gz` to each release, so [`app-fluxcd/resources.json`](app-fluxcd/resources.json) names an `asset`. Argo CD attaches only CLI binaries and SBOMs and documents `raw.githubusercontent.com/argoproj/argo-cd/<tag>/manifests/install.yaml` instead, so [`app-argocd/resources.json`](app-argocd/resources.json) names a `tree` -- a directory in the tagged source tree. Both are a pin to a tag; only the location differs.

Their `docs/prerequisites.yaml` documents are checked, though. `TestExampleDocumentsMatchTheTypes` in `magefiles/pkg/gen/schema` decodes each one into the Go types the JSON Schema is generated from, with unknown fields rejected, so an example written against an older shape of the format fails rather than quietly becoming the thing readers copy.

The [`colonel_byte.zarf.prerequisites`](../../docs/ansible/role_prerequisites.md) role is what consumes a document like these at deployment time, and [`ansible/examples/playbooks/zarf-prerequisites.yml`](../../ansible/examples/playbooks/zarf-prerequisites.yml) is a playbook that does.
