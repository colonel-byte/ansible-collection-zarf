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

## `ZarfInfoActionBase` runs a list of commands, for one module

`zarf_cluster_info` asks the cluster four questions -- the Kubernetes version, the CRDs, the StorageClasses and the Deployments -- and no amount of flags makes them one kubectl command. The base class ran exactly one command and handed its stdout to `interpret`.

The alternatives were worse than extending it:

- **Four modules.** A prerequisite check would become four tasks that can each half-fail, and the role's job would turn into assembling them.
- **One command.** `kubectl get crd,storageclass,deployments -A -o json` is one call and megabytes of answer: a CRD object carries the full OpenAPI schema of every version of its resource, and a cluster with a few operators on it answers with tens of megabytes, all of which would land in a registered variable and then in a fact. The version is a separate call regardless.
- **Overriding `run()` in the subclass.** That is a copy of the binary resolution, the environment handling and the failure reporting, which is the whole of what the base class is for.

So the base gained `zarf_argvs` and `interpret_outputs`, and they are the only path it runs. `zarf_argvs` defaults to the single command `zarf_argv` returns, and `interpret_outputs` defaults to handing that command's stdout to `interpret`, so the four single-command modules are unchanged and did not have to declare that they are. Overriding the pair is the whole declaration: there is no flag saying which shape a module is, and a subclass that overrode one of the two and not the other would fail its task on the first run rather than quietly run one command and report as though it had run four.

Two consequences worth knowing:

- `command_parts` in `zarf_cluster_info` returns the *first* command, and `command_parts_all` returns all four. `TestInfoPluginDocsMatchRenderedFlags` was taught to read both and to read every match rather than the first, because otherwise three of the four commands' flags would be invisible to the one test that looks at them -- which is the state that file exists to prevent.
- A failed query names itself in the result's `query` key. A message naming only the command line leaves the reader to work out which of four kubectl calls that was.

## The role checks one comparator, and names what it cannot evaluate

`requires.kubernetes.version` is a SemVer range in the format. The role evaluates one comparator and one version, which covers `>=1.30.0`, and reports anything else -- a compound range like `>=1.30.0 <1.34.0` -- in `zarf_prerequisites_skipped`.

Evaluating a real range needs a semver library. `Masterminds/semver` is already a dependency of this repository and is no use here: the check runs in Jinja, inside a role, on a management node. The options were a half-parse that silently checks the first comparator and calls it satisfied, or naming the constraint as unevaluated. The second is the only honest one, and it is why `zarf_prerequisites_skipped` exists as a returned fact rather than as a log line -- a caller can assert on it, and the example playbook prints it on successful runs.

`requires.storage.accessModes` is in the same list for a different reason: what a provisioner will bind cannot be known without creating a PersistentVolumeClaim, which is not a thing a read-only check does.

A role that reported success for a document it mostly ignored would be worse than no role, so the rule is: anything not checked is named.

## Where this diverges from the proposal

[`choice-zarf-ansible-module`](choice-zarf-ansible-module.md) says it is the one place the proof deliberately diverges from ZEP-0072. This is the second, for the prerequisites proposal, and the divergences are small:

| The proposal                                                              | What is here                                                                                                                                                       |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Validate from the `colonel-byte.dev/requires-fluxcd` annotation           | Validate from the document. The annotation is kept as a cheap hint readable from `inspect definition` alone, and the document is the source of truth               |
| Read the Flux version from a controller's `app.kubernetes.io/version`     | The same label, but the packaged manifests have to *set* it -- upstream's do not, so the generated kustomization adds it. Without that the constraint is unchecked |
| `kubernetes.core.k8s_info` for the cluster reads                          | `zarf tools kubectl`, so the collection's only requirement stays the zarf binary it already needs. See `zarf_state_info`, which already did this                   |
| `zarf_package_inspect` returns `package_documentation` and that is enough | It does, and it is not enough: a mapping of names to paths inside a tarball is not readable from a playbook, which is why `zarf_package_documentation` exists      |
| A `cluster-init` prerequisite package name                                | `init`, which is what `zarf package list` reports the init package as. A name in that list is only useful if it is the name a consumer can look up                 |

The first row is the one that matters. An annotation is a duplicate of a fact in the document, and a check that reads the duplicate is a check that passes when the two disagree. Keeping both is deliberate -- they answer different questions, and the asymmetry is stated where they are written -- but the document wins.

## `app-fluxcd` is vendored by a mage target, not written by hand

The reference package was first written against the Flux Operator chart, and is now the upstream flux2 release manifests, vendored from a pinned tag. `magefiles/pkg/pkgvendor` is a Go port of the three shell scripts in [colonel-byte/zarf-pkg-fluxcd](https://github.com/colonel-byte/zarf-pkg-fluxcd)'s `.github/hack`, which is where that pattern already runs.

Three simplifications against the original, each affordable because what lives here is a reference package rather than a published one: one flavor rather than `upstream` and `minimus`; one copy of the pinned tag, which the target writes into both `zarf.yaml` and the kustomizations rather than three copies that must agree; and generated kustomizations, which is what lets the version label be stamped from the pin.

The image lists are rewritten by line surgery rather than by a YAML round trip. `zarf.yaml` there is a hand-written teaching artifact full of comments, and a marshaller would reformat it and drop every one of them. The risk of that choice -- a pattern that reaches past the component it was given -- is what `TestReplaceImagesRewritesOneComponent` is for, and the blank-line case in it is a regression it already caught: a list pattern that also matched blank lines ate the separator between components, which reflowed the file on the first run and was a no-op on every run after, so the damage was invisible in the second diff.

The pin's owner is a documented manual bump -- edit `tag`, run `examples:vendor` and `examples:images` -- which is the third answer [`magefiles/AGENTS.md`](../../magefiles/AGENTS.md) allows. A target that read the upstream's latest release and wrote it back was written and then dropped: it is a reference package nothing deploys, and the pin moving without being asked is the opposite of what pinning it was for. The `renovate` annotation in `resources.json` is carried over from the repository that has Renovate configured; nothing here reads it.
