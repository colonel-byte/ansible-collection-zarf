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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// TestVendorSourceURLs holds the two places a pinned release's files are read from.
//
// Both forms are a pin to a tag and differ only in where the bytes live, which is the whole reason
// the declaration carries one field or the other rather than a flavour of fetcher. Flux attaches
// manifests.tar.gz to each release; Argo CD attaches only CLI binaries and SBOMs, and its release
// notes point at raw.githubusercontent.com/argoproj/argo-cd/<tag>/manifests/install.yaml.
//
// The repo URL is normalised rather than taken literally, because the two hosts spell a repository
// differently: `repo` is the browsable https://github.com/owner/name that belongs in a
// resources.json a human reads, and the raw host wants owner/name alone. A URL written with a
// trailing slash or a .git suffix -- both of which clone fine and both of which somebody will
// paste in -- would otherwise produce a 404 naming a path with an empty or `.git`-suffixed segment
// in the middle of it.
func TestVendorSourceURLs(t *testing.T) {
	for _, tt := range []struct {
		name      string
		resources Resources
		match     string
		want      string
	}{
		{
			name: "a release asset",
			resources: Resources{
				Repo:  "https://github.com/fluxcd/flux2",
				Tag:   "v2.9.5",
				Asset: "manifests.tar.gz",
			},
			want: "https://github.com/fluxcd/flux2/releases/download/v2.9.5/manifests.tar.gz",
		},
		{
			name: "a file in the tagged tree",
			resources: Resources{
				Repo: "https://github.com/argoproj/argo-cd",
				Tag:  "v3.5.5",
				Tree: "manifests",
			},
			match: "namespace-install.yaml",
			want:  "https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.5/manifests/namespace-install.yaml",
		},
		{
			name: "a match that names a subdirectory of the tree",
			resources: Resources{
				Repo: "https://github.com/argoproj/argo-cd",
				Tag:  "v3.5.5",
				Tree: "manifests",
			},
			match: "crds/applicationset-crd.yaml",
			want:  "https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.5/manifests/crds/applicationset-crd.yaml",
		},
		{
			name: "a repo url written with a trailing slash and a .git suffix",
			resources: Resources{
				Repo: "https://github.com/argoproj/argo-cd.git/",
				Tag:  "v3.5.5",
				Tree: "/manifests/",
			},
			match: "namespace-install.yaml",
			want:  "https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.5/manifests/namespace-install.yaml",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := assetURL(&tt.resources)
			if tt.resources.Tree != "" {
				got = treeURL(&tt.resources, tt.match)
			}
			if got != tt.want {
				t.Errorf("the url was\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

// TestResourcesValidateRejectsAHalfDeclaration holds the declarations that would vendor something
// other than what they appear to say.
//
// Naming neither source and naming both are the two new ways to get this wrong, and both are worth
// a message rather than a default: a declaration with neither has nowhere to read from, and one
// with both reads from whichever branch happens to be checked first, which is a coin toss decided
// by the order of an if.
func TestResourcesValidateRejectsAHalfDeclaration(t *testing.T) {
	valid := func() Resources {
		return Resources{
			Tag:       "v3.5.5",
			Repo:      "https://github.com/argoproj/argo-cd",
			Tree:      "manifests",
			Namespace: "argocd",
			PartOf:    "argocd",
			Projects: []Project{
				{
					Path:      "core",
					Component: "argocd-core",
					Match:     []string{"namespace-install.yaml"},
				},
			},
		}
	}

	for _, tt := range []struct {
		name    string
		mutate  func(*Resources)
		wantErr string
	}{
		{
			name:    "a complete tree declaration",
			mutate:  func(*Resources) {},
			wantErr: "",
		},
		{
			name: "a complete asset declaration",
			mutate: func(r *Resources) {
				r.Tree = ""
				r.Asset = "manifests.tar.gz"
			},
			wantErr: "",
		},
		{
			name: "neither an asset nor a tree",
			mutate: func(r *Resources) {
				r.Tree = ""
			},
			wantErr: "neither asset nor tree",
		},
		{
			name: "both an asset and a tree",
			mutate: func(r *Resources) {
				r.Asset = "manifests.tar.gz"
			},
			wantErr: "both asset and tree",
		},
		{
			name: "no part-of label to stamp",
			mutate: func(r *Resources) {
				r.PartOf = ""
			},
			wantErr: "partOf is empty",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resources := valid()
			tt.mutate(&resources)

			err := resources.validate("resources.json")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("a valid declaration was rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("the declaration was accepted, want an error naming %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("the error was %q, want one naming %q", err, tt.wantErr)
			}
		})
	}
}

// tarGz builds a gzipped tar from name/content pairs, for the archive tests below.
func tarGz(t *testing.T, entries map[string]string, dirs ...string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)

	for _, dir := range dirs {
		if err := tw.WriteHeader(&tar.Header{
			Name:     dir,
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		}); err != nil {
			t.Fatalf("unable to write the directory entry: %v", err)
		}
	}
	for name, content := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}); err != nil {
			t.Fatalf("unable to write the header for %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("unable to write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("unable to close the tar writer: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("unable to close the gzip writer: %v", err)
	}
	return buf.Bytes()
}

// TestReadArchiveReducesNamesAndRefusesAClash covers the release-asset path, which is the one that
// reads bytes fetched over the network.
//
// Two of its rules are the kind that look like tidying until they do not. Reducing each entry to
// its basename is also what makes a crafted entry name harmless -- nothing joins an archive-supplied
// path onto a directory, so there is no "../" for one to climb with -- and refusing a clash is what
// stops two entries that differ only by directory silently becoming one vendored file, which would
// produce a component deploying something other than what resources.json names.
func TestReadArchiveReducesNamesAndRefusesAClash(t *testing.T) {
	t.Run("entries are keyed by basename and directories are skipped", func(t *testing.T) {
		files, err := readArchive(tarGz(t, map[string]string{
			"source-controller.yaml":         "kind: Deployment\n",
			"./policies.yaml":                "kind: NetworkPolicy\n",
			"manifests/helm-controller.yaml": "kind: Deployment\n",
		}, "manifests/"))
		if err != nil {
			t.Fatalf("readArchive reported %v", err)
		}

		if got, want := strings.Join(sortedNames(files), ","),
			"helm-controller.yaml,policies.yaml,source-controller.yaml"; got != want {
			t.Errorf("the archive read as %q, want %q", got, want)
		}
		if got := string(files["policies.yaml"]); got != "kind: NetworkPolicy\n" {
			t.Errorf("policies.yaml read as %q", got)
		}
	})

	t.Run("a traversal in an entry name cannot escape, because only the basename is kept", func(t *testing.T) {
		files, err := readArchive(tarGz(t, map[string]string{
			"../../../etc/cron.d/payload": "root\n",
		}))
		if err != nil {
			t.Fatalf("readArchive reported %v", err)
		}
		if _, ok := files["payload"]; !ok {
			t.Fatalf("the entry was not reduced to its basename: %v", sortedNames(files))
		}
		for name := range files {
			if strings.Contains(name, "/") || strings.Contains(name, "..") {
				t.Errorf("an entry kept a path component: %q", name)
			}
		}
	})

	t.Run("two entries with the same basename are refused rather than merged", func(t *testing.T) {
		_, err := readArchive(tarGz(t, map[string]string{
			"core/rbac.yaml":   "kind: ClusterRole\n",
			"images/rbac.yaml": "kind: Role\n",
		}))
		if err == nil {
			t.Fatal("a clashing archive was accepted, so one of the two files was lost")
		}
		if !strings.Contains(err.Error(), "rbac.yaml") {
			t.Errorf("the error was %q, want one naming the clashing file", err)
		}
	})

	t.Run("an asset that is not gzip says so", func(t *testing.T) {
		_, err := readArchive([]byte("<!DOCTYPE html>\n<html>404</html>\n"))
		if err == nil {
			t.Fatal("a non-gzip asset was accepted")
		}
		// The realistic failure is a 404 page served with a 200, which is bytes that are not an
		// archive; a message about gzip is what points at that.
		if !strings.Contains(err.Error(), "gzip") {
			t.Errorf("the error was %q, want one naming gzip", err)
		}
	})
}
