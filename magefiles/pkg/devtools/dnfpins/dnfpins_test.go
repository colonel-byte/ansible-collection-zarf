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

package dnfpins_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/devtools/dnfpins"
)

func TestRpmVersionCompare(t *testing.T) {
	cases := []struct {
		name string
		a    dnfpins.RpmVersion
		b    dnfpins.RpmVersion
		want int
	}{
		{
			name: "release suffix orders after the shorter release",
			a: dnfpins.RpmVersion{
				Epoch: "1",
				Ver:   "2.16.16",
				Rel:   "2.el10",
			},
			b: dnfpins.RpmVersion{
				Epoch: "1",
				Ver:   "2.16.16",
				Rel:   "2.el10_2.1",
			},
			want: -1,
		},
		{
			name: "epoch outranks a higher version",
			a: dnfpins.RpmVersion{
				Epoch: "1",
				Ver:   "2.16.16",
				Rel:   "2.el10_2.1",
			},
			b: dnfpins.RpmVersion{
				Epoch: "2",
				Ver:   "1.0",
				Rel:   "1",
			},
			want: -1,
		},
		{
			name: "an absent epoch is epoch 0",
			a: dnfpins.RpmVersion{
				Epoch: "",
				Ver:   "2.11",
				Rel:   "16.el10",
			},
			b: dnfpins.RpmVersion{
				Epoch: "0",
				Ver:   "2.11",
				Rel:   "16.el10",
			},
			want: 0,
		},
		{
			name: "a numerically higher patch wins over a longer string",
			a: dnfpins.RpmVersion{
				Epoch: "1",
				Ver:   "2.16.9",
				Rel:   "1.el10",
			},
			b: dnfpins.RpmVersion{
				Epoch: "1",
				Ver:   "2.16.10",
				Rel:   "1.el10",
			},
			want: -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Compare(tc.b); got != tc.want {
				t.Errorf("Compare(a, b) = %d, want %d", got, tc.want)
			}
			if got := tc.b.Compare(tc.a); got != -tc.want {
				t.Errorf("Compare(b, a) = %d, want %d", got, -tc.want)
			}
		})
	}
}

func TestFullVersion(t *testing.T) {
	v := dnfpins.RpmVersion{
		Epoch: "1",
		Ver:   "2.16.16",
		Rel:   "2.el10_2.1",
	}
	if got, want := v.FullVersion(), "2.16.16-2.el10_2.1"; got != want {
		t.Errorf("FullVersion() = %q, want %q", got, want)
	}
}

