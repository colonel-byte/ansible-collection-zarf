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
	"path/filepath"
	"testing"
)

// The fixtures the cluster reads are driven over.
const (
	clusterInfoPlaybook = "test/e2e/noncluster/testdata/zarf-cluster-info.yml"
	clusterStub         = "test/e2e/noncluster/testdata/zarf-cluster-stub.sh"
	clusterFixtureDir   = "test/e2e/noncluster/testdata"
)

// The environment the cluster stub reads: the directory holding its recorded answers.
const envClusterDir = "ZARF_ANSIBLE_E2E_CLUSTER_DIR"

// The plugin this playbook exercises, read before the playbook runs for the reason given at
// documentationPlugin.
var clusterInfoPlugin = "ansible/colonel_byte/zarf/plugins/action/zarf_cluster_info.py"

// TestZarfClusterInfoDigestsFiveQueries holds what zarf_cluster_info reports, and that it keeps the
// four answers apart.
//
// The stub it runs against reads its arguments, unlike the other stubs in this suite, and that is
// the point: a stub that printed one document whatever it was asked would show that the module can
// parse a document, not that it asked four questions and kept the answers straight. The stub fails
// loudly on a command it does not recognise for the same reason -- an empty answer is a cluster
// that looks empty, and an empty cluster satisfies nothing.
func TestZarfClusterInfoDigestsFiveQueries(t *testing.T) {
	env := zarfPlaybookEnv(t, []string{clusterInfoPlugin, infoBasePlugin})
	stub := env.executable(t, clusterStub)

	runZarfPlaybook(t, env, clusterInfoPlaybook, []string{
		envClusterDir + "=" + filepath.Join(env.root, clusterFixtureDir),
	}, []string{
		"-e", "zarf_binary=" + stub,
	})
}
