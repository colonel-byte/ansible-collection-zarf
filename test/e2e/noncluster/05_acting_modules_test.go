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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	actingModulesPlaybook = "test/e2e/noncluster/testdata/acting-modules.yml"
	// The module files the playbook drives. They are symlinks to one binary, and naming them
	// individually is the point: a dispatch entry that went missing shows up as the task for that
	// module failing rather than as a missing file.
	wrapperBinary  = "build/zarfmod"
	wrapperDir     = "build"
	kubectlVersion = "kubectl-version.json"
)

// actingModulePlugins are read before the playbook runs, so that an edit to one of them
// invalidates Go's test cache. See zarfPlaybookEnv.
var actingModulePlugins = []string{ //nolint:gochecknoglobals
	"ansible/colonel_byte/zarf/plugins/action/zarf_init.py",
	"ansible/colonel_byte/zarf/plugins/action/zarf_kubectl.py",
	"ansible/colonel_byte/zarf/plugins/action/zarf_package_deploy.py",
	"ansible/colonel_byte/zarf/plugins/action/zarf_package_mirror_resources.py",
	"ansible/colonel_byte/zarf/plugins/action/zarf_package_pull.py",
	"ansible/colonel_byte/zarf/plugins/action/zarf_package_remove.py",
	"ansible/colonel_byte/zarf/plugins/action/zarf_update_creds.py",
	"ansible/colonel_byte/zarf/plugins/plugin_utils/projection.py",
}

// TestActingModulesRefuseAndSkipWithoutACluster drives all seven modules that act on a cluster
// through the paths that need no cluster: what each refuses before running anything, what each does
// in check mode, and -- for zarf_kubectl -- a command that answers for real.
//
// It is the only place outside the container image's smoke checks where argv[0] dispatch is
// exercised through Ansible for every module at once. The cluster suite walks init and deploy and
// the rest against real clusters; this covers the half an operator hits while writing the playbook,
// on a laptop with no k3d.
func TestActingModulesRefuseAndSkipWithoutACluster(t *testing.T) {
	env := zarfPlaybookEnv(t, append(actingModulePlugins, actingModulesPlaybook))

	// The wrapper is what the modules are, so a run without it would be testing nothing. Skipped
	// rather than failed, as the rest of both suites is: a contributor who has not built it gets
	// the reason.
	if _, err := os.Stat(filepath.Join(env.root, wrapperBinary)); err != nil {
		t.Skipf("the module wrapper %s is not built: run mage test:endToEndNonCluster, "+
			"which builds it", wrapperBinary)
	}

	results := t.TempDir()
	runZarfPlaybook(t, env, actingModulesPlaybook, nil, []string{
		"-e", "zarf_wrapper_dir=" + filepath.Join(env.root, wrapperDir),
		"-e", "zarf_result_dir=" + results,
	})

	// The one module whose answer is a tool's output rather than a report about a cluster, read
	// back the way a playbook would read it.
	assertVendoredKubectlAnswered(t, filepath.Join(results, kubectlVersion))
}

// assertVendoredKubectlAnswered holds the recorded kubectl version against what the module
// promises: its stdout, as kubectl wrote it, under zarf.output.
func assertVendoredKubectlAnswered(t *testing.T, path string) {
	t.Helper()

	recorded, err := os.ReadFile(path) //nolint:gosec // the suite's own temporary directory
	require.NoError(t, err, "the playbook recorded no kubectl result")

	var result struct {
		Changed bool `json:"changed"`
		Zarf    struct {
			Module  string   `json:"module"`
			Command []string `json:"command"`
			Output  string   `json:"output"`
			Version string   `json:"version"`
		} `json:"zarf"`
	}
	require.NoError(t, json.Unmarshal(recorded, &result))

	require.Equal(t, "kubectl", result.Zarf.Module)
	// The version is the zarf compiled into the wrapper, which is what makes the kubectl in it
	// the one being reported on.
	require.NotEmpty(t, result.Zarf.Version, "the result does not say which zarf answered")

	// The output is kubectl's own JSON. Decoded rather than matched on a substring, because what
	// the module promises is that the bytes arrive unaltered apart from the trailing newline.
	var version struct {
		ClientVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"clientVersion"`
		KustomizeVersion string `json:"kustomizeVersion"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Zarf.Output), &version),
		"zarf.output did not decode as the JSON kubectl was asked for: %q", result.Zarf.Output)
	require.NotEmpty(t, version.ClientVersion.GitVersion,
		"kubectl reported no client version: %q", result.Zarf.Output)
	require.NotEmpty(t, version.KustomizeVersion,
		"kubectl reported no kustomize version, so the output is not the vendored kubectl's: %q",
		result.Zarf.Output)
}
