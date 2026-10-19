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
	"testing"
)

// The fixture this layer's flow is driven over.
const packageDocumentationPlaybook = "test/e2e/noncluster/testdata/zarf-package-documentation.yml"

// The plugin this playbook exercises. It is read before the playbook runs, for the reason given at
// stateInfoPlugin: Go's test cache keys on what the test process opens, and everything here is
// opened by the ansible-playbook child instead -- so without the read, editing a plugin leaves a
// passing result cached.
var documentationPlugin = "ansible/colonel_byte/zarf/plugins/action/zarf_package_documentation.py"

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