func TestUpdateDockerfileAnsiblePins(t *testing.T) {
	dfPath := filepath.Join(t.TempDir(), "Dockerfile")

	initial := `FROM ghcr.io/almalinux/10-base:10.2
ARG TARGETPLATFORM
ARG ANSIBLE_CORE_VERSION="old"
ARG BASH_COMPLETION_VERSION="old"

RUN dnf install -y ansible-core-${ANSIBLE_CORE_VERSION}
`
	if err := os.WriteFile(dfPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := dnfpins.UpdateDockerfileAnsiblePins(dfPath, "2.16.16-2.el10_2.1", "2.11-16.el10"); err != nil {
		t.Fatalf("UpdateDockerfileAnsiblePins failed: %v", err)
	}

	data, err := os.ReadFile(dfPath)
	if err != nil {
		t.Fatal(err)
	}

	expected := `FROM ghcr.io/almalinux/10-base:10.2
ARG TARGETPLATFORM
ARG ANSIBLE_CORE_VERSION="2.16.16-2.el10_2.1"
ARG BASH_COMPLETION_VERSION="2.11-16.el10"

RUN dnf install -y ansible-core-${ANSIBLE_CORE_VERSION}
`
	if string(data) != expected {
		t.Errorf("content mismatch:\ngot:\n%s\nwant:\n%s", string(data), expected)
	}
}

func TestUpdateDockerfileAnsiblePinsReportsAMissingArg(t *testing.T) {
	dfPath := filepath.Join(t.TempDir(), "Dockerfile")

	if err := os.WriteFile(dfPath, []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := dnfpins.UpdateDockerfileAnsiblePins(dfPath, "1-1", "1-1"); err == nil {
		t.Error("expected an error naming the ARG that is not there")
	}
}

func TestUpdateGoreleaserDnfPins(t *testing.T) {
	grPath := filepath.Join(t.TempDir(), ".goreleaser.yaml")

	// The real block carries a dockerfile, images, tags, platforms and flags between the id and
	// build_args, which is what the rewriter has to span.
	initial := `dockers_v2:
  - id: zarf-ansible
    dockerfile: containers/ansible/Dockerfile
    platforms:
      - linux/amd64
    build_args:
      ANSIBLE_CORE_VERSION: old
      BASH_COMPLETION_VERSION: old
    labels:
      "org.opencontainers.image.version": |-
        {{ .Version }}
`
	if err := os.WriteFile(grPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	pins := &dnfpins.AlmaLinuxPins{
		AnsibleCore:    "2.16.16-2.el10_2.1",
		BashCompletion: "2.11-16.el10",
	}

	if err := dnfpins.UpdateGoreleaserDnfPins(grPath, pins); err != nil {
		t.Fatalf("UpdateGoreleaserDnfPins failed: %v", err)
	}

	data, err := os.ReadFile(grPath)
	if err != nil {
		t.Fatal(err)
	}

	expected := `dockers_v2:
  - id: zarf-ansible
    dockerfile: containers/ansible/Dockerfile
    platforms:
      - linux/amd64
    build_args:
      ANSIBLE_CORE_VERSION: 2.16.16-2.el10_2.1
      BASH_COMPLETION_VERSION: 2.11-16.el10
    labels:
      "org.opencontainers.image.version": |-
        {{ .Version }}
`
	if string(data) != expected {
		t.Errorf("content mismatch:\ngot:\n%s\nwant:\n%s", string(data), expected)
	}
}

// TestUpdatersMatchTheRepositoryFiles runs each updater against a copy of the file it rewrites in
// this repository, and fails when one no longer finds what it edits.
//
// The other tests use literal fixtures, so none of them would notice the image dropping a package
// or the dockers_v2 id being renamed: the pin would stay in the updater, nothing would reference
// it, and generate:dnfPins would fail the first time anybody refreshed the pins. Copies are edited
// rather than the files themselves -- the updaters write in place.
func TestUpdatersMatchTheRepositoryFiles(t *testing.T) {
	// Four levels up from magefiles/pkg/devtools/dnfpins.
	const repoRoot = "../../../.."

	copyIntoTemp := func(t *testing.T, path string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, path))
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		dest := filepath.Join(t.TempDir(), filepath.Base(path))
		if err := os.WriteFile(dest, content, 0o600); err != nil {
			t.Fatal(err)
		}
		return dest
	}

	pins := &dnfpins.AlmaLinuxPins{
		AnsibleCore:    "9.9.9-9.el10",
		BashCompletion: "9.9.9-9.el10",
	}

	t.Run("ansible Dockerfile", func(t *testing.T) {
		path := copyIntoTemp(t, dnfpins.AnsibleDockerfilePath)
		if err := dnfpins.UpdateDockerfileAnsiblePins(path, pins.AnsibleCore, pins.BashCompletion); err != nil {
			t.Error(err)
		}
	})

	t.Run("goreleaser config", func(t *testing.T) {
		path := copyIntoTemp(t, dnfpins.GoreleaserConfigPath)
		if err := dnfpins.UpdateGoreleaserDnfPins(path, pins); err != nil {
			t.Error(err)
		}
	})
}

// TestFindLatestRPMPicksByVersionNotByOrder holds the one piece of judgement in the pin lookup.
//
// Repodata lists a package's builds in whatever order the repository wrote them, which is not
// version order: AlmaLinux ships several builds of ansible-core at once and the newest is
// routinely not the last. A lookup that took the first or the last match would pin a real version
// that is simply not the newest one, and the pin would look correct in every file it lands in --
// the containers would build, the image would work, and it would quietly stay a release behind.
//
// The comparison itself is dnfpins.RpmVersion.Compare, which TestRpmVersionCompare covers; what is held
// here is that the search uses it over every candidate and ignores other packages entirely.
func TestFindLatestRPMPicksByVersionNotByOrder(t *testing.T) {
	primary := &dnfpins.PrimaryXML{
		Packages: []dnfpins.RpmPackage{
			{
				Name:    "ansible-core",
				Version: dnfpins.RpmVersion{Epoch: "0", Ver: "2.16.3", Rel: "1.el9"},
			},
			{
				Name:    "bash-completion",
				Version: dnfpins.RpmVersion{Epoch: "0", Ver: "2.11", Rel: "5.el9"},
			},
			// The newest, in the middle, so neither "first match" nor "last match" finds it.
			{
				Name:    "ansible-core",
				Version: dnfpins.RpmVersion{Epoch: "0", Ver: "2.16.14", Rel: "1.el9"},
			},
			{
				Name:    "ansible-core",
				Version: dnfpins.RpmVersion{Epoch: "0", Ver: "2.16.9", Rel: "2.el9"},
			},
		},
	}

	t.Run("the newest build wins wherever it sits in the list", func(t *testing.T) {
		got, err := dnfpins.FindLatestRPM(primary, "ansible-core")
		if err != nil {
			t.Fatalf("dnfpins.FindLatestRPM reported %v", err)
		}
		// 2.16.14 over 2.16.9 and 2.16.3: a string comparison would pick 2.16.9.
		if want := "2.16.14"; got.Version.Ver != want {
			t.Errorf("the newest ansible-core was %s, want %s", got.Version.Ver, want)
		}
	})

	t.Run("a package with one build is still found", func(t *testing.T) {
		got, err := dnfpins.FindLatestRPM(primary, "bash-completion")
		if err != nil {
			t.Fatalf("dnfpins.FindLatestRPM reported %v", err)
		}
		if want := "2.11"; got.Version.Ver != want {
			t.Errorf("bash-completion was %s, want %s", got.Version.Ver, want)
		}
	})

	t.Run("a package the repodata does not carry is an error naming it", func(t *testing.T) {
		_, err := dnfpins.FindLatestRPM(primary, "a-package-nobody-ships")
		if err == nil {
			t.Fatal("a missing package was reported as found")
		}
		if !strings.Contains(err.Error(), "a-package-nobody-ships") {
			t.Errorf("the error was %q, want one naming the package", err)
		}
	})
}
