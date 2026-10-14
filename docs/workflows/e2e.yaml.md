# E2E

Runs [`test/e2e/noncluster`](../dev/e2e-tests.md) -- the three read-only modules and the state redaction, driven through a real `ansible-playbook` against a stub zarf. It needs no cluster. Runs on every pull request and on the merge queue.

The triggers carry **no path filter**, deliberately. This is a required status check, and a filtered required check never reports at all on a change that only touches the ignored paths, which leaves the check pending forever.

Installing `ansible-core` is a step rather than an optional convenience. Every module in this collection is an action plugin, so only Ansible can run one; without `ansible-playbook` on `PATH` the whole suite skips itself with a reason and the job still passes. That is exactly the failure mode worth preventing here -- see the note in [`e2e-tests.md`](../dev/e2e-tests.md) about a suite that silently stopped running looking just like a suite that passed.

The suite that needs k3d, a real zarf and Docker is not run here; see [`e2e-cluster.yaml`](e2e-cluster.yaml.md).

`github.ref` is distinct per `merge_group` ref, so the concurrency group never lets a queued entry be cancelled by a push to the pull request it came from.
