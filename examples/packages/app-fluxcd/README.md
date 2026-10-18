# `app-fluxcd`

FluxCD, packaged from the upstream release manifests, and the provider half of the pair in [`examples/packages`](../README.md).

It requires little -- a new enough Kubernetes and an initialised cluster -- and what makes it worth reading is its `provides:` section. Every GitOps workload package downstream of it, [`operator-external-secrets`](../operator-external-secrets) being the one here, requires the `fluxcd-engine` capability and the CRDs this package establishes. The two documents together are a bundle an orchestrator can order without deploying any of it.

| File                                                 | What it is                                                                                    |
| ---------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| [`zarf.yaml`](zarf.yaml)                             | The package definition, including the `documentation:` mapping that carries the document      |
| [`docs/prerequisites.yaml`](docs/prerequisites.yaml) | What it requires, and the capability, CRDs and namespace it provides                          |
| [`resources.json`](resources.json)                   | The upstream repository, release asset and pinned tag the manifests are vendored from         |
| `upstream/`                                          | The vendored manifests and a generated `kustomization.yaml` per component -- do not hand-edit |

## The machine-maintained parts

Three things follow from the pin in `resources.json`, and two mage targets maintain them. See [the mage reference](../../../docs/dev/mage.md) and `magefiles/pkg/pkgvendor`, which is a Go port of the shell in [colonel-byte/zarf-pkg-fluxcd](https://github.com/colonel-byte/zarf-pkg-fluxcd)'s `.github/hack`.

Moving the pin is a hand edit: write the new tag into `resources.json`, then run the targets below in that order.

```sh
go run ./magefiles/core examples:vendor    # apply the pin as it stands
go run ./magefiles/core examples:images    # rediscover the image lists from the manifests
go run ./magefiles/core examples:lint
```

`examples:vendor` writes the `upstream/` trees, a `kustomization.yaml` into each, and `metadata.version` in `zarf.yaml`. `examples:images` rewrites each component's `images:` list from what `zarf dev find-images` discovers. Run them in that order when the pin moves: discovery reads the manifests that vendoring has just put there.

## Why the kustomizations are generated

Upstream's release manifests carry no namespace and no `app.kubernetes.io/version` label. The namespace has to come from somewhere, and the version label is what lets a deployed cluster answer which Flux release it is running -- `flux install` adds it from its own build, so a package that vendors the manifests has to add it too.

That is not tidiness. `requires.gitops.version` in a [`PackagePrerequisites`](../../../docs/guides/package-prerequisites.md) document is checked by reading that label off the engine's own workloads, and the controllers' own versions are a different set of numbers entirely: a document naming `>=2.9.0` is not talking about `source-controller` v1.9.5. Without the label the constraint is reported as unchecked -- honestly, and uselessly.

The generated kustomization also re-homes the images onto `ghcr.io`. Upstream names them as `fluxcd/source-controller:v1.9.5`, which resolves to Docker Hub, where an anonymous pull is rate-limited per IP -- and `zarf package create` pulls every image in the package at once.

## Nothing builds this

`zarf package create` on it reaches the network for the images. It is here to be read, linted and inspected:

```sh
zarf dev lint examples/packages/app-fluxcd
```

`zarf dev lint` warns that the images are not pinned with a digest. The tags are what upstream publishes and what `zarf dev find-images` reports, so they are what the package would actually pull; a package that ships to an air-gapped site should pin the digests.
