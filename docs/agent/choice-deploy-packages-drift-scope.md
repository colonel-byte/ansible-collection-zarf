# Drift detection converges for a package deployed in one phase, and not for one deployed in several

`colonel_byte.zarf.deploy_packages` skips an item whose components the cluster already reports as deployed. For a package that one item deploys, that works and the second run of a play is a no-op. For a package that several items deploy in phases -- which is what every `uds-bundle` init is -- it never settles, and the role re-deploys on every run.

This is a limit of what the cluster knows, not a bug in the role, and it is written down because the code looks like it ought to work and the obvious fixes are worse than the limit.

## What happens

The bundle's init is two items: the seed phase, then the rest. `zarf package list` reports the components of the **most recent** deploy of a package, because `NewDeployedPackage` in `vendor/github.com/zarf-dev/zarf/src/pkg/state/state.go` assigns `DeployedComponents: components` from the deploy that is happening and the secret is written fresh. There is no union and no history; `Generation` is a single incrementing `int`.

So on the second run of the same stack:

| Item            | What the cluster reports                        | What the item asks for                          | Decision                          |
| --------------- | ----------------------------------------------- | ----------------------------------------------- | --------------------------------- |
| init, phase one | `zarf-registry, zarf-agent`                     | `zarf-injector, zarf-seed-registry, git-server` | deploys, record becomes phase one |
| storage         | `local-path-images, local-path-chart`           | the same two                                    | skips                             |
| init, phase two | `zarf-injector, zarf-seed-registry, git-server` | `zarf-registry, zarf-agent`                     | deploys, record becomes phase two |

The two init items hand the record back and forth. There is no run after which both are satisfied.

`TestDeployPackagesRoleWalk` is what found this, and it asserts the behaviour above rather than the behaviour we wanted, so that the day zarf starts retaining the union the test fails and says so.

## Why the alternatives are worse

**Verify against the cluster instead of zarf's record.** The appealing one, and it does not reach. Helm releases do accumulate -- two deploys of the storage provider leave `local-path-provider.v1` and `.v2` -- but they do not map onto components. In the init package `zarf-seed-registry` and `zarf-registry` declare the *same* release name, `zarf-docker-registry`, so Helm cannot tell the two phases apart; and `zarf-injector` declares no chart and no manifest at all, so there is nothing of it to find. Three of the five components are unanswerable this way. Closing the gap would mean a rule per component -- "injector means look for these pods" -- which is the cluster-specific knowledge this collection exists to avoid, and which would be wrong for the next package.

**Union what the role deployed earlier in the same play.** Makes the second role invocation in one playbook skip, and changes nothing for the case that matters: a scheduled converge is a *fresh* run with no memory of the last one. It would turn a test green while leaving the behaviour it claims to prove broken, which is worse than the limit.

**A marker the role owns.** A ConfigMap recording what the role deployed would genuinely make the phased case idempotent across runs, and the collection already ships a vendored kubectl that could read and write it. It was declined because it is a second source of truth: it goes stale the moment anybody runs `zarf package remove`, or deploys the same package by hand, and a drift check that trusts a stale marker skips work that was needed. The failure mode of the limit is a wasted deploy; the failure mode of a stale marker is a cluster nobody converged.

## What this costs, concretely

- A phased package re-deploys on every run, which is `changed: true` forever and the deploy's wall-clock time each converge.
- `changed` is therefore not a drift signal for such a package. Nothing else in the collection can supply one: the modules report `changedSignal: partial` on every successful step, because zarf tells them which components it deployed and not whether any of them found the cluster already as wanted -- see [choice-zarf-ansible-module](choice-zarf-ansible-module.md).
- Nothing is at risk on the cluster. Re-deploying reconciles rather than fails, which `test/e2e/cluster/testdata/bundle-order.yml` proves on purpose in its fifth step: "a playbook that cannot be run twice is not a playbook".

## Before reversing this

The limit disappears if zarf merges `DeployedComponents` instead of replacing it, at which point the role's existing logic is correct with no change. That is drafted as finding 1 of [ZEP findings](../ZEP-finds.md). Check whether it landed before assuming the role needs work.
