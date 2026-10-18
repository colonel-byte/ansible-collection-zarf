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
	"os/exec"
	"path/filepath"
	"testing"
)

// The fixture the whole prerequisites flow is driven over.
const prerequisitesPlaybook = "test/e2e/noncluster/testdata/zarf-prerequisites.yml"

// The real zarf the cluster stub hands documentation extraction to. The role takes one zarf binary
// for both the extraction and the cluster reads, so it cannot be given a real one for half of them.
const envRealZarf = "ZARF_ANSIBLE_E2E_REAL_ZARF"

// The role files this playbook exercises, read before it runs for the reason given at
// documentationPlugin.
var prerequisitesRole = []string{
	"ansible/colonel_byte/zarf/roles/prerequisites/tasks/main.yml",
	"ansible/colonel_byte/zarf/roles/prerequisites/tasks/version_check.yml",
	"ansible/colonel_byte/zarf/roles/prerequisites/defaults/main.yml",
	"ansible/colonel_byte/zarf/roles/prerequisites/meta/argument_specs.yml",
}

// TestPrerequisitesRoleChecksAClusterAgainstADocument walks the whole prerequisites flow with no
// cluster: a real package is built and its document extracted by a real zarf, and the cluster
// behind it is the recorded one the stub serves.
//
// That combination is why the stub hands `package inspect documentation` to the real zarf rather
// than answering it. The role takes one zarf binary for both the extraction and the cluster reads,
// so it cannot be given a real one for half of them -- and faking the extraction would leave the
// role's own extraction branch untested, which is the branch an operator actually uses.
//
// The cases are the three outcomes that matter: a cluster that satisfies a document, one that fails
// four of its requirements at once and reports all four, and a version range the role cannot
// evaluate and must name rather than pass.
func TestPrerequisitesRoleChecksAClusterAgainstADocument(t *testing.T) {
	zarf, err := exec.LookPath("zarf")
	if err != nil {
		t.Skip("zarf is not on PATH: the role extracts a document from a real package")
	}

	readFirst := append([]string{documentationPlugin, clusterInfoPlugin, infoBasePlugin}, prerequisitesRole...)
	env := zarfPlaybookEnv(t, readFirst)
	stub := env.executable(t, clusterStub)

	runZarfPlaybook(t, env, prerequisitesPlaybook, []string{
		envClusterDir + "=" + filepath.Join(env.root, clusterFixtureDir),
		envRealZarf + "=" + zarf,
	}, []string{
		"-e", "zarf_binary=" + stub,
		"-e", "fixture_package=" + buildInspectFixture(t, env.root, zarf),
	})
}
