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

// Tests for the generated kustomization: the image transform it writes, which is the one
// piece of judgement in a file that is otherwise a rendering of resources.json.

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
