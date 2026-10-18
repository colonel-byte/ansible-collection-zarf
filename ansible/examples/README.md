# Zarf Ansible Examples

Working playbooks for the `colonel_byte.zarf` collection. Nothing here runs on the cluster's nodes: each playbook delegates to one management node, where the module files run the installed `zarf`, and zarf talks to the cluster through a kubeconfig.

These files are outside the collection, so they are not installed with it. Copy what you need. The one playbook that does ship is `playbooks/init.yml` inside the collection itself.

Every module parameter is in the [collection reference](../../docs/ansible/collection.md), a page per module and per role, generated from the collection itself. What `changed` means, how the collection is installed, and why a connected storage provider has to be deployed before `zarf init` on some distributions are in the [Ansible module guide](../../docs/guides/zarf-ansible-module.md). This directory is the part that is easier to read as a file than as a table.

## Playbooks

| File                                | What it runs                                                                                                         |
| ----------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `playbooks/zarf-bundle-order.yml`   | Zarf init and package deploy in bundle order, checking installed package versions and deploying only on drift.       |
| `playbooks/zarf-provider-first.yml` | Connected storage provider deploy before zarf init, checking installed package versions and deploying only on drift. |
| `playbooks/zarf-state.yml`          | Fetch and parse Zarf cluster state using both the `state` role and `zarf_state_info` module primitive.               |
| `playbooks/zarf-prerequisites.yml`  | Check each package's `PackagePrerequisites` document against the cluster before deploying it, in bundle order.       |

Run one against a management node:

```sh
ansible-playbook -i management.ini ansible/examples/playbooks/zarf-bundle-order.yml
```

The end-to-end suite under `test/e2e/cluster/` runs its own copies of the first two against real k3d clusters, so the arrangement these show is the arrangement that is tested.
