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

// Package cluster holds the e2e suite for the colonel_byte.zarf Ansible collection: real k3d
// clusters, a real zarf, and the uds-bundle's four packages walked by the two modules.
//
// The suite proves the whole path rather than the Go half of it. The unit tests in
// internal/zarfmod cover the argument vectors, the WANT_JSON intake, and the progress reader
// against a fake runner; the container smoke checks in containers/ansible/Dockerfile cover the
// module contract with no cluster. What is left -- that the action plugins deliver parameters to
// the wrappers, that the command lines are ones zarf accepts, and that the components the modules
// report are the ones that reached the cluster -- needs a cluster, and that is what this is.
//
// Two clusters, because the walk differs by what storage the cluster has. See
// testdata/k3d-config.yaml and testdata/k3d-config-local-storage.yaml, and the playbooks beside
// them.
//
// It needs k3d, a zarf on PATH, ansible-playbook, Docker, and the network to pull three packages
// once. Nothing here runs under `go test ./...`: it is driven by `mage test:endToEndCluster`, and
// every test skips with a reason when a prerequisite is missing.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/colonel-byte/ansible-collection-zarf/internal/testenv"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// The cluster fixtures and the names inside them. The names are spelled out here as well because
// the suite asks k3d about a cluster before it has parsed anything, and because the refusal to
// adopt an existing cluster must not depend on reading the file correctly.
const (
	bareConfig    = "test/e2e/cluster/testdata/k3d-config.yaml"
	bareCluster   = "zarf"
	storedConfig  = "test/e2e/cluster/testdata/k3d-config-local-storage.yaml"
	storedCluster = "zarf-local-storage"
)

const (
	providerFirstPlaybook  = "test/e2e/cluster/testdata/provider-first.yml"
	bundleOrderPlaybook    = "test/e2e/cluster/testdata/bundle-order.yml"
	infoModulesPlaybook    = "test/e2e/cluster/testdata/info-modules.yml"
	actingModulesPlaybook  = "test/e2e/cluster/testdata/acting-modules.yml"
	deployPackagesPlaybook = "test/e2e/cluster/testdata/deploy-packages-role.yml"
	packagePublicKey       = "test/e2e/cluster/testdata/colonel-byte-zarf-packages.pub"
)

// The module files the suite drives. mage test:endToEndCluster builds them; see
// magefiles/pkg/build.ZarfModuleFiles.
//
// wrapperDir is what the playbooks covering the other modules are given instead of a path apiece:
// every module file is a symlink to one binary in that directory, named after the module, so a
// playbook that names `{{ zarf_wrapper_dir }}/zarf_package_pull` needs nothing added here.
const (
	initWrapper   = "build/zarf_init"
	deployWrapper = "build/zarf_package_deploy"
	wrapperDir    = "build"
)

const (
	// packageCache is where the packages are kept between runs. It is under build/ because three
	// package downloads are not something to repeat on every run, and build/ is the directory a
	// clean removes.
	packageCache = "build/tmp/zarf-e2e"

	// The two packages come from the init-local-path uds-bundle, which is the baseline this suite
	// walks:
	// https://github.com/colonel-byte/uds-bundles/blob/main/upstream/init-local-path/uds-bundle.yaml
	//
	// Both tags are pinned here rather than resolved from the bundle at run time, because a suite
	// that read the bundle would be testing whatever was published this morning. They are also not
	// quite the bundle's own strings: the bundle asks for init-slim:v0.88.0-upstream and
	// csi-local-path-provider:v0.0.37-upstream, and neither tag exists -- the init publishes
	// v0.88.0 with no suffix, and the provider dropped its leading v at 0.0.37 (v0.0.36-upstream
	// still has one). These are the tags that resolve.
	//
	// initPackageRef is the slim init, whose five components are all optional. That is what lets
	// the walks deploy the seed phase and the rest of init as separate steps, which is the whole
	// shape of the bundle; the standard zarf init package has no optional components to pick.
	initPackageRef  = "oci://ghcr.io/colonel-byte/zarf/init-slim:v0.88.0"
	initPackageFile = "zarf-init-%s-v0.88.0.tar.zst"
	// fluxcdPackageSource is the reference package this repository builds for the suite rather
	// than pulls. It is the one package here with a single component, and that component is
	// `required: true`, so a deploy that names no components deploys exactly it -- which is the
	// only way to exercise the role's omitted-components path against a real zarf. Its own
	// prerequisites are an initialised cluster and Kubernetes 1.30, not Flux, because it is Flux.
	fluxcdPackageSource = "examples/packages/app-fluxcd"
	// fluxcdPackageGlob finds what `zarf package create` wrote, rather than naming the version.
	// metadata.version there is machine-maintained from resources.json by `mage examples:vendor`,
	// so a hard-coded version is a test that breaks on the next vendor run.
	fluxcdPackageGlob = "zarf-package-app-fluxcd-%s-*.tar.zst"

	// storagePackage is the uds-bundle's local-path storage provider.
	storagePackage = "oci://ghcr.io/colonel-byte/zarf/csi-local-path-provider:0.0.37-upstream"
	// storagePackageFile is what `zarf package pull` writes that reference to.
	storagePackageFile = "zarf-package-csi-local-path-provider-%s-0.0.37-upstream.tar.zst"

	// localPathVolume is the host directory both k3d configs mount into the nodes. k3d does not
	// create it, and a missing bind source fails the cluster create.
	localPathVolume = "/tmp/k3d"
)

