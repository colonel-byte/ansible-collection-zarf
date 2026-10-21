# E2E cluster

Runs [`test/e2e/cluster`](../dev/e2e-tests.md) -- the suite that walks the converging modules across two real k3d clusters, then runs the other five acting modules against the cluster the first walk initialised.

It is **not** a check on every push. It stands up two clusters, pulls three zarf packages, and runs a real `zarf init` against each, which makes it the slowest and least hermetic thing in this repository; the first run on a cold cache pays for the package pulls. Everything it covers that can be covered without a cluster already is, by the unit tests and by [`e2e.yaml`](e2e.yaml.md) -- which includes every acting module's refusals and check-mode answers, so what is left here is the half that needs a registry, a cluster state or something deployed to take away.

So it runs in two cases only: on demand from the Actions tab (`workflow_dispatch`), and on a pull request carrying the `e2e-cluster` label. The job's `if:` condition enforces both, which is why adding the label to an open pull request is enough to start a run.

The job installs Go, Python, `ansible-core`, k3d and zarf, then runs `test:endToEndCluster` with a 90-minute timeout and tears down any surviving clusters with `test:cleanClusters` under `if: always()`.

One step is easy to mistake for boilerplate: `mkdir -p /tmp/k3d`. Both k3d configs bind-mount that directory into the nodes as the local-path provisioner's root, k3d does not create it, and a missing bind source fails the cluster create before any test runs.

The Go build is fully vendored, but the suite itself does reach the network to pull packages, so `GOPROXY=off` closes the module proxy only -- not the network.
