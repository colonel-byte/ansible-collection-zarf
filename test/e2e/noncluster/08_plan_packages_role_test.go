// Copyright 2026 colonel-byte
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package noncluster

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	planPackagesPlaybook = "test/e2e/noncluster/testdata/plan-packages-role.yml"
	bundleFixtureDir     = "test/e2e/noncluster/testdata/bundle-fixture"
)

// The role files this playbook exercises, read before it runs so that a change to one of them
// without a change here is visible in the diff.
var planPackagesRole = []string{
	"ansible/colonel_byte/zarf/roles/plan_packages/tasks/main.yml",
	"ansible/colonel_byte/zarf/roles/plan_packages/defaults/main.yml",
	"ansible/colonel_byte/zarf/roles/plan_packages/meta/argument_specs.yml",
	deploymentOrderFilter,
}

// planFixtures is the clean subset of the bundle fixture pool: one candidate per group that wins
// it, one that loses, and the package carrying no prerequisites document.
//
// The whole pool is deliberately not staged. It holds a cycle and a priority tie, both of which
// the planner refuses, so a directory containing all of it has no plan -- which is the point of
// the pool being a pool. testdata/deployment-order.yml feeds those subsets to the planner
// directly, where a refusal is the assertion rather than a failed play.
var planFixtures = []string{
	"init",
	"metrics-server",
	"csi-rook-ceph",
	"csi-longhorn",
	"gitops-fluxcd",
	"gitops-argocd",
	"workload-app",
}

// TestPlanPackagesRolePlansAStagingDirectory drives colonel_byte.zarf.plan_packages over a
// directory of real package archives built from the fixture pool.
//
// This is the half testdata/deployment-order.yml cannot reach. That playbook hands the planner
// documents that were read with `lookup('file')`, which proves the ordering and nothing about how
// a document gets out of a tarball. Here the role runs the two zarf inspections itself, so what is
// covered is the join between them: an archive's name and version come from the package
// definition, its document from the documentation layer, and a package carrying no documentation
// mapping has to come through that join as a root rather than as a missing entry.
//
// The directory also holds two files that are not packages, one matching the archive patterns and
// one not. A staging directory in the field holds checksums and signatures beside the packages,
// and a planner that aborted on one of them would be useless for its one job.
func TestPlanPackagesRolePlansAStagingDirectory(t *testing.T) {
	zarf, err := exec.LookPath("zarf")
	if err != nil {
		t.Skip("zarf is not on PATH: the role inspects real package archives to plan them")
	}

	env := zarfPlaybookEnv(t, planPackagesRole)

	staging := t.TempDir()
	for _, name := range planFixtures {
		source := filepath.Join(env.root, bundleFixtureDir, name)
		build := exec.Command(zarf, "package", "create", source, "--confirm", "-o", staging) //nolint:gosec // the suite's own constants
		build.Dir = env.root
		out, err := build.CombinedOutput()
		require.NoError(t, err, "building the %s fixture failed:\n%s", name, out)
	}

	// A tarball that is not a Zarf package, and a file that is not an archive at all. The first
	// is inspected and has to be dropped; the second never matches the patterns.
	require.NoError(t, os.WriteFile(filepath.Join(staging, "not-a-package.tar"), []byte("not a package\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(staging, "checksums.sha256"), []byte("0  zarf-package-init-amd64-0.89.0.tar.zst\n"), 0o600))

	runZarfPlaybook(t, env, planPackagesPlaybook, nil, []string{
		"-e", "zarf_plan_directory=" + staging,
		"-e", "zarf_binary=" + zarf,
	})
}
