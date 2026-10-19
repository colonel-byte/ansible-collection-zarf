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

package testrunner

import (
	"strings"
	"testing"
)

// TestUnitPackagesSkipsTheTreesItMustNot holds which packages `test:unit` runs, and both omissions
// matter for a different reason.
//
// The e2e suites are excluded because they need Ansible, k3d and Docker and have runners of their
// own; running them here would mean every `test:unit` either needs a cluster or skips most of what
// it claims to have run.
//
// build/ is excluded because the e2e targets point TMPDIR at build/tmp, so a run that is in flight
// -- or one that was interrupted -- leaves a go-build tree of generated cgo there. It is not
// gitignored out of `go list`, which walks the filesystem rather than the index, and the package
// it finds fails to load with "use of internal package internal/runtime/sys not allowed". That
// names the Go toolchain's own source and reads as a broken checkout rather than as leftovers from
// the suite that is still running in the next terminal.
func TestUnitPackagesSkipsTheTreesItMustNot(t *testing.T) {
	listed := strings.Join([]string{
		modulePath + "/internal/zarfmod",
		modulePath + "/cmd/init",
		// The leftovers. The go-build tree is what fails to load; the zarf staging directories
		// beside it are ordinary temporaries that happen to share the parent.
		modulePath + "/build/tmp/go-build1829435702/b170",
		modulePath + "/build/tmp/zarf-1019171547",
		modulePath + "/test/e2e/cluster",
		modulePath + "/test/e2e/noncluster",
		modulePath + "/magefiles/pkg/pkgvendor",
	}, "\n")

	got := strings.Join(unitPackages(listed), ",")
	want := strings.Join([]string{
		modulePath + "/internal/zarfmod",
		modulePath + "/cmd/init",
		modulePath + "/magefiles/pkg/pkgvendor",
	}, ",")

	if got != want {
		t.Errorf("the packages to test were\n  %s\nwant\n  %s", got, want)
	}
}

// TestUnitPackagesKeepsPackagesMerelyNamedLikeTheExclusions guards the prefixes against being
// loosened into substring matches.
//
// Both exclusions are anchored on the module path for a reason .golangci.yaml gives about the same
// directory: `pkg/utils/build` is real source, and a `build` that matched anywhere in a package
// path would silently stop testing it.
func TestUnitPackagesKeepsPackagesMerelyNamedLikeTheExclusions(t *testing.T) {
	for _, pkg := range []string{
		modulePath + "/magefiles/pkg/build",
		modulePath + "/internal/testenv",
		modulePath + "/magefiles/pkg/devtools/buildinfo",
	} {
		t.Run(pkg, func(t *testing.T) {
			if got := unitPackages(pkg); len(got) != 1 {
				t.Errorf("%s was excluded, and it is a package Unit must test", pkg)
			}
		})
	}
}
