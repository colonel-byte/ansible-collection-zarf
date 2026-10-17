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

package pkgvendor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRehomedImagesSkipsWhatAlreadyHasARegistry holds the rule the kustomize image transform is
// built from: a reference's first path element is a registry host only if it holds a dot or a
// colon.
//
// Getting it wrong in either direction is quiet. Re-homing an image that already names a registry
// produces `ghcr.io/ghcr.io/...`, which fails at pull time; failing to re-home one that does not
// leaves the package pulling from a rate-limited Docker Hub, which fails only sometimes.
func TestRehomedImagesSkipsWhatAlreadyHasARegistry(t *testing.T) {
	for _, tt := range []struct {
		name     string
		manifest string
		registry string
		want     []string
	}{
		{
			name: "bare names are re-homed",
			manifest: "spec:\n  containers:\n    - image: fluxcd/source-controller:v1.9.5\n" +
				"    - image: fluxcd/helm-controller:v1.6.4\n",
			registry: "ghcr.io",
			want:     []string{"fluxcd/helm-controller", "fluxcd/source-controller"},
		},
		{
			name:     "a name with a dotted host is left alone",
			manifest: "    - image: ghcr.io/fluxcd/source-controller:v1.9.5\n",
			registry: "ghcr.io",
			want:     []string{},
		},
		{
			name:     "a name with a port is left alone",
			manifest: "    - image: localhost:5000/fluxcd/source-controller:v1.9.5\n",
			registry: "ghcr.io",
			want:     []string{},
		},
		{
			name:     "a single-element name has no repository to re-home",
			manifest: "    - image: busybox:1.36\n",
			registry: "ghcr.io",
			want:     []string{},
		},
		{
			name:     "no registry declared means no transform at all",
			manifest: "    - image: fluxcd/source-controller:v1.9.5\n",
			registry: "",
			want:     []string{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "deployment.yaml"), []byte(tt.manifest), 0o600); err != nil {
				t.Fatal(err)
			}

			got, err := rehomedImages(directory, Project{Match: []string{"deployment.yaml"}}, tt.registry)
			if err != nil {
				t.Fatalf("rehomedImages: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("re-homed %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("re-homed %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}

// TestSetPackageVersionWritesThePin holds the one line Vendor rewrites in a hand-written zarf.yaml,
// and that it writes no other.
//
// The pattern is anchored on two-space indentation so that it cannot match a chart's version inside
// a component. A pattern that matched both would rewrite a chart version to a flux release tag, and
// the package would still build.
func TestSetPackageVersionWritesThePin(t *testing.T) {
	const source = `kind: ZarfPackageConfig
metadata:
  name: app-fluxcd
  version: v2.9.4
components:
  - name: a-chart
    charts:
      - name: something
        version: 1.2.3
`

	path := filepath.Join(t.TempDir(), "zarf.yaml")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := setPackageVersion(path, "v2.9.5"); err != nil {
		t.Fatalf("setPackageVersion: %v", err)
	}

	written, err := os.ReadFile(path) //nolint:gosec // a path in this test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}

	const want = `kind: ZarfPackageConfig
metadata:
  name: app-fluxcd
  version: v2.9.5
components:
  - name: a-chart
    charts:
      - name: something
        version: 1.2.3
`
	if string(written) != want {
		t.Errorf("setPackageVersion wrote:\n%s\nwant:\n%s", written, want)
	}
}
