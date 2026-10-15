# Where the prerequisites work departs from the shape already here

The `colonel-byte.dev/v1alpha1` `PackagePrerequisites` work adds read-only modules, a role, a Go API, a schema generator, and a vendoring layer for one of the reference packages. Most of that follows the shape already here. The things that do not are recorded here as they arrive.

[choice-zarf-info-modules](choice-zarf-info-modules.md) is the record to read first: it is where the read-only modules' Python implementation is argued for, and it names a threshold that this change crosses.

## `zarf_package_documentation` renders three parameter-controlled flags, and is not a wrapper

The threshold in that record reads: *a read-only module that grows a second or third flag, or any flag whose value attaches in a way pflag cares about, wants a wrapper binary rather than a better regular expression.* `zarf_package_documentation` renders `--keys`, `--key` and `--verify=`, all three controlled by a parameter, and `--verify` is the attaching one. By the letter of it, this module wants a wrapper.

It did not get one, for three reasons.

**The threshold was already crossed, and the record had not noticed.** It says the three read-only modules "render one flag between them that any parameter controls -- `--key` on `zarf_package_inspect`". That stopped being true when `verify` was added to `zarf_package_inspect`: it renders `--key` and `--verify=` today. The threshold as written would have made that module a wrapper too, and nobody thought so at the time -- which is a sign the number was the wrong thing to count.

**What the number was standing in for is coverage, and the coverage here is better than a wrapper's.** The worry behind the threshold is spelled out plainly in that record: `TestInfoPluginDocsMatchRenderedFlags` reads flag spellings out of Python as string literals, so it sees *which* flags are named and not which parameter they are rendered for, in what order, or whether a value attaches with a space or an `=`. A wrapper's test is better because it populates a struct and renders a real command line.

But `--verify=always` -- the spelling that record calls the mistake the weak test would miss -- is covered here by something stronger than either: `TestZarfPackageDocumentationExtractsTheLayer` runs the module against a real unsigned package built by the test and asserts that zarf refused it *for having no signature*. A module that rendered `--verify always` fails that assertion, because zarf rejects the command for a surplus positional argument instead and says so. The same test covers `--keys` by extracting one key from a package carrying two, and `--output` by extracting into a destination it then stats. Every flag is exercised against a real zarf. That is what a wrapper's command-line test would have bought, and more.

`equalsFlags` in `internal/zarfmod/infoplugins_test.go` also catches the `--verify` spelling statically, in both modules, and did so before this change.

**A wrapper is the wrong artifact for this command anyway.** The three reasons that record gives for a wrapper -- a flag surface worth projecting, progress worth following, a `changed` worth signalling -- are still all absent. Extracting a documentation layer returns in under a second, has no components, and changes nothing. What a wrapper would add is a third `main` package, a third binary in every release, and a third thing an operator installs on the management node before a playbook runs, for a collection [choice-zarf-collection-removal](choice-zarf-collection-removal.md) has scheduled for deletion once ZEP-0072 resolves.

The threshold is worth restating as what it was standing in for, rather than raised: **a read-only module needs its rendered command line covered against a real zarf, or it needs to be a wrapper.** Counting flags was a proxy for that, and a bad one.

## `app-fluxcd` is vendored by a mage target, not written by hand

The reference package was first written against the Flux Operator chart, and is now the upstream flux2 release manifests, vendored from a pinned tag. `magefiles/pkg/pkgvendor` is a Go port of the three shell scripts in [colonel-byte/zarf-pkg-fluxcd](https://github.com/colonel-byte/zarf-pkg-fluxcd)'s `.github/hack`, which is where that pattern already runs.

Three simplifications against the original, each affordable because what lives here is a reference package rather than a published one: one flavor rather than `upstream` and `minimus`; one copy of the pinned tag, which the target writes into both `zarf.yaml` and the kustomizations rather than three copies that must agree; and generated kustomizations, which is what lets the version label be stamped from the pin.

The image lists are rewritten by line surgery rather than by a YAML round trip. `zarf.yaml` there is a hand-written teaching artifact full of comments, and a marshaller would reformat it and drop every one of them. The risk of that choice -- a pattern that reaches past the component it was given -- is what `TestReplaceImagesRewritesOneComponent` is for, and the blank-line case in it is a regression it already caught: a list pattern that also matched blank lines ate the separator between components, which reflowed the file on the first run and was a no-op on every run after, so the damage was invisible in the second diff.

The pin's owner is a documented manual bump -- edit `tag`, run `examples:vendor` and `examples:images` -- which is the third answer [`magefiles/AGENTS.md`](../../magefiles/AGENTS.md) allows. A target that read the upstream's latest release and wrote it back was written and then dropped: it is a reference package nothing deploys, and the pin moving without being asked is the opposite of what pinning it was for. The `renovate` annotation in `resources.json` is carried over from the repository that has Renovate configured; nothing here reads it.
