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

package cluster

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestDeployPackagesRoleWalk drives the same three packages TestBundleOrderWalk does, through
// colonel_byte.zarf.deploy_packages rather than task by task.
//
// What it proves that the module walks cannot is the role's decision: that it inspects each
// package, reads the components the cluster reports against the ones the item asks for, and runs
// zarf only for the difference. The second invocation of the role in the playbook asks for
// everything the first one deployed, so every item has to be skipped -- and a skipped item writes
// no result file, which is what the second half of this test reads.
//
// The order is the initialised-first one. The role can express the provider-first order too, now
// that an item carries `connected`, and nothing covers that -- see testdata/deploy-packages-role.yml.
func TestDeployPackagesRoleWalk(t *testing.T) {
	requireSuite(t)

	cluster := withCluster(t, storedConfig, storedCluster)

	out, results := cluster.walk(t, deployPackagesPlaybook)

	// The first invocation named no log_dest, so these are the names the role built: the item's
	// position, then the package's file name. Found by position rather than spelled out, because
	// the package paths come from the suite and an offline run points them somewhere else -- and
	// because the position prefix is the half of the rule worth holding. Two items of this stack
	// name the same init package, and a name built from the package alone would leave two results
	// where there should be three.
	assertInitStep(t, readStep(t, results, stepByPosition(t, results, 1)), initSeedComponents, out)
	assertDeployStep(t, readStep(t, results, stepByPosition(t, results, 2)), out)
	assertInitStep(t, readStep(t, results, stepByPosition(t, results, 3)), initRestComponents, out)

	// What the second invocation of the same stack did, which is two different answers.
	//
	// The storage package is deployed by one item, so the cluster's record of it is complete and
	// the skip settles. The init package is deployed by two, and `zarf package list` reports only
	// the most recent deploy's components, so each phase sees the other's and both run again --
	// see docs/agent/choice-deploy-packages-drift-scope.md. This asserts the behaviour as it is
	// rather than as it should be, so that the day zarf retains the union, this fails and says so.
	//
	// The two expected files are also what stops the skip assertion passing for the wrong reason:
	// a second invocation that never ran at all would write none of the three.
	for _, step := range []string{"4-init-rerun.json", "6-reinit-rerun.json"} {
		if _, err := os.Stat(filepath.Join(results, step)); err != nil {
			t.Errorf("the phased init item wrote no %s, so the second invocation did not run it: %v",
				step, err)
		}
	}
	if _, err := os.Stat(filepath.Join(results, "5-storage-rerun.json")); err == nil {
		t.Error("the role re-deployed the storage package instead of skipping it; one item " +
			"deploys it, so the cluster's record is complete and the skip should have settled")
	}

	// app-fluxcd: an item naming no components deploys the package's one required component, and
	// naming that component afterwards skips. This is the converging half of drift detection, and
	// the omitted-components path that used to kill the play before it reached zarf.
	assertStepRan(t, readStep(t, results, "7-fluxcd-no-components.json"), fluxcdComponents, out)
	if _, err := os.Stat(filepath.Join(results, "8-fluxcd-named-rerun.json")); err == nil {
		t.Error("the role re-deployed app-fluxcd instead of skipping it, so a single-component " +
			"package does not converge either")
	}

	// The whole directory, not just the absences above. Every invocation names its own log_dest,
	// so each step that ran wrote exactly one file and the set is known. Counting absences alone
	// would miss a regression in log_dest handling: the role's own naming restarts its position
	// count at 1 per invocation, so a second invocation that fell back to it would overwrite the
	// first invocation's results rather than appearing as new files, and three absent names would
	// still be absent.
	// Six: the three the first invocation named by position, the two phased init items that
	// re-deployed on the second, and app-fluxcd's no-components deploy. The two that must not
	// appear -- 5-storage-rerun.json and 8-fluxcd-named-rerun.json -- are asserted absent above;
	// this is what catches a file nobody predicted.
	const want = 6
	if wrote := resultNames(t, results); len(wrote) != want {
		t.Errorf("the walk wrote %d result files, want %d: %v", len(wrote), want, wrote)
	}

	assertClusterInitialised(t, cluster)
	assertStorageClass(t, cluster)
	assertRegistryPVCBound(t, cluster)
	assertRegistryProxyMode(t, cluster)
}

// stepByPosition returns the name of the one result file the role wrote for the given position in
// the stack, failing when there is not exactly one.
//
// The role names a result `<position>-<package file name>.json`, and the position is what this
// looks for. Matching the whole name would mean spelling the package's file name here, which an
// offline run overrides, or rebuilding it from the path -- a second copy of the role's naming rule
// that would agree with the role right up until one of them changed.
// fluxcdComponents is app-fluxcd's one component, which is `required: true` -- so it is what a
// deploy that names no components deploys.
var fluxcdComponents = []string{"fluxcd-core"} //nolint:gochecknoglobals

func stepByPosition(t *testing.T, results string, position int) string {
	t.Helper()

	// shortcut: position 1 also matches 10-, so this breaks on a stack of ten or more items,
	// presenting as "step 1 wrote 2 result files". Upgrade when a role walk needs ten items -- by
	// filtering the matches on the digits before the first dash rather than by a cleverer glob,
	// which cannot express "not followed by a digit".
	matches, err := filepath.Glob(filepath.Join(results, fmt.Sprintf("%d-*.json", position)))
	if err != nil {
		t.Fatalf("unable to look for the result of step %d: %v", position, err)
	}
	if len(matches) != 1 {
		// What is actually there, because "want exactly 1" with nothing to compare against
		// reads as the step having failed rather than as its result being named something else.
		entries, readErr := os.ReadDir(results)
		if readErr != nil {
			t.Fatalf("step %d wrote %d result files, want exactly 1, and the result directory "+
				"cannot be listed: %v", position, len(matches), readErr)
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("step %d wrote %d result files, want exactly 1; the directory holds %v",
			position, len(matches), names)
	}
	return filepath.Base(matches[0])
}

// resultNames is every file the walk wrote into the result directory, sorted.
func resultNames(t *testing.T, results string) []string {
	t.Helper()
	entries, err := os.ReadDir(results)
	if err != nil {
		t.Fatalf("unable to list the result directory: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}
