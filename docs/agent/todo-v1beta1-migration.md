# TODO: what zarf v1beta1 costs this repository

Zarf's `v1beta1` package schema is not a rename pass. It removes eight features outright, renames or restructures roughly forty fields, inverts four booleans, and changes how imports, templating and healthchecks work. The upstream account is [v1alpha1 to v1beta1](https://docs.zarf.dev/best-practices/v1alpha1-to-v1beta1/); this page is only the part that lands on this tree, with the mapping verified against the `v1beta1` types in the zarf currently vendored here rather than against the guide alone.

**Nothing forces this yet.** A definition with no `apiVersion` is still read as v1alpha1, every package in this repository is exactly that, and v1alpha1 remains supported. The deadline is zarf v1.0.0, where the fields already carrying `[Deprecated] … will be removed in Zarf v1.0.0` in `api/v1alpha1/component.go` go away.

## The good news first: this repository dodges the brutal half

Every feature v1beta1 *removes* is one nothing here uses. Checked across `examples/packages/*/zarf.yaml`, `test/e2e/*/testdata/*/zarf.yaml`, both skills and every example playbook:

| Removed in v1beta1                  | Used here | Replacement                                            |
| ----------------------------------- | --------- | ------------------------------------------------------ |
| `variables`, `constants`            | No        | The Values model -- `values:`, `--set-values`          |
| action `setVariables`               | No        | `setValues` on an action                               |
| `components[].dataInjections`       | No        | See upstream's data-injection guide                    |
| `metadata.yolo`                     | No        | `--connected` at deploy time                           |
| `components[].group`, `.default`    | No        | `selector.flavor`; no replacement for a default member |
| `components[].import.name`          | No        | A `ZarfComponentConfig` file per component             |
| `components[].only.cluster.distros` | No        | Nothing; the field did nothing                         |
| Skeleton packages                   | No        | Published component configs                            |
| `healthChecks`                      | No        | A wait action on `actions.onDeploy.onSuccess`          |

So the migration here is a field-rename exercise over five YAML files, one Go writer that will emit invalid YAML, and one skill page that teaches the old shape. That is a much smaller job than the guide's length suggests, and it is worth saying out loud before anyone reads the mapping table and panics.

## The sequencing problem: the converter is not in the zarf we pin

Upstream's instruction is `zarf dev upgrade zarf.yaml > zarf-v1beta1.yaml`. **That subcommand does not exist in the vendored zarf.** The `dev` tree here is `generate-schema`, `inspect` (`definition`, `manifests`, `values-files`), `deploy`, `generate`, `patch-git`, `sha256sum`, `find-images`, `generate-config` and `lint`. `upgrade` ships in a later release, and [`choice-version-tracks-zarf.md`](choice-version-tracks-zarf.md) governs when this repository sees it.

Two consequences:

1. **Converting by hand now means doing work the tool will do later, and doing it worse.** Five files is small enough to hand-edit, but the tool also catches the fields nobody remembered.
2. **The tool will not preserve the comments**, and comments are most of what the files in `examples/packages/` *are*. `app-fluxcd/zarf.yaml` carries about thirty lines of explanation above `kind:` and more inline, including the paragraph telling a reader that the `documentation:` mapping is the part to copy. Budget for re-attaching every comment by hand after the converter runs, and for rewriting the ones that name a renamed field.

