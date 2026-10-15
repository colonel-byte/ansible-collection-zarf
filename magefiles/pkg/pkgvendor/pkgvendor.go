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

// Package pkgvendor vendors the upstream manifests an example Zarf package deploys, from a pinned
// release, and keeps the package's generated parts in step with the pin.
//
// It is a Go port of the three shell scripts in colonel-byte/zarf-pkg-fluxcd's `.github/hack`:
// `resources.json` declares what to fetch and at which tag, `update.sh` downloads the release asset
// and extracts the manifests into the package tree, and `zarf.sh` runs `zarf dev find-images` and
// writes the discovered images back into `zarf.yaml`. The shape of resources.json is deliberately
// close to theirs, so that a reader who knows one knows the other.
//
// Three things are different, and each is a simplification this repository can afford because what
// lives here is a reference package rather than a published one:
//
//   - One flavor. zarf-pkg-fluxcd builds `upstream` and `minimus`; this vendors upstream only,
//     which is what the manifests the example teaches from are.
//   - One pin. There, the release tag appears in resources.json once per project and again as the
//     package version in zarf.yaml, each with its own Renovate annotation. Here it appears once, at
//     the top of resources.json, and Vendor writes it into zarf.yaml's metadata.version and into
//     the generated kustomizations. Three copies of a version that must agree is three chances for
//     them not to.
//   - The kustomizations are generated. Upstream's manifests carry no namespace and no
//     `app.kubernetes.io/version` label, so the kustomization has to add both -- and since the
//     version it labels with is the pin, generating the file is what keeps the cluster able to
//     report which Flux release it is running. The prerequisites role reads exactly that label.
//
// Nothing here runs in CI or in a test: it reaches the network for a release asset, and
// `examples:images` needs a zarf on PATH. It is a maintenance command, run when the pin moves.
package pkgvendor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Root is where the example packages live, each one a directory holding a resources.json.
var Root = filepath.Join("examples", "packages")

// ResourcesFile is the declaration of what to vendor, inside a package's directory.
const ResourcesFile = "resources.json"

// VendoredDir is the directory under a package that the vendored manifests are written into, one
// subdirectory per project. It is named for the flavor the manifests are, the way zarf-pkg-fluxcd
// names its trees, so that adding a second flavor later is a directory rather than a redesign.
const VendoredDir = "upstream"

// fetchTimeout bounds the release download. A manifest archive is tens of kilobytes; a minute is
// long enough for a slow link and short enough that a hung proxy fails rather than hangs.
const fetchTimeout = time.Minute

// maxAssetBytes bounds what is read out of the release asset and out of the archive inside it.
// Nothing legitimate here is close to it, and a decompression bomb is the one hazard of extracting
// an archive fetched over the network.
const maxAssetBytes = 64 << 20

// Resources is a package's resources.json: one upstream release, and the projects extracted from it.
type Resources struct {
	// Schema is the generated JSON Schema this file validates against, as a relative path. It is
	// read and rewritten unchanged; it exists for the editor, not for this package.
	Schema string `json:"$schema,omitempty"`
	// Tag is the upstream release tag to vendor. It is the pin, and the only copy of it: Vendor
	// writes it into the package's zarf.yaml and into every generated kustomization.
	Tag string `json:"tag" jsonschema:"example=v2.9.5"`
	// Renovate is the annotation a Renovate configuration reads to offer a bump of Tag. It is
	// carried from zarf-pkg-fluxcd, where Renovate is wired up; nothing in this repository reads
	// it, and `examples:refresh` is what moves the pin here.
	Renovate string `json:"renovate,omitempty" jsonschema:"example=datasource=github-releases depName=fluxcd/flux2"`
	// Repo is the upstream repository's URL.
	Repo string `json:"repo" jsonschema:"example=https://github.com/fluxcd/flux2"`
	// Asset is the release asset to download, by name.
	Asset string `json:"asset" jsonschema:"example=manifests.tar.gz"`
	// Namespace is the namespace every generated kustomization places its resources in.
	Namespace string `json:"namespace" jsonschema:"example=flux-system"`
	// Registry re-homes every image the vendored manifests name without one onto this host, as a
	// kustomize image transform in the generated kustomization.
	//
	// Upstream's manifests name their images as `fluxcd/source-controller:v1.9.5`, which resolves
	// to Docker Hub -- where an anonymous pull is rate-limited per IP, and a zarf package create
	// pulls every image in the package at once. The same images are published to ghcr.io, so this
	// is the difference between a create that works and a create that fails partway through on a
	// shared outbound address. Left empty, the manifests are vendored as they came.
	Registry string `json:"registry,omitempty" jsonschema:"example=ghcr.io"`
	// Projects are the subdirectories the asset's contents are split into.
	Projects []Project `json:"projects"`
}

