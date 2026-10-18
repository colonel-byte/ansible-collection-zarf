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
	"strings"
	"testing"
)

// twoComponents is a zarf.yaml shaped like the example packages: comments above a component, a
// blank line between components, and keys after the images list.
const twoComponents = `---
kind: ZarfPackageConfig
metadata:
  name: app-fluxcd
  version: v2.9.5
components:
  # A comment above the component, which has to survive.
  - name: fluxcd-core
    required: true
    description: The controllers
    manifests:
      - name: fluxcd
        kustomizations:
          - upstream/core
    images:
      - ghcr.io/fluxcd/source-controller:v1.0.0

  - name: fluxcd-source-watcher
    required: false
    images:
      - ghcr.io/fluxcd/source-watcher:v2.0.0
`

// TestReplaceImagesRewritesOneComponent is the test the image surgery exists for, and the one that
// would have caught what it got wrong the first time.
//
// The surgery is a regular expression over a hand-written file rather than a YAML round trip,
// because a round trip would reformat the file and drop every comment in it. The risk of that
// choice is a pattern that reaches past the component it was given, and both of these cases are
// that risk: the first replaces one component's list and has to leave the other's alone, and the
// second is the blank-line regression -- a list group that also matched blank lines swallowed the
// separator between components, so the first run reflowed the file and every run after it was a
// no-op that made the damage invisible.
func TestReplaceImagesRewritesOneComponent(t *testing.T) {
	for _, tt := range []struct {
		name      string
		component string
		images    []string
		want      []string
		wantKept  []string
	}{
		{
			name:      "the first component, leaving the second and the comment alone",
			component: "fluxcd-core",
			images: []string{
				"ghcr.io/fluxcd/helm-controller:v1.6.4",
				"ghcr.io/fluxcd/source-controller:v1.9.5",
			},
			want: []string{
				"      - ghcr.io/fluxcd/helm-controller:v1.6.4\n",
				"      - ghcr.io/fluxcd/source-controller:v1.9.5\n",
			},
			wantKept: []string{
				"  # A comment above the component, which has to survive.\n",
				"      - ghcr.io/fluxcd/source-watcher:v2.0.0\n",
				"\n  - name: fluxcd-source-watcher\n",
			},
		},
		{
			name:      "the last component, keeping the blank line that precedes it",
			component: "fluxcd-source-watcher",
			images: []string{
				"ghcr.io/fluxcd/source-watcher:v2.2.4",
			},
			want: []string{
				"      - ghcr.io/fluxcd/source-watcher:v2.2.4\n",
			},
			wantKept: []string{
				"\n  - name: fluxcd-source-watcher\n",
				"      - ghcr.io/fluxcd/source-controller:v1.0.0\n",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := replaceImages(twoComponents, tt.component, tt.images)
			if err != nil {
				t.Fatalf("replaceImages: %v", err)
			}

			for _, line := range tt.want {
				if !strings.Contains(got, line) {
					t.Errorf("the result does not hold %q", line)
				}
			}
			for _, line := range tt.wantKept {
				if !strings.Contains(got, line) {
					t.Errorf("the result lost %q, which it was not asked to change", line)
				}
			}

			// Replacing the same list with the same contents twice has to be a no-op. A pattern
			// that consumes one character more than it writes back is stable on the second run and
			// has already done its damage on the first, which is why idempotence is asserted
			// rather than assumed.
			again, err := replaceImages(got, tt.component, tt.images)
			if err != nil {
				t.Fatalf("the second replaceImages: %v", err)
			}
			if again != got {
				t.Error("replacing the same images twice changed the file the second time")
			}
		})
	}
}

// TestReplaceImagesRefusesWhatItCannotFind holds the two failure cases, because both of them would
// otherwise be a silent no-op: a component that was renamed, and one with no images key to write
// into.
func TestReplaceImagesRefusesWhatItCannotFind(t *testing.T) {
	for _, tt := range []struct {
		name      string
		source    string
		component string
	}{
		{
			name:      "no component of that name",
			source:    twoComponents,
			component: "fluxcd-image-controller",
		},
		{
			name: "a component with no images key",
			source: `components:
  - name: fluxcd-core
    required: true
`,
			component: "fluxcd-core",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := replaceImages(tt.source, tt.component, []string{"ghcr.io/x:v1"}); err == nil {
				t.Error("replaceImages reported no error, so a component it could not find would " +
					"have been left at whatever images it already had")
			}
		})
	}
}