// Environment variables the suite reads.
const (
	// envInitPackage names an init package to use instead of pulling one. It is what an offline
	// run sets, and what to set when testing against a package built from a branch.
	envInitPackage = "ZARF_ANSIBLE_E2E_INIT_PACKAGE"
	// envStoragePackage names a storage provider package to use instead of pulling one.
	envStoragePackage = "ZARF_ANSIBLE_E2E_STORAGE_PACKAGE"
	// envFluxCDPackage names an app-fluxcd package to use instead of building one.
	envFluxCDPackage = "ZARF_ANSIBLE_E2E_FLUXCD_PACKAGE"
	// envKeep leaves the clusters running after the suite, for looking at what the modules did.
	envKeep = "ZARF_ANSIBLE_E2E_KEEP"
)

// environment is what the suite resolved at startup.
type environment struct {
	root           string
	arch           string
	zarf           string
	ansible        string
	initWrapper    string
	deployWrapper  string
	initPackage    string
	storagePackage string
	fluxcdPackage  string
	publicKey      string
	// collections is the directory handed to Ansible as its collections path. It holds an
	// ansible_collections symlink into the repository, which is the layout Ansible insists on and
	// the repository does not have.
	collections string
	// skip is why the suite cannot run, empty when it can.
	skip string
}

var suite environment //nolint:gochecknoglobals

func TestMain(m *testing.M) {
	e2e, root, err := testenv.Bootstrap()
	if err != nil {
		log.Fatal(err)
	}
	suite.root = root
	suite.arch = e2e.Arch
	runTmp, err := testenv.RunTempDir("zarf-e2e-run-")
	if err != nil {
		log.Fatal(err)
	}

	suite.skip = prepare(&suite)

	code := m.Run()

	if err := os.RemoveAll(runTmp); err != nil {
		log.Printf("unable to remove the suite's temporary directory: %v", err)
	}

	os.Exit(code)
}

// prepare resolves everything the suite needs and returns why it cannot run, empty when it can.
//
// Nothing here is fatal. A developer without k3d installed should get a skip that says so, not a
// failure that reads as the modules being broken.
func prepare(env *environment) string {
	if runtime.GOOS != "linux" {
		return "the suite runs k3d, which needs Linux containers"
	}

	for _, tool := range []string{"docker", "k3d"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Sprintf("%s is not on PATH", tool)
		}
	}

	ansible, err := exec.LookPath("ansible-playbook")
	if err != nil {
		return "ansible-playbook is not on PATH: the suite drives the modules through a playbook, " +
			"because the action plugins are half of what it is proving"
	}
	env.ansible = ansible

	zarfPath, err := exec.LookPath("zarf")
	if err != nil {
		return "zarf is not on PATH: the modules run the installed zarf rather than embedding one"
	}
	env.zarf = zarfPath

	for _, wrapper := range []struct {
		path string
		into *string
	}{
		{path: initWrapper, into: &env.initWrapper},
		{path: deployWrapper, into: &env.deployWrapper},
	} {
		absolute := filepath.Join(env.root, wrapper.path)
		if _, err := os.Stat(absolute); err != nil {
			return fmt.Sprintf("the module wrapper %s is not built: run mage test:endToEndCluster, "+
				"which builds both", wrapper.path)
		}
		*wrapper.into = absolute
	}
	env.publicKey = filepath.Join(env.root, packagePublicKey)

	for _, cluster := range []string{bareCluster, storedCluster} {
		if clusterExists(cluster) {
			// Refusing rather than adopting. The suite deletes the clusters it created, and a
			// cluster somebody else is using is not one to delete on their behalf.
			return fmt.Sprintf("a k3d cluster named %q already exists; delete it with "+
				"`k3d cluster delete %s` before running this suite", cluster, cluster)
		}
	}

	if reason := resolvePackages(env); reason != "" {
		return reason
	}

	collections, err := stageCollections(env.root)
	if err != nil {
		return fmt.Sprintf("unable to stage the collections path: %v", err)
	}
	env.collections = collections

	return ""
}

