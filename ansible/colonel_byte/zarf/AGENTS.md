# Documentation Strategy and Agent Guidance for `colonel_byte.zarf`

This directory tree defines the `colonel_byte.zarf` Ansible collection. The documentation for this collection, its modules, and its roles published under `docs/ansible/` is generated automatically from the code in this directory and validated by contract tests.

This file did not exist while the collection lived in [colonel-byte/cargoship](https://github.com/colonel-byte/cargoship) -- it borrowed `ansible/colonel_byte/cargoship/AGENTS.md`, which the generators' contract was written against. What follows is that contract, restated for the collection that is actually here.

## Scope

Zarf is not an SSH orchestrator. It talks to one cluster through a kubeconfig, from the node it runs on, so every module here runs `zarf` on the node the task is delegated to and nothing on any managed node. There is no inventory to project and no fleet to connect to: what a task supplies is a kubeconfig, a package, and the parameters the zarf command takes.

A role that configures cluster nodes directly -- writing files on them, installing packages, managing services -- does not belong here. It belongs in the playbook that consumes this collection.

## Two kinds of module, and the boundary between them

| Kind | Modules | How it is built |
| ---- | ------- | --------------- |
| Converging | `zarf_init`, `zarf_package_deploy` | A compiled wrapper binary, with its parameters as a Go struct in `internal/zarfmod` and its command line rendered by a Go function. The action plugin holds the documented parameter surface and the delivery, and no translation rules. |
| Read-only | `zarf_package_info`, `zarf_package_inspect`, `zarf_state_info` | The action plugin is the whole implementation, sharing `ZarfInfoActionBase` in `plugins/plugin_utils/info.py`. No binary, no Go struct. |

The boundary is not stylistic, and [choice-zarf-info-modules](../../../docs/agent/choice-zarf-info-modules.md) records where it sits. The short version: a wrapper earns its keep on a flag surface worth projecting, progress worth following, and a `changed` worth signalling. A read-only command has none of the three. The threshold for crossing back is named in that record -- **a read-only module that grows a second or third parameter-controlled flag, or any flag whose value attaches in a way pflag cares about, wants a wrapper binary** rather than a better regular expression.

## Markdown line formatting

Follow the line and table formatting rules in [`ansible/AGENTS.md`](../../AGENTS.md):

- Do not hard-wrap lines in Markdown documents. Write full paragraphs and list items on single, continuous lines.
- Pad table cells so columns line up.

## Links that leave this directory

`README.md` and `galaxy.yml` ship inside the built collection, where a relative path into `docs/` does not resolve. Their links are absolute, and they point at the published book -- `https://colonel-byte.github.io/ansible-collection-zarf/<path>.html` -- rather than at the Markdown on GitHub.

Files that do not ship, and links between files inside `docs/`, stay relative.

## Single sources of truth

| Component | Source of truth | Generated page |
| --------- | --------------- | -------------- |
| Roles | `roles/<role>/meta/argument_specs.yml` and `roles/<role>/defaults/main.yml` | `docs/ansible/role_<role>.md` |
| Modules | `plugins/action/<module>.py` (`DOCUMENTATION` and `EXAMPLES` blocks) | `docs/ansible/module_<module>.md` |
| Collection overview | `docs/ansible/collection.md` | Hand-written; not generated |
| Modules overview | `docs/ansible/modules.md` | Hand-written; not generated |
| Roles overview | `docs/ansible/roles.md` | Hand-written; not generated |

## Quote every description entry

In `roles/<role>/meta/argument_specs.yml` and in the `DOCUMENTATION` blocks, every entry of a `description:` list is written as a double-quoted scalar:

```yaml
description:
  - "The package to deploy. It is the positional package source C(zarf package deploy) accepts."
  - "Every task carries C(run_once: true) and delegates to the node holding the kubeconfig."
```

An unquoted entry is a plain scalar, and a plain scalar cannot contain `: `. Descriptions here routinely do -- `C(run_once: true)`, `C(no_log: true)`, and any sentence with a colon before a clause -- and YAML reads the entry as a mapping instead, so the file either fails to parse or parses into the wrong shape. Quoting only the entries that happen to contain one means the next sentence with a colon in it breaks the file, so quote all of them.

`choices:` and `author:` entries stay unquoted. They are single tokens with no prose in them.

## Updating role documentation

1. Define the role variables, types, defaults, choices, descriptions, and synopsis in `roles/<role>/meta/argument_specs.yml`. Double-quote every `description:` entry.
2. Define the corresponding default values in `roles/<role>/defaults/main.yml`. The key order there determines the table row order in the generated page.
3. Regenerate: `go run ./magefiles/core generate:document`.

## Updating module documentation

1. Update the `DOCUMENTATION = r"""..."""` block in `plugins/action/<module>.py`.
   - Each option under `options:` must specify `type:`, `description:`, and `cli_flag:` -- the zarf flag the parameter renders, or `None` when it renders none, because it is the command's positional argument or because it is consumed before the command line is built.
   - `cli_flag:` is an extension rather than a standard Ansible documentation key. The generator reads it; `ansible-doc` would not know it.
   - If `required: false` and a default value exists, provide `default:`.
   - Maintain the option order in the YAML mapping; the generator preserves declaration order for table rows.
2. Update the `EXAMPLES = r"""..."""` block with a realistic playbook snippet.
3. For a **converging** module, update the matching parameter struct in `internal/zarfmod/<action>.go`.
4. For a **read-only** module, keep every flag a literal inside the module-level `command_parts` function. The test that covers these reads string literals out of Python source; a flag assembled from a variable is a flag it cannot see.
5. Run the contract tests:

```sh
go test -v ./internal/zarfmod -run 'TestActionPluginDocsMatchModuleParams|TestInfoPluginDocsMatchRenderedFlags|TestEveryActionPluginIsCovered'
```

6. Regenerate the documentation, and commit the plugins and the generated pages together:

```sh
go run ./magefiles/core generate:document
```

## A new action plugin must join a test list

`TestEveryActionPluginIsCovered` globs `plugins/action/*.py` and requires each plugin to appear in exactly one of two lists in `internal/zarfmod/infoplugins_test.go`: `wrapperModuleCases` or `infoModules`. A plugin in neither fails the test by name, and a list naming a plugin that no longer exists fails too, since that is a check which passes by checking nothing.

This exists because `plugindocs_test.go` is a table of modules, so a module absent from the table is a module no test looks at, and nothing said so.

## Do not add `cacheable: true` to the state role

`set_fact` defaults to `cacheable: false`, so the parsed `zarf_state` stays in the play's memory and never reaches a configured fact cache plugin. Setting it true would write the cluster's registry credentials into a jsonfile cache on the control node, outliving the play that read them.

For the same reason, `zarf_no_log` defaults to true on **both** of the `state` role's tasks, not only the module call: a `set_fact` that reads a censored result prints the value in its own result, so `no_log` on the module alone looks correct in review and leaks on the next line.
