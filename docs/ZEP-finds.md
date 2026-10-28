# ZEP findings

A staging ground, not a submission. What follows is what building this proof of [ZEP-0072](https://github.com/zarf-dev/proposals/pull/73) turned up about zarf itself -- things a native Ansible module cannot work around from the outside, with the evidence for each and a proposed change. It is drafted here so the wording can be argued over in this repository before any of it is put to the ZEP team.

Every line reference is into the `github.com/zarf-dev/zarf` version this repository pins in `go.mod` and vendors, so each one can be opened and read without fetching anything.

The through-line is one sentence: **Ansible's contract is idempotency, and zarf does not currently give a consumer enough information to honour it.** Each finding below is a different face of that.

| #   | Finding                                                                                                 | Severity for a native module | Workaround available?               |
| --- | ------------------------------------------------------------------------------------------------------- | ---------------------------- | ----------------------------------- |
| 1   | The deployed-package record is replaced per deploy, so a phased deploy cannot converge                  | Blocking                     | No                                  |
| 2   | A deploy reports what it ran, never whether anything changed                                            | Blocking                     | No                                  |
| 3   | The component selection a deploy would make is not exposed                                              | Serious                      | Only by reimplementing it           |
| 4   | `required` and `optional` carry inverted defaults between the two package schemas                       | Trap                         | Yes, by branching on apiVersion     |
| 5   | Leaked viper default for git-push-username breaks subsequent init when git was omitted                  | Serious                      | Yes, clear default on the flag      |
| 6   | A package action's `./zarf` resolves to the system PATH or the consumer's own binary, never a named one | Serious                      | Yes, by keeping the library default |

## 1. The deployed-package record is replaced, not merged

`DeployedPackage.DeployedComponents` is assigned from the components of the deploy that is happening, and the secret is written fresh:

```go
// vendor/github.com/zarf-dev/zarf/src/pkg/state/state.go, NewDeployedPackage
deployedPackage := &DeployedPackage{
	Name:               definition.Metadata.Name,
	...
	DeployedComponents: components,
```

`Generation` beside it is a single `int` that increments, with no retained history. So after

```sh
zarf init --components zarf-injector,zarf-seed-registry,git-server   # phase one
zarf init --components zarf-registry,zarf-agent                      # phase two
```

`zarf package list` reports the init package's components as `zarf-registry, zarf-agent` and nothing else. The cluster has all five; its record remembers two.

### Why that is blocking rather than untidy

This is the shape every `uds-bundle` init takes, because the phases exist for a reason -- the seed registry has to exist before a storage provider can be deployed through it, and the full registry needs the StorageClass that provider installs. A consumer that wants to skip work already done has to ask "are these components present", and the only cluster-side answer is the last phase's.

The practical result, which we measured: a role that deploys such a bundle oscillates forever. Phase one's item sees phase two's components recorded, deploys, and overwrites the record with phase one's; phase two's item then sees phase one's, deploys, and overwrites it back. `changed: true` on every run, for every run, with no stable state to reach. The suite that found this is `TestDeployPackagesRoleWalk` in `test/e2e/cluster/`.

### Proposed change

Merge `DeployedComponents` by component name rather than replacing the list, keeping the newest entry per name. The shape for this already exists: `DeployedComponent` carries an `ObservedGeneration`, which is only meaningful if several generations can be represented at once.

Removal already prunes correctly -- `vendor/github.com/zarf-dev/zarf/src/pkg/packager/remove.go` deletes entries with `slices.DeleteFunc` and removes the secret when the list empties -- so a merge would not strand a component that was taken off the cluster, and would not strand one dropped from a later revision of the package.

### What we did instead

Nothing that recovers it. The role's drift detection is documented as converging only for a package deployed in a single phase; a package named by several items in one stack is re-deployed on every run. That is safe -- re-deploying reconciles rather than fails, which `test/e2e/cluster/testdata/bundle-order.yml` asserts deliberately in its fifth step -- but it is not idempotent, and it is the limitation this finding would remove. See [choice-deploy-packages-drift-scope](agent/choice-deploy-packages-drift-scope.md).

## 2. A deploy reports what it ran, never whether anything changed

`changed` is the single most load-bearing field in an Ansible module result. Handlers fire on it, `--check` reports against it, drift gates read it, and a module that always says `true` is a module an operator learns to ignore.

Zarf reports which components it deployed. It does not report whether deploying them altered the cluster, so a wrapper has no honest way to answer. This repository's own module says so in `docs/agent/choice-zarf-ansible-module.md` and signals `changed` as `partial` for exactly this reason -- the test holding it is `assertStepRan` in `test/e2e/cluster/zarf_init_test.go`, which asserts the signal is `partial` on every successful step:

> Partial is the honest answer and the only one a wrapper can give: zarf reports which components it deployed and not whether any of them found the cluster already as it wanted.

Combined with finding 1, the consequence is that **the only path to `changed: false` anywhere in a zarf Ansible collection is the consumer deciding to skip the deploy entirely** -- which is the decision finding 1 makes impossible for a phased package.

### Proposed change

Either of these would be enough, and the first is the smaller:

- A per-component outcome on the deploy result distinguishing installed, upgraded and unchanged. Helm already knows this per release; the information exists inside a deploy and is discarded.
- A deploy mode that reports the delta it would apply without applying it, which would also serve `--check`.

A native module would map the first onto `changed` directly, and the second onto check mode, which it currently has to implement as "refuse and report skipped".

## 3. The component selection a deploy would make is not exposed

With no `--components`, the set zarf deploys is decided in `vendor/github.com/zarf-dev/zarf/src/pkg/packager/filters/deploy.go`. Non-interactively it is every non-optional component, plus every optional component marked `default`, plus for each component group of more than one the member marked `default` -- and a group with no default is an error rather than an empty selection.

None of that is answerable from outside. `zarf package inspect definition` returns the authored definition, so a consumer wanting to know "would this item deploy anything new" has to reimplement the filter, across both package schemas, including the group rules. Getting it wrong in the permissive direction means silently declining a deploy that was needed.

### Proposed change

Expose the resolved selection -- a flag on `zarf package inspect definition`, or part of the dry-run in finding 2. The authoritative answer is the one the filter itself computes; anything else is a copy that agrees until it does not.

### What we did instead

Declined to guess. An item that names no components is deployed on every run, and the reasoning is recorded in [todo-deploy-packages-component-selection](agent/todo-deploy-packages-component-selection.md) so that nobody reimplements the filter by accident.

## 4. `required` and `optional` carry inverted defaults between the two package schemas

| Schema     | Field            | Absent means |
| ---------- | ---------------- | ------------ |
| `v1alpha1` | `required *bool` | optional     |
| `v1beta1`  | `optional bool`  | required     |

`v1alpha1.ZarfComponent.IsRequired()` returns `false` when `Required` is nil; `v1beta1.Component.Optional` is `false` when absent, which means required. `zarf package inspect definition` emits whichever version the package was authored in, so a consumer reading a definition must branch on `apiVersion` to get the sense of the field right -- and a consumer that does not will be exactly wrong, not approximately wrong, on half the packages it meets.

This is working as designed per schema; the finding is that the two are a silent-wrong-answer trap for anything reading definitions generically. A note in the migration guidance would cost nothing and would be read by the next person to write a consumer.

## 5. Leaked viper default for git-push-username breaks subsequent init when git was omitted

In `src/cmd/package.go` (`newPackageMirrorResourcesCommand`), viper registers a default for `init.git.push_username`:

```go
v.SetDefault(VInitGitPushUser, state.ZarfGitPushUser) // "zarf-git-user"
```

Because viper is a process-level singleton, `newInitCommand()` in `src/cmd/initialize.go` picks up this default when registering the flag:

```go
cmd.Flags().StringVar(&o.gitServer.PushUsername, "git-push-username", v.GetString(VInitGitPushUser), lang.CmdInitFlagGitPushUser)
```

When an initial `zarf init` runs without `git-server` (e.g. in a seed phase before dynamic persistent storage exists), `s.GitServer` in `zarf-state` remains unpopulated (`s.GitServer.PushUsername == ""`). On any subsequent `zarf init`, even if the operator provides no git arguments or excludes `git-server`, Cobra's flag parsing defaults `o.gitServer.PushUsername` to `"zarf-git-user"`.

`validateExistingStateMatchesInput` then evaluates:

```go
if helpers.IsNotZeroAndNotEqual(gitServer, s.GitServer) {
    return fmt.Errorf("cannot change git server information after initial init, to update run `zarf tools update-creds git`")
}
```

Because `"zarf-git-user"` is non-zero while `s.GitServer.PushUsername` is empty, `helpers.IsNotZeroAndNotEqual` evaluates to `true` and the re-init fails immediately.

### Proposed change

Do not register command-specific defaults into the shared viper instance, or do not treat the default username as an operator-provided change in `validateExistingStateMatchesInput` when `s.GitServer` was not configured by the initial init.

### What we did instead

In `internal/zarfmod/libraryrunner.go`, after `cmd.NewZarfCommand()` constructs the command tree, `pruneInitGitPushUserDefault` clears `DefValue` and resets the flag value to empty on the `init` command if it holds the leaked default. A subsequent in-process init without an explicit `--git-push-username` then leaves `gitServer.PushUsername` empty, allowing `validateExistingStateMatchesInput` to pass.

## 6. A package action's `./zarf` resolves to the system PATH or to the consumer's own binary, and nothing else

Zarf rewrites `./zarf ` inside a package action to whatever `utils.GetFinalExecutableCommand` returns (`vendor/github.com/zarf-dev/zarf/src/pkg/packager/actions/actions.go`, `actionCmdMutation`), and `cluster.InjectData` builds its `tools kubectl` invocation from the same call (`vendor/github.com/zarf-dev/zarf/src/pkg/cluster/data.go`). That function offers three answers and no fourth (`vendor/github.com/zarf-dev/zarf/src/pkg/utils/io.go`):

```go
if config.ActionsCommandZarfPrefix != "" {
    return fmt.Sprintf("%s %s", executablePath, config.ActionsCommandZarfPrefix), nil
}
if config.ActionsUseSystemZarf {
    return "zarf", nil
}
return executablePath, nil
```

So an action either runs the bare name `zarf` resolved on `PATH`, or runs the consumer's own executable -- optionally under a subcommand prefix. There is no way to say "the zarf at this path".

That matters here because this collection's modules link zarf in as a library rather than shelling out, so `executablePath` is the module binary. That binary dispatches on `argv[0]` and reads its parameters as WANT_JSON on stdin; it accepts no zarf command line at all. A package action running `./zarf tools kubectl ...`, or any component with a data injection, therefore fails outright under `ActionsUseSystemZarf = false` -- and fails inside zarf's own subprocess, where the module cannot improve the diagnosis.

It also undercuts the role's `zarf_binary` key. An operator who points the modules at `/opt/zarf/bin/zarf` has said where zarf is, and that answer cannot be passed on: an action still gets `zarf` from `PATH`, or nothing.

### Proposed change

Let a library consumer name the binary -- an `ActionsZarfPath`, or an `ActionsCommandZarfPrefix` that may carry an absolute path of its own -- so a consumer that is not a zarf CLI can point actions at a real zarf without putting it on `PATH` under that exact name.

### What we did instead

We leave `config.ActionsUseSystemZarf` at the `true` that `src/config` initialises it to, which is the deliberate delta: zarf's own `main` sets it `false`, and the obvious reading of "we linked zarf in so there is no child process" says a library consumer should do the same. It must not. `internal/zarfmod/libraryrunner.go` restates the `true` in its `init` rather than relying on the default, so a zarf release that flips the default does not silently break every package action, and the management-node image installs `zarf` at `/usr/local/bin/zarf` so the bare name resolves.

## Checked and not a finding

**The deployed version string is the package's own `metadata.version`.** Worth recording because a consumer that skips work already done needs to compare versions, and it was not obvious that the two came from the same place. `zarf package list` builds its output with `Version: pkg.Metadata.Version` (`vendor/github.com/zarf-dev/zarf/src/cmd/package.go`, `packageListOptions.run`), which is the same field `zarf package inspect definition` reports, so the two are equal by construction rather than by convention. A package's flavour appears in the tarball's file name and in `build.flavor`, not in the version the cluster reports.