// resolvePackages finds or pulls the two packages the walks deploy.
//
// Both tags are pinned to the uds-bundle's, rather than derived from the zarf on PATH as the init
// package's once was. That derivation stopped meaning anything when the modules linked zarf in:
// the zarf that deploys the init package is the one compiled into the module, so the installed
// zarf's version says nothing about which package belongs here. The installed zarf is still what
// pulls them, which is why it remains a prerequisite.
func resolvePackages(env *environment) string {
	dir := filepath.Join(env.root, packageCache)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("unable to create %s: %v", dir, err)
	}

	// The slim init goes in a subdirectory of its own, which is not tidiness. `zarf package pull`
	// names its output after the package's kind and version, and for any ZarfInitConfig of this
	// version that is zarf-init-<arch>-v0.88.0.tar.zst -- the same name the standard zarf-dev
	// init package pulls to. A cache that had ever held the standard one would serve it here, and
	// the walk would quietly test the wrong package: the standard init's components are all
	// required, so every step would deploy all of them and the bundle's split would mean nothing.
	initDir := filepath.Join(dir, "init-slim")
	if err := os.MkdirAll(initDir, 0o755); err != nil {
		return fmt.Sprintf("unable to create %s: %v", initDir, err)
	}
	initName := fmt.Sprintf(initPackageFile, env.arch)
	pkg, reason := havePackage(env, initDir, initName, initPackageRef, envInitPackage)
	if reason != "" {
		return reason
	}
	env.initPackage = pkg

	storageName := fmt.Sprintf(storagePackageFile, env.arch)
	pkg, reason = havePackage(env, dir, storageName, storagePackage, envStoragePackage)
	if reason != "" {
		return reason
	}
	env.storagePackage = pkg

	pkg, reason = haveCreatedPackage(env, dir)
	if reason != "" {
		return reason
	}
	env.fluxcdPackage = pkg

	return ""
}

// haveCreatedPackage returns the app-fluxcd package, building it when it is not already cached,
// and why it could not be had. The environment variable wins over both.
//
// It is built rather than pulled because nothing publishes it: it is a reference package in this
// repository, and `mage examples:lint` is all that has ever read it. The create needs the network
// once, for the four Flux controller images, and then the cache serves it like the pulled ones.
func haveCreatedPackage(env *environment, cacheDir string) (string, string) {
	if named := os.Getenv(envFluxCDPackage); named != "" {
		if _, err := os.Stat(named); err != nil {
			return "", fmt.Sprintf("%s names %s, which cannot be read: %v", envFluxCDPackage, named, err)
		}
		return named, ""
	}

	dir := filepath.Join(cacheDir, "app-fluxcd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Sprintf("unable to create %s: %v", dir, err)
	}

	pattern := filepath.Join(dir, fmt.Sprintf(fluxcdPackageGlob, env.arch))
	if matches, err := filepath.Glob(pattern); err == nil && len(matches) == 1 {
		return matches[0], ""
	}

	source := filepath.Join(env.root, fluxcdPackageSource)
	log.Printf("building %s into %s", source, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, env.zarf, "package", "create", source,
		"--confirm", "--architecture", env.arch, "--output-directory", dir, "--no-color")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Sprintf("unable to build %s: %v; set %s to a package already on disk",
			fluxcdPackageSource, err, envFluxCDPackage)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) != 1 {
		return "", fmt.Sprintf("the build of %s produced %d packages matching %s, want exactly 1",
			fluxcdPackageSource, len(matches), pattern)
	}
	return matches[0], ""
}

