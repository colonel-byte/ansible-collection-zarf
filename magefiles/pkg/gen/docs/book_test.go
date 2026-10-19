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

package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/colonel-byte/ansible-collection-zarf/internal/testenv"
)

// TestMain chdirs into the repository root, because rebaseLink stats the target it is given and
// the paths it works in are spelled relative to the root.
func TestMain(m *testing.M) {
	if _, _, err := testenv.Bootstrap(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// TestRebaseLinkResolvesOnlyWhatTheBookCanHold is the rule AGENTS.md states about README.md and
// nothing but this function enforces: a relative link in a copied page must point at something
// under docs/, because the book is built from docs/ and no other relative path resolves there.
//
// Both directions are quiet when wrong. A link reported fine that does not resolve is a 404 in the
// published book; a link refused that was always an external URL fails the generator over nothing
// and blocks every other page with it.
func TestRebaseLinkResolvesOnlyWhatTheBookCanHold(t *testing.T) {
	// A real file under docs/, so that the os.Stat inside rebaseLink has something to find. Using
	// one the repository already has rather than a fixture keeps this honest about the layout.
	const existing = "guides/zarf-ansible-module.md"
	if _, err := os.Stat(filepath.Join("docs", existing)); err != nil {
		t.Fatalf("the page this test rebases against is gone: %v", err)
	}

	for _, tt := range []struct {
		name      string
		sourceDir string
		pageDir   string
		target    string
		want      string
		wantOK    bool
	}{
		{
			name:      "a page under docs, reached from the repository root",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "docs/" + existing,
			want:      existing,
			wantOK:    true,
		},
		{
			name:      "an absolute url is left exactly as it is",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "https://github.com/zarf-dev/proposals/pull/73",
			want:      "https://github.com/zarf-dev/proposals/pull/73",
			wantOK:    true,
		},
		{
			name:      "a protocol-relative url is not a path here either",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "//example.test/x",
			want:      "//example.test/x",
			wantOK:    true,
		},
		{
			name:      "a mailto is not a path",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "mailto:someone@example.test",
			want:      "mailto:someone@example.test",
			wantOK:    true,
		},
		{
			name:      "a bare fragment resolves against the page either way",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "#the-reference-packages",
			want:      "#the-reference-packages",
			wantOK:    true,
		},
		{
			name:      "a fragment travels with the rebased path",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "docs/" + existing + "#the-reference-packages",
			want:      existing + "#the-reference-packages",
			wantOK:    true,
		},
		{
			name:      "a target outside docs cannot be reached from the book",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "magefiles/AGENTS.md",
			want:      "magefiles/AGENTS.md",
			wantOK:    false,
		},
		{
			name:      "a target that climbs out of the repository",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "../secrets.md",
			want:      "../secrets.md",
			wantOK:    false,
		},
		{
			name:      "a target under docs that does not exist",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "docs/guides/a-page-nobody-wrote.md",
			want:      "docs/guides/a-page-nobody-wrote.md",
			wantOK:    false,
		},
		{
			name:      "a server-rooted path resolves to neither base",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "/docs/" + existing,
			want:      "/docs/" + existing,
			wantOK:    false,
		},
		{
			name:      "an empty target",
			sourceDir: ".",
			pageDir:   "docs",
			target:    "",
			want:      "",
			wantOK:    false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := rebaseLink(tt.sourceDir, tt.pageDir, tt.target)
			if ok != tt.wantOK {
				t.Errorf("rebaseLink(%q) reported ok=%v, want %v", tt.target, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("rebaseLink(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}