// Project is one subdirectory of vendored manifests, and the zarf component that deploys it.
type Project struct {
	// Path is the subdirectory under the package's upstream/ tree.
	Path string `json:"path" jsonschema:"example=core"`
	// Component is the zarf component in the package's zarf.yaml whose images are refreshed from
	// these manifests. It is named here rather than inferred, because the mapping is the one thing
	// a reader of either file cannot work out from the other.
	Component string `json:"component" jsonschema:"example=fluxcd-core"`
	// Match are the filenames to take out of the release asset, which is also the resource list of
	// the generated kustomization and so the order the manifests are applied in.
	//
	// They are exact names rather than globs. The upstream archive is a flat handful of files whose
	// names are part of its interface, and a glob that silently stops matching -- because upstream
	// renamed one -- produces a component that deploys less than it used to and fails nothing.
	// A name that is not in the archive is an error.
	Match []string `json:"match"`
}

// Load reads one package's resources.json.
func Load(pkg string) (*Resources, error) {
	path := filepath.Join(Root, pkg, ResourcesFile)

	source, err := os.ReadFile(path) //nolint:gosec // a path under Root, from the caller's package name
	if err != nil {
		return nil, err
	}

	var resources Resources
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&resources); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &resources, resources.validate(path)
}

// validate rejects a declaration that would produce a half-vendored tree.
func (r *Resources) validate(path string) error {
	switch {
	case r.Tag == "":
		return fmt.Errorf("%s: tag is empty, so there is no pinned release to vendor", path)
	case r.Repo == "":
		return fmt.Errorf("%s: repo is empty", path)
	case r.Asset == "":
		return fmt.Errorf("%s: asset is empty", path)
	case len(r.Projects) == 0:
		return fmt.Errorf("%s: no projects, so vendoring would write nothing", path)
	}

	for _, project := range r.Projects {
		if project.Path == "" || project.Component == "" {
			return fmt.Errorf("%s: a project is missing its path or component", path)
		}
		if len(project.Match) == 0 {
			return fmt.Errorf("%s: project %s matches no files", path, project.Path)
		}
	}
	return nil
}

// Packages returns the example packages that declare a resources.json, in directory order.
//
// A package without one is not an error: examples/packages/operator-external-secrets ships its own manifests
// and vendors nothing, and it is a reference implementation all the same.
func Packages() ([]string, error) {
	entries, err := os.ReadDir(Root)
	if err != nil {
		return nil, err
	}

	var packages []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(Root, entry.Name(), ResourcesFile)); err == nil {
			packages = append(packages, entry.Name())
		}
	}
	return packages, nil
}

// Vendor downloads the pinned release asset for one package and rewrites everything that follows
// from it: the extracted manifests, a kustomization.yaml per project, and the package version in
// zarf.yaml.
//
// The asset is downloaded once and each project is extracted from the same copy in memory, rather
// than fetched per project the way update.sh does it. Three downloads of one file was a consequence
// of the loop being in shell, not a decision.
func Vendor(pkg string) error {
	resources, err := Load(pkg)
	if err != nil {
		return err
	}

	archive, err := download(assetURL(resources))
	if err != nil {
		return err
	}

	files, err := readArchive(archive)
	if err != nil {
		return err
	}

	for _, project := range resources.Projects {
		directory := filepath.Join(Root, pkg, VendoredDir, project.Path)

		// The tree is removed rather than written over, so that a file dropped from Match or
		// renamed upstream leaves with it. update.sh does the same, for the same reason: a stale
		// manifest in a kustomization's directory is invisible until something applies it.
		if err := os.RemoveAll(directory); err != nil {
			return err
		}
		if err := os.MkdirAll(directory, 0o775); err != nil {
			return err
		}

		for _, name := range project.Match {
			content, ok := files[name]
			if !ok {
				return fmt.Errorf("%s names %s, which release %s of %s does not carry. Its files are: %s",
					ResourcesFile, name, resources.Tag, resources.Repo, strings.Join(sortedNames(files), ", "))
			}
			path := filepath.Join(directory, name)
			if err := os.WriteFile(path, content, 0o644); err != nil { //nolint:gosec // vendored manifests are world-readable
				return err
			}
			fmt.Println(path)
		}

		if err := writeKustomization(directory, resources, project); err != nil {
			return err
		}
	}

	return setPackageVersion(filepath.Join(Root, pkg, "zarf.yaml"), resources.Tag)
}