// havePackage returns the package at name inside dir, pulling ref when it is not already there,
// and why it could not be had. The environment variable wins over both.
func havePackage(env *environment, dir, name, ref, envName string) (string, string) {
	if named := os.Getenv(envName); named != "" {
		if _, err := os.Stat(named); err != nil {
			return "", fmt.Sprintf("%s names %s, which cannot be read: %v", envName, named, err)
		}
		return named, ""
	}

	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return path, ""
	}

	log.Printf("pulling %s into %s", ref, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, env.zarf, "package", "pull", ref,
		"--architecture", env.arch, "--output-directory", dir, "--no-color")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Sprintf("unable to pull %s: %v; set %s to a package already on disk",
			ref, err, envName)
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Sprintf("the pull of %s did not produce %s", ref, path)
	}
	return path, ""
}

// stageCollections builds the directory Ansible wants: a collections path holding an
// ansible_collections directory whose children are the namespaces.
//
// The repository's tree is ansible/colonel_byte/<collection>, which is the same shape one level
// down, so a symlink named ansible_collections pointing at ansible/ is the whole of it. It is a
// symlink rather than a copy so that a run exercises the working tree: editing an action plugin
// and running the suite again has to pick the edit up.
func stageCollections(root string) (string, error) {
	dir, err := os.MkdirTemp("", "cargoship-zarf-e2e-collections-")
	if err != nil {
		return "", err
	}
	if err := os.Symlink(filepath.Join(root, "ansible"), filepath.Join(dir, "ansible_collections")); err != nil {
		return "", err
	}
	return dir, nil
}

// cluster is a k3d cluster the suite created, and the kubeconfig that reaches it.
type cluster struct {
	name       string
	kubeconfig string
}

// withCluster creates the cluster the given fixture describes and removes it when the test ends.
//
// Both k3d configs ask k3d to write the cluster into the default kubeconfig and switch to it. A
// test suite has no business doing either, so both are overridden on the command line and the
// kubeconfig is written somewhere of its own. The fixtures stay as the upstream file.
func withCluster(t *testing.T, config, name string) *cluster {
	t.Helper()

	if err := os.MkdirAll(localPathVolume, 0o755); err != nil {
		t.Fatalf("unable to create the bind source %s: %v", localPathVolume, err)
	}

	t.Logf("creating the k3d cluster %q from %s", name, config)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	create := exec.CommandContext(ctx, "k3d", "cluster", "create",
		"--config", filepath.Join(suite.root, config),
		"--kubeconfig-update-default=false",
		"--kubeconfig-switch-context=false",
		"--wait")
	create.Stdout = os.Stderr
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		t.Fatalf("unable to create the cluster: %v", err)
	}

	t.Cleanup(func() {
		if os.Getenv(envKeep) != "" {
			t.Logf("%s is set, leaving the cluster %q in place", envKeep, name)
			return
		}
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer deleteCancel()
		del := exec.CommandContext(deleteCtx, "k3d", "cluster", "delete", name)
		del.Stdout = os.Stderr
		del.Stderr = os.Stderr
		if err := del.Run(); err != nil {
			t.Errorf("unable to delete the cluster %q: %v", name, err)
		}
	})

	out, err := exec.CommandContext(ctx, "k3d", "kubeconfig", "get", name).Output()
	if err != nil {
		t.Fatalf("unable to read the cluster's kubeconfig: %v", err)
	}
	kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(kubeconfig, out, 0o600); err != nil {
		t.Fatalf("unable to write %s: %v", kubeconfig, err)
	}

	// One server and one agent, per both fixtures. Waiting for both matters because the registry
	// the initialisation deploys can be scheduled onto either.
	clientset := kubeClient(t, kubeconfig)
	if err := testenv.WaitForNodesReady(ctx, clientset, 2, 5*time.Minute); err != nil {
		t.Fatalf("the cluster's nodes did not become ready: %v", err)
	}

	return &cluster{name: name, kubeconfig: kubeconfig}
}

