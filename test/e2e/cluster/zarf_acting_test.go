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
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// removedPackage is the package acting-modules.yml takes off the cluster. The removal is measured
// by asking the cluster what is deployed rather than by reading zarf's report of the removal, which
// is `unknown` by design: see runPackageRemove.
//
// What is deliberately not asserted is the provisioner Deployment or the StorageClass going away.
// One of the two clusters this suite builds has k3s local-storage enabled, so a `local-path`
// StorageClass and a `local-path-provisioner` Deployment exist there whether or not the package is
// deployed -- asserting their absence would fail on the cluster that has storage of its own, which
// is the opposite of what the removal means.
const removedPackage = "csi-local-path-provider"

// testActingModules runs the five acting modules the walks do not cover against the cluster a walk
// has initialised: zarf_kubectl, zarf_package_pull, zarf_package_mirror_resources,
// zarf_update_creds and zarf_package_remove.
//
// It runs last on this cluster and nothing in the suite runs after it, because two of those five
// change what the rest of the suite assumes: the removal takes away a package the read-only module
// tests expect to find, and the rotation rewrites credentials the cluster is using.
//
// What it adds over test/e2e/noncluster, which covers every refusal and every check-mode answer
// with no cluster at all, is the half that cannot be faked -- a mirror against the registry `zarf
// init` stood up, a rotation against the state it wrote, and a removal whose effect the cluster is
// asked about afterwards.
func testActingModules(t *testing.T, cluster *cluster) {
	t.Helper()

	// The premise of the removal below -- that the storage provider is deployed -- is what the
	// read-only module tests just asserted out of `zarf package list`. It is not re-checked by
	// looking for the StorageClass: one of this suite's two clusters has k3s local-storage
	// enabled and carries a local-path class of its own, so that would prove nothing here.
	_, results := cluster.walk(t, actingModulesPlaybook)

	assertPullFetchedOnePackage(t, readStep(t, results, "acting-pull.json"))
	assertMirrorPushedWithoutAComponentList(t, readStep(t, results, "acting-mirror.json"))
	assertPackageNoLongerDeployed(t, readStep(t, results, "acting-remaining.json"))
	assertCredentialsUntouchedByCheckMode(t, cluster)
}

// assertPullFetchedOnePackage reads the pull's own report. The playbook already asserted the shape;
// what is read here is the filename, because that is what a playbook does with the result -- it
// deploys the file the pull names.
func assertPullFetchedOnePackage(t *testing.T, result map[string]any) {
	t.Helper()

	detail := zarfDetail(t, result)
	pulled := stringsOf(t, detail["packagesPulled"])
	if len(pulled) != 1 {
		t.Fatalf("the pull reported %v, want exactly one package file", pulled)
	}
	if !strings.HasPrefix(pulled[0], "zarf-package-"+removedPackage) {
		t.Errorf("the pull fetched %q, want the storage provider package", pulled[0])
	}
	if !strings.HasSuffix(pulled[0], ".tar.zst") {
		t.Errorf("the pull fetched %q, which is not a package tarball", pulled[0])
	}
}

// assertMirrorPushedWithoutAComponentList holds the mirror's report against what a mirror can
// know. It was written the other way round first -- expecting the component list and the partial
// signal the converging modules report -- and this suite is what showed that zarf announces the
// images a mirror pushes and never the component they came from. The module reports `unknown` for
// that reason, and an empty component list here is the assertion rather than an omission.
func assertMirrorPushedWithoutAComponentList(t *testing.T, result map[string]any) {
	t.Helper()

	detail := zarfDetail(t, result)
	if ran := stringsOf(t, detail["componentsRan"]); len(ran) != 0 {
		t.Errorf("the mirror named the components %v: zarf announces the images, so a list here "+
			"means the progress reader started matching something it should not", ran)
	}
	signal, ok := detail["changedSignal"].(string)
	if !ok || signal != "unknown" {
		t.Errorf("the mirror reported the signal %v, want unknown", detail["changedSignal"])
	}
	if code, ok := detail["exitCode"].(float64); ok && code != 0 {
		t.Errorf("the mirror exited %v", code)
	}
}

// assertCredentialsUntouchedByCheckMode is the other half of the check-mode rotation the playbook
// asks for: the module reported a skip, and the cluster still holds credentials. Neither
// subcommand can complete against the cluster this suite builds -- the playbook's header names
// both zarf-side reasons -- so what is left to check is that asking in check mode left the
// cluster's own credentials alone rather than blanking them on the way to refusing.
func assertCredentialsUntouchedByCheckMode(t *testing.T, c *cluster) {
	t.Helper()
	clientset := kubeClient(t, c.kubeconfig)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	secret, err := clientset.CoreV1().Secrets("zarf").Get(ctx, "zarf-state", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("the zarf-state secret is not there: %v", err)
	}
	var state struct {
		RegistryInfo struct {
			PullPassword string `json:"pullPassword"`
			PushPassword string `json:"pushPassword"`
		} `json:"registryInfo"`
	}
	if err := json.Unmarshal(secret.Data["state"], &state); err != nil {
		t.Fatalf("unable to read zarf's state: %v", err)
	}

	if state.RegistryInfo.PushPassword == "" || state.RegistryInfo.PullPassword == "" {
		t.Error("the cluster's registry credentials are empty after a check-mode rotation")
	}
	// The value the playbook offered must not be anywhere near the cluster.
	if strings.Contains(state.RegistryInfo.PullPassword, "FIXTURE-") {
		t.Error("a check-mode rotation wrote the credential it was given")
	}
}

// assertPackageNoLongerDeployed reads what zarf says is on the cluster after the removal. The
// playbook asserted the same thing about the dictionary it was handed; this reads the recording,
// which is the path a developer takes when it fails.
func assertPackageNoLongerDeployed(t *testing.T, result map[string]any) {
	t.Helper()

	entries, ok := result["packages"].([]any)
	if !ok {
		t.Fatalf("the package list carried no packages: %v", result)
	}

	var deployed []string
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("a package record was not an object: %v", entry)
		}
		name, ok := record["package"].(string)
		if !ok {
			t.Fatalf("a package record carried no name: %v", record)
		}
		deployed = append(deployed, name)
	}

	for _, name := range deployed {
		if name == removedPackage {
			t.Errorf("%s is still deployed, so the removal removed nothing", removedPackage)
		}
	}
	// The other way round: a removal that took the init package with it would leave this list
	// empty, and an empty list satisfies every assertion about what is gone.
	if !slices.Contains(deployed, "init-slim") {
		t.Errorf("the cluster lists %v, and the init package is not among them: the removal took "+
			"more than it was asked to", deployed)
	}
}
