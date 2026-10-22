# TODO: the per-item keys of `zarf_packages_to_deploy` are prose, not a contract

`zarf_packages_to_deploy` is declared in `ansible/colonel_byte/zarf/roles/deploy_packages/meta/argument_specs.yml` as `type: list`, `elements: dict`, and nothing else. The sixteen keys an item may carry are a list of prose bullets under one `description`. Ansible validates none of them.

So `path` is not enforced as required, no key is type-coerced, and `package_type`, `verify`, `registry_mode` and the three boolean pass-throughs get no `choices` check -- even though their role-level counterparts, declared a few lines further down in the same file, have all of that.

## What it cost already

Three separate defects traced back to this, all found in review and all now patched defensively in the tasks rather than at the boundary:

| Symptom                                                                                              | Patched as                                                |
| ---------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| `package_type: deploy_package` deployed nothing, wrote no result, and still marked the stack changed | an `assert` at the top of `deploy_package.yml`            |
| `timeout:` written with no value reached `\| length` as `None` and failed the task                   | `\| default('', true) \| string` on every per-item string |
| `take_ownership:` written with no value reached a boolean module parameter as `None`                 | `is not none` added to each three-state guard             |

Each patch is correct and each is a guard that an `options:` sub-spec would make unnecessary, with a better error: Ansible names the key and the expected type, where an assert names only what the role thought to check for.

## Why it was not just done

The obvious fix is to convert the bullets into an `options:` block under `zarf_packages_to_deploy`. Ansible would then validate every item key, and the role's own guards could go.

**The documentation generator would silently drop it.** `docOption` in `magefiles/pkg/gen/docs/ansible.go` has fields for `type`, `elements`, `required`, `default`, `choices`, `description` and the `cli_flag` extension -- and nothing for `options` or `suboptions`. `orderedOptions` decodes each option body through that struct, so a nested block is parsed and discarded. The generated `docs/ansible/role_deploy_packages.md` would lose the item keys entirely, which is worse than prose.

Writing both -- an `options:` block for validation and the bullets for the page -- would leave the item contract stated twice in one file, which is the duplication [`ansible/colonel_byte/zarf/AGENTS.md`](../../ansible/colonel_byte/zarf/AGENTS.md) names a single source of truth to avoid.

## What a real fix is

Two parts, in this order:

1. **Teach the generator suboptions.** Add an `Options goyaml.MapSlice` field to `docOption`, recurse in `orderedOptions`, and render a nested option as an indented sub-table or a second table under the parent's row. `magefiles/pkg/gen/docs/ansible_role.go`'s `writeRoleDoc` builds the rows; that is where the shape is decided. The module pages would get the same ability for free, which no module needs today.
2. **Convert the bullets**, delete them from the `description`, and then remove the three defensive guards above -- the assert on `package_type`, the `| default('', true) | string` wrappers, and the `is not none` clauses -- because argument validation covers all three and reports better.

Step 1 is the real work and it is the one with no caller asking for it, which is why this is written down rather than done. Step 2 without step 1 regresses the published page.

## Related

- The role's remaining per-item gap of a different kind is in [todo-deploy-packages-component-selection](todo-deploy-packages-component-selection.md): what an item that names no `components` should be compared against.
- [choice-deploy-packages-drift-scope](choice-deploy-packages-drift-scope.md) records why skipping settles for a single-phase package and not a phased one.