The order that follows from this is in [Order of work](#order-of-work) below.

## Field mapping

Verified against `vendor/github.com/zarf-dev/zarf/src/api/v1beta1/package.go` and `component.go` at the pinned version. The guide on zarf's `main` branch describes a slightly later v1beta1, so re-check before converting on a newer release.

### Package level

| v1alpha1                                                                     | v1beta1                                | Note                                                    |
| ---------------------------------------------------------------------------- | -------------------------------------- | ------------------------------------------------------- |
| *(absent)*                                                                   | `apiVersion: zarf.dev/v1beta1`         | **Mandatory.** Absent still means v1alpha1              |
| `documentation`                                                              | `attachments`                          | [zarf#5518](https://github.com/zarf-dev/zarf/pull/5518) |
| `metadata.allowNamespaceOverride`                                            | `metadata.preventNamespaceOverride`    | Boolean inverted                                        |
| `metadata.image`, `.source`, `.documentation`, `.url`, `.authors`, `.vendor` | `metadata.annotations`, same key names | An existing annotation wins                             |
| `metadata.aggregateChecksum`                                                 | `build.aggregateChecksum`              |                                                         |
| `build.terminal`                                                             | `build.hostname`                       |                                                         |
| `build.differential: true`                                                   | `build.differential` becomes an object |                                                         |
| `build.differentialPackageVersion`                                           | `build.differential.fromVersion`       |                                                         |
| `variables`, `constants`                                                     | `values.files`, `values.schema`        | A different model, not a rename                         |

### Component level

| v1alpha1                                 | v1beta1                                                          | Note                                               |
| ---------------------------------------- | ---------------------------------------------------------------- | -------------------------------------------------- |
| `required: true`                         | omit it                                                          | v1beta1 defaults to required                       |
| `required: false`                        | `optional: true`                                                 | Boolean inverted                                   |
| `only.flavor`                            | `selector.flavor`                                                | `selector` filters at **create** time              |
| `only.cluster.architecture`              | `selector.architecture`                                          |                                                    |
| `only.localOS`                           | `target.os`                                                      | `target` filters at **deploy** time                |
| `repos`                                  | `repositories`                                                   |                                                    |
| a repo URL with `@ref`                   | `url` plus `ref.branch`/`.tag`/`.commit`                         | The ref is its own object now                      |
| `images: [ "ref" ]`                      | `images: [ { name: "ref" } ]`                                    | Objects, with optional `id` and `source`           |
| *(new)*                                  | `imageArchives: [ { path, images } ]`                            | Images from a local archive rather than a registry |
| `files[].target`                         | `files[].destination`                                            |                                                    |
| `files[].shasum`                         | `files[].checksum`                                               | `sha256:`-prefixed; unprefixed still means SHA-256 |
| `files[].template`                       | `files[].enableTemplating`                                       |                                                    |
| `manifests[].kustomizations`             | `manifests[].kustomize.files`                                    |                                                    |
| `manifests[].kustomizeAllowAnyDirectory` | `manifests[].kustomize.allowAnyDirectory`                        |                                                    |
| `manifests[].enableKustomizePlugins`     | `manifests[].kustomize.enablePlugins`                            |                                                    |
| `manifests[].noWait`, `charts[].noWait`  | `skipWait`                                                       |                                                    |
| `manifests[].template`                   | `manifests[].enableTemplating`                                   |                                                    |
| `charts[].schemaValidation`              | `charts[].skipSchemaValidation`                                  | Boolean inverted                                   |
| action `maxRetries`, `mute`              | `retries`, `silent`                                              |                                                    |
| action `after`                           | prepended to `actions.onSuccess`                                 |                                                    |
| `healthChecks`                           | a wait action on `onDeploy.onSuccess`                            | Conditionless checks gain `condition: exists`      |
| *(new)*                                  | `serverSideApply` on manifests and charts                        |                                                    |
| *(new)*                                  | `stateAccess: [registryCredentials\|gitCredentials\|agentCerts]` | A component declares what cluster state it reads   |

### Charts: exactly one source, and values files become objects

This is the restructure most likely to catch someone out, because the v1alpha1 form overloaded `url` for four different kinds of source and v1beta1 refuses to.

| v1alpha1                                            | v1beta1                                               |
| --------------------------------------------------- | ----------------------------------------------------- |
| `url`, `repoName`, `version`                        | `helmRepository: { url, name, version }`              |
| git `url`, `gitPath`, `version` or `@ref`           | `git: { url, path, ref }` -- a tag, branch or commit  |
| OCI `url`, `version`                                | `oci: { url, ref: { tag } }` or `oci.ref.digest`      |
| `localPath`                                         | `local: { path }` -- version comes from `Chart.yaml`  |
| `valuesFiles: [ "a.yaml" ]`, `templatedValuesFiles` | `valuesFiles: [ { path, enableTemplating } ]`, merged |

### Imports and templating

- A component import now references a `ZarfComponentConfig` file holding **one** component, with its own `apiVersion: zarf.dev/v1beta1`. Skeleton packages are gone; published component configs replace them.
- A package template is renamed `zarf.tpl.yaml` and uses `[[ .KEY ]]` -- case-sensitive, no `.Values` prefix -- rendered by `zarf dev template` **before** `zarf package create`. Note that `test/e2e/noncluster/testdata/` has no templates today, and upstream's own fixture `src/test/packages/52-v1beta1-deploy/zarf.tpl.yaml` is the pattern to copy if one is ever needed.
- A remote component may not carry `onCreate` actions or `kustomize.allowAnyDirectory: true`, and has to be fully rendered before publishing.

## The work, file by file

### 1. `magefiles/pkg/pkgvendor` -- the only thing that breaks silently

This is the item to do first regardless of when the YAML moves, because it is the one failure no test names.

`replaceImages` in [`images.go`](../../magefiles/pkg/pkgvendor/images.go) rewrites a component's image list by regex over the raw YAML text and emits plain scalars:

```go
for _, image := range images {
    rendered.WriteString("      - " + image + "\n")
}
```

In v1beta1 an entry is an object -- `Image{Name, ID, Source}` in `api/v1beta1/component.go` -- so that writer produces a document the new schema rejects. `kustomization.go` writes an `images:` key the same way. Neither is covered by a Go test over the types: `TestExampleDocumentsMatchTheTypes` holds `docs/prerequisites.yaml` against *our* `api/`, not `zarf.yaml` against zarf's. `examples:lint` is the only check that would see it, and it needs a zarf binary, so a contributor without one gets no signal at all.

What a fix needs:

- `replaceImages` emits `      - name: <ref>` per image, and the `componentImagesPattern` regex has to tolerate the object form when it reads the existing block back.
- `kustomization.go`'s generated `images:` block matches.
- `findImagesOutput` in `images.go` currently decodes `Images []string` out of `zarf dev find-images`. Confirm what that command prints for a v1beta1 package before trusting it -- if it prints objects, the decode changes too. **This is unverified**; it cannot be checked without a zarf that can create a v1beta1 package.
- A unit test over `replaceImages` asserting the object form, so this cannot regress to scalars silently.

### 2. The five package definitions

Three reference packages and two fixtures. Mechanical, and small enough to hand-check after the converter runs.

| File                                                          | Fields needing change                                                                                |
| ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `examples/packages/app-fluxcd/zarf.yaml`                      | `apiVersion`, `documentation:`, `metadata.source`, `required: true` ×N, `manifests[].kustomizations` |
| `examples/packages/app-argocd/zarf.yaml`                      | the same set                                                                                         |
| `examples/packages/operator-external-secrets/zarf.yaml`       | the same set, minus `kustomizations` if it has none                                                  |
| `test/e2e/noncluster/testdata/zarf-inspect-fixture/zarf.yaml` | `apiVersion`, `documentation:`, `required: true`, `files[].target` → `destination`                   |
| the cluster-suite fixtures                                    | audit for the same; none use a removed feature                                                       |

Concretely, for the fixture:

```yaml
apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: inspect-fixture
  version: 1.2.3
  description: A package holding one file, for inspecting rather than deploying
attachments:
  prerequisites: docs/prerequisites.yaml
  notes: docs/notes.md
components:
  - name: a-file
    files:
      - source: payload.txt
        destination: /tmp/zarf-inspect-fixture-payload.txt
```

Note what the diff costs beyond the fields. The `yaml-language-server: $schema=` comment at the top of each example points at zarf's combined `zarf.schema.json`, which carries both versions, so it needs no change -- but the long comment in `app-fluxcd/zarf.yaml` that ends *"The part to copy for a package of your own is the `documentation:` mapping"* is prose about a renamed key and has to be rewritten, as does the fixture's comment explaining what the `documentation:` mapping is the fixture for. Grep for the old names in comments, not only in keys.

Two fixtures are worth keeping on **both** schemas rather than migrating: see [`todo-tarball-metadata-extraction.md`](todo-tarball-metadata-extraction.md), where the archive layout differs by apiVersion and both branches need a fixture. A v1beta1 package stores attachments as individual files under `attachments/`; a v1alpha1 one stores them inside `documentation.tar`. Migrating every fixture to v1beta1 would leave the legacy branch untested, and this repository will be reading v1alpha1 packages for as long as anyone is still shipping them.

### 3. `skills/zarf-package/SKILL.md` -- the largest content change

The skill teaches authoring, so a stale page actively teaches the wrong thing. It currently shows, among others:

| Line       | Teaches                                                                | Becomes                                                         |
| ---------- | ---------------------------------------------------------------------- | --------------------------------------------------------------- |
| 29         | "`required: true` for the part without which the package is pointless" | omit `required`; `optional: true` for the skippable part        |
| 50         | a `documentation:` block                                               | `attachments:`                                                  |
| 57, 66, 83 | `required: true` / `required: false` in the example                    | omitted / `optional: true`                                      |
| 68         | a `charts:` block                                                      | one of `helmRepository:`, `git:`, `oci:`, `local:`              |
| 73         | `valuesFiles:` as a list of paths                                      | a list of `{ path, enableTemplating }` objects                  |
| 104        | the prose about `required: true` and optional-component traps          | rewritten around `optional`, with the inverted default stated   |
| 131, 134   | the `documentation:` key and `zarf package inspect documentation`      | `attachments:`; the subcommand is already aliased `attachments` |

Add the mandatory `apiVersion`, and decide whether the skill teaches one schema or both. Teaching both doubles the page; teaching only v1beta1 while every example in this repository is v1alpha1 leaves the two disagreeing, and [`skills/AGENTS.md`](../../skills/AGENTS.md) has tests that hold a page to its format. That decision belongs in a `choice-` doc, not here.

`skills/zarf-package-prerequisites/SKILL.md` needs less: it is about the `colonel-byte.dev/v1alpha1` document, not zarf's schema. Only its references to the `documentation:` key and to `zarf package inspect documentation` move.

### 4. The Python plugins

One real bug, already written up in [`todo-tarball-metadata-extraction.md`](todo-tarball-metadata-extraction.md) under "A separate defect the rename surfaced", and it generalises here: `zarf package inspect definition` prints the definition **in the package's own apiVersion** (`packageForDisplay` in `src/cmd/package.go` converts to v1alpha1 or v1beta1 and prints that), so any plugin reading a key out of that YAML by its v1alpha1 name silently returns nothing for a v1beta1 package.

`zarf_package_inspect.py` does exactly that with `definition.get("documentation")`, feeding its `package_documentation` result key, the `zarf_package_documentation` fact in `roles/package_inspect`, and the drift comparison in `roles/deploy_packages`. Audit every other key the plugins read out of definition output against the two mapping tables above before assuming this is the only one.

### 5. Documentation pages

[`docs/guides/package-prerequisites.md`](../guides/package-prerequisites.md) names the `documentation:` key in six places. `skills/`, `README.md` and the generated module pages follow from their sources. Run `generate:document` after, per [`docs/AGENTS.md`](../AGENTS.md).

## What does not change

Worth recording, because the instinct is to assume a schema break of this size reaches everything.

- **The Go module flag surface needs nothing.** `internal/zarfmod` already renders `--set-values`, `--set-variables`, `--values`, `--connected`, `--retries` and `--skip-values-schema-validation`. It already spans both schemas: `--connected` is the v1beta1 replacement for `metadata.yolo` and arrived with the 0.89.0 bump.
- **The `*ArgsAreKnownFlags` tests are the early warning.** They parse what the modules render against the linked-in zarf's own command tree, so the day zarf drops v1alpha1 and `--set-variables` goes with it, `TestDeployArgsAreKnownFlags` fails by name. Nothing to schedule; the test is already watching.
- **`api/colonel-byte.dev/v1alpha1/prerequisites/` and all four schema tests are untouched.** That apiVersion is this project's, unrelated to zarf's. `requires.packages` names deployed zarf packages by name, which the rename does not touch.
- **The softer consequence is user-facing, not structural.** `set_variables` stays a documented parameter on `zarf_init`, `zarf_package_deploy` and the `deploy_packages` role, and quietly does nothing for a v1beta1 package. Whether its description says so, or whether the parameter grows a deprecation note, is a decision for when the examples move.

## Order of work

1. **Fix `pkgvendor` first.** It is independent of everything else and it is the only silent failure. Do it with the test.
2. **Fix the definition-key lookup in `zarf_package_inspect.py`** to accept either name. Small, and it makes the collection correct against v1beta1 packages it does not produce -- which is the realistic case, since a user's packages will move before this repository's examples do.
3. **Wait for `zarf dev upgrade`** in a pinned zarf release.
4. **Convert the examples with the tool**, then re-attach every comment by hand and rewrite the ones naming a renamed field.
5. **Keep one v1alpha1 fixture** per the extraction TODO, and add a v1beta1 one; do not convert the whole fixture set.
6. **Rewrite the skill last**, once the examples it points at are converted, and record the one-schema-or-both decision as a `choice-` doc.

## Verification

- `go run ./magefiles/core examples:lint` -- `zarf dev lint` over every reference package is the check that the converted YAML is valid. It needs a zarf binary, which is why step 1's unit test matters more than it looks.
- `go run ./magefiles/core test:unit` -- the `*ArgsAreKnownFlags` and plugin-docs contract tests.
- `go run ./magefiles/core test:endToEndNonCluster` -- the fixtures build with `zarf package create` at test time, so a bad conversion fails here rather than in review.
- `go run ./magefiles/core examples:images` over a converted package, to prove `replaceImages` writes the object form and `find-images` still parses.
- `prek run --all-files -c .pre-commit-config.yaml` -- `check-jsonschema` still validates each package's `docs/prerequisites.yaml` against our schema, which this migration does not touch.

## The question worth asking before any of it

[`choice-zarf-collection-removal.md`](choice-zarf-collection-removal.md) schedules this collection for deletion once ZEP-0072 resolves. A full v1beta1 migration of the examples and the authoring skill is real work on a tree with an end date, and most of it buys nothing that a reader of the current schema needs today.

The split that respects both facts: do items 1 and 4 of "The work" -- the `pkgvendor` writer and the definition-key lookup -- because those are *wrong*, not merely old, and they are wrong for a user whose own packages are already v1beta1. Leave items 2, 3 and 5 until zarf v1.0.0 forces them or the proof outlives its removal date. That is a smaller commitment than this page's length implies, and it is the recommendation.