// assetURL is where a GitHub release asset is downloaded from.
//
// It is built rather than fetched through the API, and the download needs no token: a public
// release asset is a plain URL, and `gh` -- which update.sh needs -- is one more thing to have
// installed and authenticated on a machine that is only refreshing a pin.
func assetURL(resources *Resources) string {
	return fmt.Sprintf("%s/releases/download/%s/%s",
		strings.TrimSuffix(resources.Repo, "/"), resources.Tag, resources.Asset)
}

// download fetches a URL into memory, within fetchTimeout and maxAssetBytes.
func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: fetchTimeout}

	response, err := client.Get(url) //nolint:gosec,noctx // a URL built from the pinned declaration
	if err != nil {
		return nil, fmt.Errorf("unable to fetch %s: %w", url, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s. Check that the pinned tag and asset name exist",
			url, response.Status)
	}

	fmt.Printf("fetched %s\n", url)
	return io.ReadAll(io.LimitReader(response.Body, maxAssetBytes))
}

// readArchive returns the regular files in a .tar.gz, keyed by base name.
//
// Keyed by base name rather than by path because that is how the names in Match read, and because
// the upstream archive is flat. A nested archive would collapse two same-named files into one, so
// that is an error rather than a silent last-one-wins.
func readArchive(archive []byte) (map[string][]byte, error) {
	gzipped, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("the release asset is not gzip: %w", err)
	}
	defer func() { _ = gzipped.Close() }()

	files := map[string][]byte{}
	reader := tar.NewReader(io.LimitReader(gzipped, maxAssetBytes))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// The name is reduced to its base before it is used, which is also what makes a crafted
		// entry name harmless: nothing here joins an archive-supplied path onto a directory, so
		// there is no "../" for one to climb with.
		name := filepath.Base(filepath.Clean(header.Name))
		if _, clash := files[name]; clash {
			return nil, fmt.Errorf("the release asset holds two files named %s", name)
		}

		content, err := io.ReadAll(io.LimitReader(reader, maxAssetBytes))
		if err != nil {
			return nil, err
		}
		files[name] = content
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("the release asset holds no files")
	}
	return files, nil
}

// sortedNames returns the keys of a file map, for a message that names what was available.
func sortedNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	// A message listing what the archive holds is read by a person, so the order has to be stable.
	sort.Strings(names)
	return names
}

// packageVersion matches the version line in a zarf.yaml's metadata, which is the one line Vendor
// rewrites in a file that is otherwise hand-written.
//
// It is anchored on two-space indentation so that it cannot match a version inside a component, a
// chart or a comment. The replacement keeps whatever follows on the line -- a trailing comment is
// the author's, not this generator's.
var packageVersion = regexp.MustCompile(`(?m)^(  version: )(\S+)(.*)$`)

// setPackageVersion rewrites a package's metadata.version to the pinned tag.
//
// The version is the tag rather than a version of its own because the package is the upstream
// release, repackaged: a package that said 1.0.0 while deploying Flux v2.9.5 would be the one
// number nobody could act on.
func setPackageVersion(path, tag string) error {
	source, err := os.ReadFile(path) //nolint:gosec // a path under Root
	if err != nil {
		return err
	}

	matches := packageVersion.FindAllSubmatchIndex(source, -1)
	if len(matches) != 1 {
		return fmt.Errorf("%s has %d metadata version lines and this expects one, so the pinned "+
			"tag %s was not written into it", path, len(matches), tag)
	}

	updated := packageVersion.ReplaceAll(source, []byte("${1}"+tag+"${3}"))
	if string(updated) == string(source) {
		return nil
	}

	fmt.Printf("%s: version %s\n", path, tag)
	return os.WriteFile(path, updated, 0o644) //nolint:gosec // an example package is world-readable
}