func clusterExists(name string) bool {
	out, err := exec.Command("k3d", "cluster", "list", name, "--no-headers").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func kubeClient(t *testing.T, kubeconfig string) *kubernetes.Clientset {
	t.Helper()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("unable to build a client for %s: %v", kubeconfig, err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("unable to build a client for %s: %v", kubeconfig, err)
	}
	return clientset
}

// requireSuite skips the calling test when a prerequisite was missing, naming the one that was.
func requireSuite(t *testing.T) {
	t.Helper()
	// Checked here rather than in prepare: testing.Short reads a flag, and flags are not parsed
	// until m.Run, which is after TestMain has called prepare.
	if testing.Short() {
		t.Skip("the suite needs k3d clusters, which -short excludes")
	}
	if suite.skip != "" {
		t.Skipf("the zarf module e2e suite cannot run: %s", suite.skip)
	}
}

// walk runs one of the playbooks against a cluster and returns its output and the directory the
// per-step results were written to.
func (c *cluster) walk(t *testing.T, playbook string) (string, string) {
	t.Helper()

	results := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, suite.ansible, filepath.Join(suite.root, playbook),
		"-v", // the progress the action plugins display is verbose-level output
		"-i", "localhost,",
		"-e", "zarf_init_wrapper="+suite.initWrapper,
		"-e", "zarf_deploy_wrapper="+suite.deployWrapper,
		"-e", "zarf_wrapper_dir="+filepath.Join(suite.root, wrapperDir),
		"-e", "zarf_init_package="+suite.initPackage,
		"-e", "zarf_storage_package="+suite.storagePackage,
		"-e", "zarf_fluxcd_package="+suite.fluxcdPackage,
		// The reference as well as the staged file: zarf_package_pull fetches from a registry, so
		// it is the one module here that needs the thing the file was pulled from.
		"-e", "zarf_storage_package_ref="+storagePackage,
		"-e", "zarf_public_key="+suite.publicKey,
		"-e", "zarf_kubeconfig="+c.kubeconfig,
		"-e", "zarf_result_dir="+results,
		"-e", "zarf_timeout=15m",
		// No git credentials are passed to anything. The headers of bundle-order.yml and
		// provider-first.yml say why the walks do not give them to an init, and the one in
		// acting-modules.yml says why the rotation there is the registry's rather than the git
		// server's.
		// The read-only modules run the installed zarf rather than a wrapper binary, so they are
		// given its path rather than resolving it on the PATH of a test process.
		"-e", "zarf_binary="+suite.zarf,
	)
	cmd.Env = append(os.Environ(),
		"ANSIBLE_COLLECTIONS_PATH="+suite.collections,
		// A play over localhost with no inventory warns, and the warning is noise in a failure.
		"ANSIBLE_LOCALHOST_WARNING=False",
		"ANSIBLE_INVENTORY_UNPARSED_WARNING=False",
		// The suite reads the progress lines out of the output, so they have to stay unstyled.
		"ANSIBLE_FORCE_COLOR=0",
		"NO_COLOR=1",
	)

	out, runErr := cmd.CombinedOutput()
	t.Logf("ansible-playbook %s:\n%s", filepath.Base(playbook), out)
	if runErr != nil {
		t.Fatalf("the playbook failed: %v", runErr)
	}
	return string(out), results
}

// runWrapper invokes a module wrapper the way the action plugin does: the action in
// ZARF_ANSIBLE_MODULE, the parameters on stdin, and one JSON object on stdout.
//
// It returns the decoded result and the module's stderr, which is where zarf's own output went.
func runWrapper(t *testing.T, wrapper, module string, params map[string]any) (map[string]any, string) {
	t.Helper()

	payload, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("unable to encode the module parameters: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	var stdout, stderr strings.Builder
	cmd := exec.CommandContext(ctx, wrapper)
	cmd.Env = append(os.Environ(), "ZARF_ANSIBLE_MODULE="+module)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// A module reports failure inside its result and still exits 0. A non-zero status here is the
	// wrapper itself breaking, which is a different bug and worth a different message.
	if err := cmd.Run(); err != nil {
		t.Fatalf("the wrapper exited non-zero (%v); stdout %q stderr %q",
			err, stdout.String(), stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
		t.Fatalf("the wrapper did not write one JSON object: %v; stdout %q", err, stdout.String())
	}
	return result, stderr.String()
}

// readStep reads one step's registered result.
func readStep(t *testing.T, results, name string) map[string]any {
	t.Helper()
	path := filepath.Join(results, name)
	data, err := os.ReadFile(path) //nolint:gosec // the path is the test's own temporary directory
	if err != nil {
		t.Fatalf("unable to read the registered result %s: %v", name, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("the registered result %s is not JSON: %v (%s)", name, err, data)
	}
	return result
}
