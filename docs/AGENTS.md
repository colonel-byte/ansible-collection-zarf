# Markdown line formatting

## Do not hard-wrap lines in Markdown documents

Markdown files should NOT have hard-wrapped or truncated lines. Instead, write full paragraphs and list items on single, continuous lines without manual line breaks, letting editors and renderers handle line wrapping dynamically.

- Write entire paragraphs or list items on a single line without manual line breaks / hard wraps.
- Let editors and rendering tools handle line wrapping dynamically.

Table cells are padded so the columns line up in the source, and a pre-commit hook does it for you. That rule is in the root [`AGENTS.md`](../AGENTS.md).

## A moved page keeps a redirect

Every page under `docs/` is published at `https://colonel-byte.github.io/ansible-collection-zarf/<path>.html`, so renaming or moving one breaks whatever already links to it from outside this repository -- a bookmark, an older release's `galaxy.yml`, a listing on Galaxy. mdBook does not notice: the old URL simply 404s.

So a move comes with an entry in `[output.html.redirect]` in [`book.toml`](../book.toml), keyed on the old path and valued with the new one. The value is resolved relative to the key's own directory, not the site root, so a page that moved one level down reads `"/ansible/collection.html" = "zarf/collection.html"`. mdBook writes a small redirecting HTML file at each key when the book is built.

Keep the entry for as long as anything outside the repository might still hold the old link. They cost one file each, and `mdbook build` is what proves one works -- the redirect file appears at the old path and names the new one.

A page that was never published does not need one, which is why the generated pages added for a new collection have no entries.

## A pinned release version is release-please's to update

A page that pins this project's own version -- an image tag, a release archive's filename, a `galaxy.yml` version in an install snippet -- goes stale on the next release, and nobody notices until a reader copies the command. Release-please already bumps `galaxy.yml`, and it will bump a Markdown page the same way once the page tells it where to look and the configuration tells it the page exists. Both halves are required: an annotated page that is not listed is never read, and a listed page with no annotation is read and left alone.

Annotate with the block form, as HTML comments on their own lines around the fence:

````markdown
<!-- x-release-please-start-version -->
```sh
ansible-galaxy collection install colonel_byte.zarf:0.87.0
```
<!-- x-release-please-end-version -->
````

The per-line form, `x-release-please-version` on the same line as the version, is the one to reach for in YAML or a Dockerfile and the wrong one here: inside a fenced code block an HTML comment is content, so it renders to the reader as part of the command they are about to run.

Then add the page to `extra-files` in [`release-please-config.json`](../release-please-config.json), beside the `galaxy.yml` entry. The next release PR rewrites every version inside every annotated region in it, and a page that nothing lists is a page release-please does not open.

**An annotated region must hold no other dotted-decimal string.** Release-please replaces each semver match it finds between the markers, and `0.0.0.0` matches one -- a region containing both an image tag and that address comes out of the next release with `https://0.29.1.0:41609` in it. `127.0.0.1`, a k3s version, a CIDR and a zarf or k3d version are all the same hazard. Where an example needs both, split the fence: the version-free half stays unannotated.

A version in a generated page is annotated where it is written, not where it lands: a pinned version in `README.md` gets the markers in `README.md`, since `docs/index.md` is rewritten from it on the next `generate:document`.

Illustrative output is the one place to prefer a placeholder over an annotation. `rpm -q` prints `zarf-modules-0.87.0-1.x86_64`, where the trailing `-1.x86_64` is the RPM release and reads as a semver prerelease, so a rewrite of that region can swallow it. Write `zarf-modules-<version>-1.x86_64` instead, which cannot go stale.

Set the version to the current release when adding the annotation, rather than leaving whatever was there. The markers fix the page from the next release onward and do nothing for the one in front of a reader today.

## Do not hand-edit the generated pages

Every generated path below is also marked `linguist-generated` in [`.gitattributes`](../.gitattributes); see the "Keeping `.gitattributes` in sync with the generators" section of the root [`AGENTS.md`](../AGENTS.md) when a generator starts or stops writing one.

Four parts of the `docs/` tree are generated from the code and are overwritten wholesale on the next run. `docs/golang/` is deleted and recreated, and `docs/ansible/module_*.md` and `role_*.md` are removed by pattern -- by pattern rather than wholesale, because the hand-written overviews that introduce them sit in the same directory and a `RemoveAll` would take them too. `docs/index.md` and `docs/security.md` are rewritten from files that live outside `docs/` because GitHub reads them there, and `docs/SUMMARY.md` is rewritten from the tree that run produced.

| Path                       | Generated from                                                                     |
| -------------------------- | ---------------------------------------------------------------------------------- |
| `docs/golang/`             | Godoc comments in `internal/`, rendered by `gomarkdoc`                             |
| `docs/ansible/module_*.md` | The Ansible module action plugins parsed in `generateModuleDocs()`                 |
| `docs/ansible/role_*.md`   | The role `meta/argument_specs.yml` files, parsed in `generateRoleDocs()`           |
| `docs/index.md`            | `README.md`, with its relative links rebased from the repository root onto `docs/` |
| `docs/security.md`         | `.github/SECURITY.md`, with its relative links rebased the same way                |
| `docs/SUMMARY.md`          | The mdBook table of contents, compiled from the rest of the `docs/` tree           |

`docs/ansible/collection.md`, `modules.md` and `roles.md` are hand-written, and sit beside the generated pages. They are not marked `linguist-generated` and are not removed by a generation run.

To change one of those pages, change what it is generated from - a godoc comment in `internal/`, a module's `DOCUMENTATION` block, or a role's `meta/argument_specs.yml` - and then regenerate:

```sh
go run ./magefiles/core generate:document
```

Use that `go run` form rather than `mage generate:document`. It runs the same entry point and needs only the Go toolchain, so it works on a host that has no `mage` binary installed. See [dev/mage.md](dev/mage.md) for the rest of the targets and what else each one writes.

Commit the regenerated files with the change that caused them, so the tree matches the code.
