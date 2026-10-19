# Working in `skills/`

Two skills, for the two authoring tasks this repository is about: writing a Zarf package, and writing the document that package carries to say what it needs of a cluster. They are prompts for an agent, not documentation -- the reader is a model with a task, in a tree that is not this one.

## They cover authoring on a connected machine, and stop there

The scope is deliberate and is the first thing to check a change against: **a skill here covers writing and building a package on a machine with a network, and nothing a cluster afterwards does with it.** Scaffolding a `zarf.yaml`, discovering images, splitting components, deriving what to require, `zarf dev lint`, `zarf package create` -- in scope. A deployment that failed, a component that never became ready, a cluster to diagnose -- out of scope, and both pages say so in their frontmatter and their opening section.

That line is not about what is interesting. It is about what a skill can be good at. Authoring happens where the charts and manifests are, so the advice can be mechanical -- "grep the rendered output for non-core `apiVersion`s, those are your `requires.crds`" is a procedure that works. Debugging happens on a cluster the author has never seen, where the same page could only offer guesses. A skill that tried both would be fired by the wrong trigger and answer with the wrong half.

Two consequences worth stating, because they look like omissions:

- **The `prerequisites` role is described, but only as the consumer.** An author writes a better document knowing what reads it. It is not there so that somebody can run the role and interpret its output.
- **Nothing here is a troubleshooting guide**, and a change that adds one belongs in `docs/` instead -- where a human reads it and can notice when it disagrees with the code.

If the scope changes, change the frontmatter `description` with it. The description is what decides whether a skill is loaded at all, so a page whose body outgrows its triggers is a page that is never read when it would have helped, and loaded when it cannot.

## A skill is read somewhere else

Nothing in a skill may reference a path outside its own directory. It is copied into somebody's package tree, where no path into this repository resolves -- which is why `zarf-package-prerequisites` carries its own copy of the JSON Schema, written by `generate:schema` rather than by hand. Absolute URLs are fine; the connected-side scope is what makes them fine.

## What holds the pages to the code

The schema copy is generated and `TestCommittedSchemasAreCurrent` holds it against a fresh render, so it cannot go stale. The prose can, and did: the `requires.fluxcd` section was renamed and the page went on teaching the old shape, because regenerating a schema says nothing about the markdown beside it. Two tests in `magefiles/pkg/gen/schema` close that:

| Test                                         | What it would catch                                                                                               |
| -------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `TestSkillYAMLBlocksParse`                   | A YAML block on any skill page that does not parse, including the `zarf.yaml` example, which has no Go types here |
| `TestSkillExamplesMatchTheTypes`             | Any `PackagePrerequisites` example drifting from the Go types, or quietly losing a section it used to show        |
| `TestSkillSchemaPathResolvesFromTheDocument` | An example's `$schema` not resolving from the layout the page's own instructions prescribe                        |

They cover every page under `skills/` and every example on one, not the one page that carries a document today. An example is found by the `kind:` it declares rather than by position, and only fences written at the margin are read -- a block indented into a list item is a fragment illustrating one key, and a pattern that matched those could not see their closing fence and swallowed the prose after it.

Neither test reads the prose. When a change to the format reaches these pages, the paragraphs are still on you.
