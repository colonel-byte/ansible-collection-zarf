// Copyright 2026 colonel-byte
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package testrunner implements runners for the unit and end-to-end test suites.
package testrunner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/build"
	"github.com/magefile/mage/sh"
)

// modulePath is this module, used to tell the e2e packages apart from the rest in `go list`.
const modulePath = "github.com/colonel-byte/ansible-collection-zarf"

// RunE2ENoBuild runs go test against pkg with the temporary directory configured for e2e suites.
func RunE2ENoBuild(timeout string, pkg string, extra ...string) error {
	e2eTmpDir, err := filepath.Abs(filepath.Join(build.BuildDir, "tmp"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(e2eTmpDir, 0o755); err != nil {
		return err
	}
	args := append([]string{"test", "-timeout=" + timeout, pkg, "-count=1", "-v"}, extra...)
	return sh.RunWithV(
		map[string]string{
			"ZARF_ANSIBLE_E2E_TMPDIR": e2eTmpDir,
			"TMPDIR":                  e2eTmpDir,
		},
		"go",
		args...,
	)
}

// RunE2ECluster builds the Ansible module files, then runs the suite that needs a cluster.
//
// The timeout is generous because the suite stands up two k3d clusters and walks four zarf
// packages across each of them; the first run also pulls the packages.
func RunE2ECluster() error {
	if err := build.ZarfModuleFiles(); err != nil {
		return err
	}
	return RunE2ENoBuild("90m", modulePath+"/test/e2e/cluster/...")
}

// RunE2ENonCluster runs the suite that needs no cluster: the read-only modules against a stub
// zarf, the state redaction, and the acting modules' refusals and check-mode answers.
//
// It builds the module wrapper, which it did not have to while every module it exercised was an
// action plugin with no binary behind it -- see docs/agent/choice-zarf-info-modules.md. The
// acting modules are that binary, and the paths covered here need no cluster: a refusal happens
// before zarf is reached, check mode runs nothing, and `kubectl version --client` answers from
// inside the binary. The test skips with a reason when the wrapper is missing, so `go test` over
// the package still works without this target.
func RunE2ENonCluster() error {
	if err := build.ZarfModuleFiles(); err != nil {
		return err
	}
	return RunE2ENoBuild("20m", modulePath+"/test/e2e/noncluster/...")
}

// K3dClusters are the clusters the cluster suite creates, named in
// test/e2e/cluster/testdata/k3d-config.yaml and k3d-config-local-storage.yaml.
var K3dClusters = []string{"zarf", "zarf-local-storage"} //nolint:gochecknoglobals

// DeleteK3dClusters removes the clusters the cluster suite creates, for a run that was
// interrupted before its own cleanup ran. A cluster that is not there is not an error.
func DeleteK3dClusters() error {
	for _, name := range K3dClusters {
		out, err := sh.Output("k3d", "cluster", "list", name, "--no-headers")
		if err != nil || strings.TrimSpace(out) == "" {
			continue
		}
		fmt.Printf("Deleting the k3d cluster %s\n", name)
		if err := sh.RunV("k3d", "cluster", "delete", name); err != nil {
			return err
		}
	}
	return nil
}

// Unit runs go test over every package except the e2e suites under test/, which need Ansible,
// k3d and Docker and have runners of their own. Those are driven by Test.EndToEnd* instead.
//
// build/ is excluded for the reason .golangci.yaml gives about the same directory: the e2e targets
// point TMPDIR at build/tmp, so a run that is in flight -- or one that was interrupted -- leaves a
// go-build tree of generated cgo there. `go list ./...` walks the filesystem rather than the index,
// so build/.gitignore does not keep it out, and listing it fails the whole command with
// "use of internal package internal/runtime/sys not allowed" before a single test runs. The
// failure names the toolchain's own source and reads as a broken checkout rather than as leftovers.
func Unit() error {
	// -e is what makes the build/ exclusion below reachable. Without it `go list` fails on the
	// first unbuildable directory it walks into and returns nothing, so there is no list left to
	// filter -- the leftovers would take the command down before it could skip them.
	out, err := sh.Output("go", "list", "-e", "./...")
	if err != nil {
		return err
	}

	pkgs := unitPackages(out)
	if len(pkgs) == 0 {
		return fmt.Errorf("go list ./... returned no packages to test")
	}

	return sh.RunV("go", append([]string{"test", "-count=1"}, pkgs...)...)
}

// unitPackages is the package list `go list -e ./...` printed, minus the two trees Unit does not
// test: the e2e suites, which have runners of their own, and anything under build/.
func unitPackages(listed string) []string {
	const (
		e2ePrefix   = modulePath + "/test/"
		buildPrefix = modulePath + "/build/"
	)

	var pkgs []string
	for _, pkg := range strings.Fields(listed) {
		if strings.HasPrefix(pkg, e2ePrefix) || strings.HasPrefix(pkg, buildPrefix) {
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}
