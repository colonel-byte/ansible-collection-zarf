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

// The fixtures the prerequisites flow is driven over.
const (
	packageDocumentationPlaybook = "test/e2e/noncluster/testdata/zarf-package-documentation.yml"
	clusterInfoPlaybook          = "test/e2e/noncluster/testdata/zarf-cluster-info.yml"
	prerequisitesPlaybook        = "test/e2e/noncluster/testdata/zarf-prerequisites.yml"
	clusterStub                  = "test/e2e/noncluster/testdata/zarf-cluster-stub.sh"
	clusterFixtureDir            = "test/e2e/noncluster/testdata"
)

// The environment the cluster stub reads: the directory holding its recorded answers, and the real
// zarf it hands documentation extraction to.
const (
	envClusterDir = "ZARF_ANSIBLE_E2E_CLUSTER_DIR"
	envRealZarf   = "ZARF_ANSIBLE_E2E_REAL_ZARF"
)

// The plugins and role files these playbooks exercise. They are read before the playbooks run, for
// the reason given at stateInfoPlugin: Go's test cache keys on what the test process opens, and
// everything here is opened by the ansible-playbook child instead -- so without the reads, editing
// a plugin or a role leaves a passing result cached.
var (
	documentationPlugin = "ansible/colonel_byte/zarf/plugins/action/zarf_package_documentation.py"
	clusterInfoPlugin   = "ansible/colonel_byte/zarf/plugins/action/zarf_cluster_info.py"

	prerequisitesRole = []string{
		"ansible/colonel_byte/zarf/roles/prerequisites/tasks/main.yml",
		"ansible/colonel_byte/zarf/roles/prerequisites/tasks/version_check.yml",
		"ansible/colonel_byte/zarf/roles/prerequisites/defaults/main.yml",
		"ansible/colonel_byte/zarf/roles/prerequisites/meta/argument_specs.yml",
	}
)

// TestZarfPackageDocumentationExtractsTheLayer holds what zarf_package_documentation returns from a
// package's documentation layer.
//
// It needs a real zarf and no cluster: extracting a layer is reading a tarball. The fixture package
// carries two documentation keys, a prerequisites document and a Markdown file, and the Markdown
// one is what holds the module to parsing by suffix -- yaml.safe_load succeeds on a Markdown
// document and hands back the whole thing as one string, which looks like a parse and is not.
func TestZarfPackageDocumentationExtractsTheLayer(t *testing.T) {
	zarf, err := exec.LookPath("zarf")
	if err != nil {
		t.Skip("zarf is not on PATH: the module runs the installed zarf rather than embedding one")
	}

	env := zarfPlaybookEnv(t, []string{documentationPlugin, infoBasePlugin})

	runZarfPlaybook(t, env, packageDocumentationPlaybook, nil, []string{
		"-e", "zarf_package=" + buildInspectFixture(t, env.root, zarf),
		"-e", "zarf_binary=" + zarf,
		"-e", "documentation_destination=" + t.TempDir(),
	})
}

// TestZarfClusterInfoDigestsFourQueries holds what zarf_cluster_info reports, and that it keeps the
// four answers apart.
//
// The stub it runs against reads its arguments, unlike the other stubs in this suite, and that is
// the point: a stub that printed one document whatever it was asked would show that the module can
// parse a document, not that it asked four questions and kept the answers straight. The stub fails
// loudly on a command it does not recognise for the same reason -- an empty answer is a cluster
// that looks empty, and an empty cluster satisfies nothing.
func TestZarfClusterInfoDigestsFourQueries(t *testing.T) {
	env := zarfPlaybookEnv(t, []string{clusterInfoPlugin, infoBasePlugin})
	stub := env.executable(t, clusterStub)

	runZarfPlaybook(t, env, clusterInfoPlaybook, []string{
		envClusterDir + "=" + filepath.Join(env.root, clusterFixtureDir),
	}, []string{
		"-e", "zarf_binary=" + stub,
	})
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
