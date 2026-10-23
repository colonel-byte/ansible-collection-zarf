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
)

const deployPackagesRolePlaybook = "test/e2e/noncluster/testdata/deploy-packages-role.yml"

// The role files this playbook exercises, read before it runs so that a change to one of them
// without a change here is visible in the diff.
var deployPackagesRole = []string{
	"ansible/colonel_byte/zarf/roles/deploy_packages/tasks/main.yml",
	"ansible/colonel_byte/zarf/roles/deploy_packages/tasks/deploy_package.yml",
	"ansible/colonel_byte/zarf/roles/deploy_packages/defaults/main.yml",
	"ansible/colonel_byte/zarf/roles/deploy_packages/meta/argument_specs.yml",
}

// TestDeployPackagesRoleRendersEveryItem drives colonel_byte.zarf.deploy_packages in check mode
// against the stub, which is the cheapest thing that evaluates its expressions.
//
// Almost every decision the role makes is Jinja, and the only thing that had ever run any of it
// was TestDeployPackagesRoleWalk in the cluster suite -- k3d, zarf, Docker and a network. That is
// how an item naming no components came to kill the play while three places documented it as
// deploying: no test cheap enough to run had rendered the branch, and the e2e fixture names
// components on every item.
//
// Check mode is what makes this possible rather than a limitation. Ansible templates a task's
// arguments before the action plugin decides anything, so the acting modules report themselves
// skipped while every expression that built their arguments has still been evaluated.
//
// What it cannot see is what zarf does with those arguments; that is the cluster suite's job.
func TestDeployPackagesRoleRendersEveryItem(t *testing.T) {
	zarf, err := exec.LookPath("zarf")
	if err != nil {
		t.Skip("zarf is not on PATH: the role inspects a real package to learn its name")
	}

	readFirst := append(
		[]string{packageInspectPlugin, packageInfoPlugin, infoBasePlugin},
		deployPackagesRole...)
	env := zarfPlaybookEnv(t, readFirst)

	// The wrappers are what the init and deploy tasks invoke. Check mode stops them short of
	// zarf, but a missing binary is a module error rather than a skip, so the run would prove
	// nothing without them.
	if _, err := os.Stat(filepath.Join(env.root, wrapperBinary)); err != nil {
		t.Skipf("the module wrapper %s is not built: run mage test:endToEndNonCluster, "+
			"which builds it", wrapperBinary)
	}

	stub := env.executable(t, clusterStub)

	// zarf_kubeconfig is set to a path that does not exist, and nothing notices: the stub answers
	// `package list` from a fixture and never opens it. It is set at all because the role refuses
	// a stack whose items name their own kubeconfig while the role-level one is empty, and that
	// refusal is one of the cases below.
	runZarfPlaybook(t, env, deployPackagesRolePlaybook, []string{
		envClusterDir + "=" + filepath.Join(env.root, clusterFixtureDir),
		envRealZarf + "=" + zarf,
	}, []string{
		"-e", "zarf_binary=" + stub,
		"-e", "zarf_init_wrapper=" + filepath.Join(env.root, wrapperDir, "zarf_init"),
		"-e", "zarf_deploy_wrapper=" + filepath.Join(env.root, wrapperDir, "zarf_package_deploy"),
		"-e", "zarf_kubeconfig=" + filepath.Join(env.root, clusterFixtureDir, "no-such-kubeconfig"),
		"-e", "zarf_result_dir=" + t.TempDir(),
		"-e", "fixture_package=" + buildInspectFixture(t, env.root, zarf),
		// Every acting task stops at the wrapper's check-mode refusal, which is what keeps this
		// free of a cluster while still rendering the arguments it would have passed.
		"--check",
	})
}
