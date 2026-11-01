# TODO: reading 20 KB of prerequisites out of a package costs a multi-GB decompress

`colonel_byte.zarf.zarf_package_documentation` is how the `prerequisites` role gets a package's `docs/prerequisites.yaml`, and against a local tarball it is pathological. Extracting that document fully decompresses the whole archive to a temp directory, sha256s every file it wrote, and then copies out the one member that was wanted. For a package carrying a few GB of images, reading a 20 KB YAML document costs a few GB of decompression, a few GB of temp space, and a full checksum pass over both.

On an air-gapped management node the temp space is the real failure, not the wall clock. A node with 4 GB free cannot read the prerequisites of a 6 GB package at all, and the error it gives names a temp directory rather than the package.

The cause is upstream. [`packager.LoadPackage`](../../vendor/github.com/zarf-dev/zarf/src/pkg/packager/load.go) threads the caller's `LayerSelection` into the OCI branch only; the `"tarball"` branch drops it on the floor. So `layout.LoadFromTar` calls `archive.Decompress(ctx, tarPath, dirPath, archive.DecompressOpts{})` with no filter, `LoadFromDir` then runs `validatePackageIntegrity` over everything that landed, and only then does `GetAttachments` pull `documentation.tar` out of the extracted directory. The selection `zarf package inspect documentation` already passes in `src/cmd/package.go` is honoured for a registry and ignored for a file.

`oci://` sources are therefore already correct and are not part of this. Zarf pulls only the metadata and attachment layers from a registry; the image layers are never fetched. On a granular package `attachmentResourceLayers` in [`zoci/pull.go`](../../vendor/github.com/zarf-dev/zarf/src/pkg/zoci/pull.go) narrows that further to the layers for the requested keys, and on an archive-backed one it falls back to the single `documentation.tar` layer. Both are cheap. Nothing about the remote path needs changing, and work spent there is wasted.

## Why the obvious fix is not one

The obvious fix is "seek to the member and ignore the rest". It does not exist for the format zarf ships, and the reasons decide the whole design.

1. **tar has no index.** Unlike zip there is no central directory, so a member is found only by walking 512-byte headers from the beginning of the stream.
2. **An uncompressed `.tar` is nevertheless cheap.** Go's `archive/tar` discards member bodies with `Seek` when the underlying reader is an `io.Seeker`, which an `os.File` is. Header-sized reads plus seeks; payload bytes are never read. Any member is near-instant, including the last one.
3. **A `.tar.zst` is not.** A plain zstd stream must be decoded sequentially from byte zero, and there is no jumping to an offset. A *seekable* zstd format does exist -- a skippable frame carrying a frame index -- and zarf does not produce one. So the only available lever is stopping early, which makes the order of the members decisive.

The order is knowable. `PackageLayout.Archive` reads the layout directory and hands it to `archive.Compress`, which sorts every entry by its name in the archive. That is alphabetical.

### There are two layouts, and the newer one is better placed

