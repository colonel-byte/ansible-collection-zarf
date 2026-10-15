# Where the prerequisites work departs from the shape already here

The `colonel-byte.dev/v1alpha1` `PackagePrerequisites` work adds read-only modules, a role, a Go API, a schema generator, and a vendoring layer for one of the reference packages. Most of that follows the shape already here. The things that do not are recorded here as they arrive.

[choice-zarf-info-modules](choice-zarf-info-modules.md) is the record to read first: it is where the read-only modules' Python implementation is argued for, and it names a threshold that this change crosses.

## `app-fluxcd` is vendored by a mage target, not written by hand

The reference package was first written against the Flux Operator chart, and is now the upstream flux2 release manifests, vendored from a pinned tag. `magefiles/pkg/pkgvendor` is a Go port of the three shell scripts in [colonel-byte/zarf-pkg-fluxcd](https://github.com/colonel-byte/zarf-pkg-fluxcd)'s `.github/hack`, which is where that pattern already runs.

Three simplifications against the original, each affordable because what lives here is a reference package rather than a published one: one flavor rather than `upstream` and `minimus`; one copy of the pinned tag, which the target writes into both `zarf.yaml` and the kustomizations rather than three copies that must agree; and generated kustomizations, which is what lets the version label be stamped from the pin.

The image lists are rewritten by line surgery rather than by a YAML round trip. `zarf.yaml` there is a hand-written teaching artifact full of comments, and a marshaller would reformat it and drop every one of them. The risk of that choice -- a pattern that reaches past the component it was given -- is what `TestReplaceImagesRewritesOneComponent` is for, and the blank-line case in it is a regression it already caught: a list pattern that also matched blank lines ate the separator between components, which reflowed the file on the first run and was a no-op on every run after, so the damage was invisible in the second diff.

The pin's owner is a documented manual bump -- edit `tag`, run `examples:vendor` and `examples:images` -- which is the third answer [`magefiles/AGENTS.md`](../../magefiles/AGENTS.md) allows. A target that read the upstream's latest release and wrote it back was written and then dropped: it is a reference package nothing deploys, and the pin moving without being asked is the opposite of what pinning it was for. The `renovate` annotation in `resources.json` is carried over from the repository that has Renovate configured; nothing here reads it.