[zarf-dev/zarf#5518](https://github.com/zarf-dev/zarf/pull/5518) ("rename documentation to attachments", merged 2026-10-27, already vendored here) is a schema rename, but it arrived beside the layout split that matters more. `UsesGranularResourceLayout` in [`layout.go`](../../vendor/github.com/zarf-dev/zarf/src/pkg/packager/layout/layout.go) reads `pkg.GetAPIVersion() != v1alpha1.APIVersion`, so where a package's attachments live depends on its apiVersion:

| Layout                                       | Where the document lives          | Archive order                                                                                              |
| -------------------------------------------- | --------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Granular (v1beta1 and later)                 | `attachments/prerequisites.yaml`  | `attachments/… < checksums.txt < components/… < images/… < sboms.tar < values.schema.json < zarf.yaml`     |
| Archive-backed (v1alpha1, and no apiVersion) | `documentation.tar`, a nested tar | `checksums.txt < components/… < documentation.tar < images/… < sboms.tar < values.schema.json < zarf.yaml` |

`attachments/` sorts before everything. On a granular package the document is the **first member of the archive**, there is no inner tar to decode, and a reader that stops after it has touched a few kilobytes of a six-gigabyte file. The `documentation.tar` case is the worse one -- it sits after all of `components/` -- and it is the legacy branch, which is the right way round for a design that has to carry both.

Every package in this repository is `kind: ZarfPackageConfig` with no `apiVersion`, which the comment on `UsesGranularResourceLayout` calls out explicitly as archive-backed. So the slow layout is the only one exercised here today, and the granular path has no fixture.

Both layouts share one clean stop condition: everything worth reading sorts before `images/`. A single pass that collects `attachments/*` and `documentation.tar` and aborts on the first `images/` entry covers both without knowing the apiVersion in advance -- which is simpler than branching on it.

`zarf.yaml` is the exception under either layout. It is the last member, so asking for it forces a full streaming decode no matter how the read is written. Recovering the writes and the checksum pass is worth having, but it is a temp-space fix and should not be described as a speedup.

| Source     | The document                                             | `zarf.yaml`                                        |
| ---------- | -------------------------------------------------------- | -------------------------------------------------- |
| `.tar`     | Seek to it. Near-instant.                                | Seek to it. Near-instant.                          |
| `.tar.zst` | Decode up to it, then abort. 1-2% of bytes, or far less. | Last member -- a full decode. No write, no sha256. |

The prerequisites flow does not need `zarf.yaml`. A `PackagePrerequisites` document identifies itself by `apiVersion: colonel-byte.dev/v1alpha1` and its kind -- which `_prerequisites()` in the action plugin already checks -- so the key-to-filename map in the definition never has to be consulted. That is what makes the early abort legal rather than a guess, and it matters more now: that map is spelled `documentation:` in v1alpha1 and `attachments:` in v1beta1, so not depending on it sidesteps the rename entirely.

## What a real fix is

### 1. An extractor in `internal/zarfmod`

One entry point taking the archive path and the wanted keys, returning the members decoded in memory. An attachment is kilobytes, so nothing is written to disk at any point, which is the part that un-breaks the node with 4 GB free.

One handler serves both layouts: collect any `attachments/*` member and `documentation.tar`, decode the latter's contents in memory if that is what turned up, and stop at the first `images/` entry or once every wanted key is in hand. No apiVersion branch, and a granular package exits after the very first member.

- **`.tar`**: `os.Open` into `tar.NewReader`. `Next()` seeks past each body. Stop on the same condition.
- **`.tar.zst`**: `archives.Zstd{}` into `archives.Tar{}.Extract`, with that handler reading the wanted member through `f.Open()` and returning `fs.SkipAll` when done. `mholt/archives` honours `fs.SkipAll` in its tar reader and tears the zstd stream down mid-archive.
- Deliberately **not** `archive.Decompress` with `DecompressOpts{Files: …}`. Its `filterHandler` returns nil for a non-match and never aborts, so it still decodes every byte of `images/`. It fixes the temp space and the checksum pass only -- a half-measure worth knowing about and not worth using.
- Anything that is not a local `.tar` or `.tar.zst` -- `oci://`, `https://`, a deployed package name -- falls through to the in-process zarf command tree in `libraryrunner.go`. Unchanged behaviour, already efficient.

`github.com/mholt/archives` is already vendored and already in `go.mod` as an indirect requirement. Importing it directly costs one moved line after `go mod tidy`, and `vendor/` comes out byte-identical. The tidied `go.mod` has to be committed or `check-go-mod.yaml` fails.

### 2. `zarf_package_documentation` becomes a wrapper module

Python cannot read a `.tar.zst` here. `tarfile` gained `r:zst` only in CPython 3.14, the management node runs 3.9 to 3.12, and `python3-zstandard` is neither a dependency of this collection nor installed by `containers/ansible/Dockerfile`. Putting a compiled extension on an air-gapped node to read 20 KB is worse than the problem. So the logic moves to Go and the module changes category, from a plain action plugin to one of the wrappers.

That is the expensive half, and it is why this is written down rather than done. The bill:

| File                                                                               | Change                                                                                                                                                                                                                                                                                                                                           |
| ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `internal/zarfmod/dispatch.go`                                                     | add `package_documentation` to the dispatch table                                                                                                                                                                                                                                                                                                |
| `internal/zarfmod/dispatch_test.go`                                                | `TestModulesAnswersEveryAction` -- both pinned strings                                                                                                                                                                                                                                                                                           |
| `internal/zarfmod/documentation.go`, `documentation_test.go`                       | new                                                                                                                                                                                                                                                                                                                                              |
| `internal/zarfmod/plugindocs_test.go`                                              | `TestActionPluginDocsMatchModuleParams` -- a new `wrapperModuleCases` row                                                                                                                                                                                                                                                                        |
| `internal/zarfmod/infoplugins_test.go`                                             | drop `zarf_package_documentation` from `infoModules`; its `command_parts` goes with it                                                                                                                                                                                                                                                           |
| `plugins/action/zarf_package_documentation.py`                                     | `command_parts`, `_extracted_directory`, `_read_documents`, `_prerequisites` and the `PREREQUISITES_*` constants move to Go, where the real `api/` constants are usable instead of the copies the file's own comment apologises for. The result keys `documents`, `parsed`, `files`, `prerequisites` and `directory` must be reproduced exactly. |
| `containers/ansible/Dockerfile`                                                    | the comment naming which modules are plain action plugins, plus the symlink and the dispatch assertion                                                                                                                                                                                                                                           |
| `README.md`, `docs/ansible/modules.md`                                             | the row moves from read-only to converging                                                                                                                                                                                                                                                                                                       |
| [`choice-package-documentation-module.md`](choice-package-documentation-module.md) | must be amended. It argues specifically against this, and the new reason -- that reading a document costs a multi-GB decompress -- is one it did not have.                                                                                                                                                                                       |

`roles/prerequisites/tasks/main.yml` is untouched if and only if the result keys match. The four `magefiles/pkg/gen/schema` tests are untouched; nothing here changes `api/`. `docs/SUMMARY.md` and `docs/ansible/module_zarf_package_documentation.md` come from `generate:document`.

One thing to confirm before writing any of it: whether `ZarfInfoActionBase` runs `zarf` on the controller or on the delegated node. A wrapper module runs on the node, which is where the tarball is. If the current plugin runs on the controller, that is a behaviour change to state out loud, not to slip in.

### 3. The upstream request, which is the smaller fix

Honour `LayerSelection` in `LoadPackage`'s `"tarball"` branch and pass `DecompressOpts{Files: …, SkipValidation: true}` and `IsPartial` down through `LoadFromTar`. Roughly 20 to 30 lines, after which `zarf package inspect documentation <tarball>` is fast for every zarf user and this repository needs none of section 2 -- its whole diff becomes a version bump, which it does for every zarf release anyway.

PR 5518 does not change this ask -- it touched the schema and the naming, not `LoadFromTar` -- but it strengthens it. On a granular package the fix is `Files: ["attachments/<name>"]` with no nested decompress at all, which is a cleaner patch to write and to review than one that has to reach inside `documentation.tar`.

Two things the PR has to say plainly. It is not ours to merge, and an air-gapped node runs the `zarf` binary that is *installed* rather than the one vendored here, so the win arrives on the node's upgrade cycle and not on the merge. And it recovers only the writes and the checksum pass for `inspect definition`, not the decode, because `zarf.yaml` is the last member -- a reviewer will otherwise ask why one subcommand got fast and the other did not. `inspect definition` passes no `LayerSelection` at all today (see its `run` in `src/cmd/package.go`), so that half needs the flag plumbed as well as honoured.

## How the fast path is held

The early abort rests on `archive.Compress` sorting members alphabetically. That is a zarf implementation detail, not a documented property of the format, and a release that changes it turns the fast path silently slow. So two tests, not one:

1. **Order is asserted, not assumed.** List the member names of the real fixture tarball in stream order and assert that `attachments/…` or `documentation.tar`, whichever the fixture has, precedes every `images/` entry. A zarf that reorders the archive then fails by name instead of quietly regressing.
2. **The stream aborted.** Wrap the fixture in a byte-counting `io.Reader`, run the extraction, and assert the bytes consumed are fewer than the offset of the first `images/` member, and that the document came out correct. That is the honest substitute for a multi-GB fixture: it measures the property that matters -- reading stopped early -- without a gigabyte in `testdata/`. Give it headroom with a few-KB file in `test/e2e/noncluster/testdata/zarf-inspect-fixture` named so it sorts after the attachment, as a file rather than a Go constant, per [`test/e2e/AGENTS.md`](../../test/e2e/AGENTS.md).

Four cases, not two: `.tar` and `.tar.zst` crossed with the two layouts. `test/e2e/noncluster/02_zarf_package_documentation_test.go` already builds a real package through `buildInspectFixture`, but the existing fixture has no `apiVersion` and so only ever produces the archive-backed layout. A second fixture declaring `apiVersion: zarf.dev/v1beta1` and `attachments:` is required, and it is worth having for its own sake -- nothing in this repository currently builds a v1beta1 package at all.

## A separate defect the rename surfaced

Reviewing PR 5518 turned up a bug that has nothing to do with extraction cost and should not be bundled into the fix above, but must not be lost either.

`zarf_package_inspect.py` reads `definition.get("documentation")` into its `package_documentation` result key. `inspect definition` prints the definition in the package's *own* apiVersion -- `packageForDisplay` in `src/cmd/package.go` converts to v1alpha1 or v1beta1 and prints that -- so a v1beta1 package prints `attachments:` and that lookup returns `{}`. The module reports a package with attachments as having none, silently. `roles/package_inspect` sets a `zarf_package_documentation` fact from it, and `roles/deploy_packages` compares against it.

Internally zarf has already unified the two: `api.Package` carries a single `Attachments` field and `v1alpha1/convert.go` maps the old `documentation:` key onto it. Only the printed wire form differs. So the fix is to read either key, and the naming question -- whether this collection's own parameters, result keys and role facts follow zarf to `attachments` -- is a separate decision with a deprecation cost, not something to settle in passing. Note too that `zarf package inspect documentation` is already aliased `attachments`, and that every `zarf.yaml` in `examples/packages/` and `test/e2e/` still writes `documentation:`.

## What is deliberately not in scope

- **A cache of extracted documents keyed by checksum**, which [`choice-prerequisites-package-dag.md`](choice-prerequisites-package-dag.md) asks for. Reading the archive to checksum it is the cost being avoided, keying on mtime and size is a correctness hole, and neither helps the first run or the temp space. It is a planner optimisation for the N-package DAG, and the planner does not exist yet.
- **Making `inspect definition` fast on a `.tar.zst`.** There is no cheap read of the last member of a zstd stream without a format change -- a trailing index, or hoisting `zarf.yaml` to member zero. Worth naming upstream; not worth chasing here.
- **A Python zstd reader** in any form: a stdlib backport, a bundled wheel, or shelling out to a `zstd` binary that may not be installed.
- **A second module beside the existing one.** Two modules doing one job with different performance is a trap for whoever writes the next playbook.
- **Selective extraction of `components/` or anything else.** `documentation.tar` is the whole requirement.
