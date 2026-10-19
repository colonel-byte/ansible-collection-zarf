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
	"errors"
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
	// here because Load rejects unknown fields and would otherwise refuse the key; it exists for
	// the editor and the check-jsonschema hook, not for this package, and nothing rewrites it.
	Schema string `json:"$schema,omitempty"`
	// Tag is the upstream release tag to vendor. It is the pin, and the only copy of it: Vendor
	// writes it into the package's zarf.yaml and into every generated kustomization.
	Tag string `json:"tag" jsonschema:"example=v2.9.5"`
	// Renovate is the annotation a Renovate configuration reads to offer a bump of Tag. It is
	// carried from zarf-pkg-fluxcd, where Renovate is wired up; nothing in this repository reads
	// it, and moving the pin here is a hand edit followed by `examples:vendor`.
	Renovate string `json:"renovate,omitempty" jsonschema:"example=datasource=github-releases depName=fluxcd/flux2"`
	// Repo is the upstream repository's URL.
	Repo string `json:"repo" jsonschema:"example=https://github.com/fluxcd/flux2"`
	// Asset is the release asset to download, by name. Exactly one of Asset and Tree is set.
	Asset string `json:"asset,omitempty" jsonschema:"example=manifests.tar.gz"`
	// Tree vendors from a directory in the tagged source tree instead of from a release asset,
	// and is the directory the project Match paths are relative to.
	//
	// It exists because not every upstream attaches its manifests to a release. Flux publishes
	// manifests.tar.gz and this package was written for that; Argo CD attaches only CLI binaries
	// and SBOMs, and its own release notes tell you to apply
	// raw.githubusercontent.com/argoproj/argo-cd/<tag>/manifests/install.yaml -- a path in the
	// tagged tree. Both are a pin to a tag, which is the only property that matters here; the
	// difference is where the bytes are.
	//
	// The repository's source archive is not used instead, which would keep one code path: it is
	// a gzip tar and readArchive flattens entry names to their basename, so a whole repository of
	// thousands of files collapses onto a handful of names and trips the clash check. Fetching the
	// named files is both smaller and exact.
	Tree string `json:"tree,omitempty" jsonschema:"example=manifests"`
	// Namespace is the namespace every generated kustomization places its resources in.
	Namespace string `json:"namespace" jsonschema:"example=flux-system,example=argocd"`
	// PartOf is the app.kubernetes.io/part-of label every generated kustomization stamps on. It
	// is what upstream already sets on most of its own resources, repeated so that every resource
	// in the project carries it including the ones that do not today.
	PartOf string `json:"partOf" jsonschema:"example=flux,example=argocd"`
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
	// Imageless says this project's manifests deploy no containers, so `zarf dev find-images`
	// reporting none for its component is the right answer rather than a misconfiguration.
	//
	// It is declared rather than inferred because the two cases look identical from the outside:
	// a component genuinely carrying only CustomResourceDefinitions, and a component whose name
	// here no longer matches anything in zarf.yaml. Treating an empty result as acceptable for
	// every project would turn a renamed component into a package that quietly stops carrying its
	// images -- which fails on an air-gapped cluster, at deploy time, with nothing to pull from.
	Imageless bool `json:"imageless,omitempty"`
	// Match are the filenames to take out of the release asset, which is also the resource list of
	// the generated kustomization and so the order the manifests are applied in.
	//
	// They are exact names rather than globs. The upstream archive is a flat handful of files whose
	// names are part of its interface, and a glob that silently stops matching -- because upstream
	// renamed one -- produces a component that deploys less than it used to and fails nothing.
	// A name that is not in the archive is an error.
	//
	// On the tree source they are paths relative to Resources.Tree, because a source tree has
	// directories where an archive is flat. The file is still written out under its basename.
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
	case r.Asset == "" && r.Tree == "":
		return fmt.Errorf("%s: neither asset nor tree is set, so there is nowhere to read the "+
			"manifests from: name a release asset with \"asset\", or a directory in the tagged "+
			"source tree with \"tree\"", path)
	case r.Asset != "" && r.Tree != "":
		return fmt.Errorf("%s: both asset and tree are set, and the manifests come from one place "+
			"or the other", path)
	case r.PartOf == "":
		return fmt.Errorf("%s: partOf is empty, and every generated kustomization stamps it", path)
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
func Packages(marker string) ([]string, error) {
	entries, err := os.ReadDir(Root)
	if err != nil {
		return nil, err
	}

	var packages []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(Root, entry.Name(), marker)); err == nil {
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

	files, err := fetchFiles(resources)
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

		for _, match := range project.Match {
			// Keyed and written by basename: an archive is flat already, and a tree-sourced match
			// carries the directory it came from upstream, which has no meaning beside the
			// kustomization that lists it.
			name := filepath.Base(match)
			content, ok := files[name]
			if !ok {
				return fmt.Errorf("%s names %s, which release %s of %s does not carry. Its files are: %s",
					ResourcesFile, match, resources.Tag, resources.Repo, strings.Join(sortedNames(files), ", "))
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

// treeURL is one file in the tagged source tree, on the raw host that serves file contents rather
// than the HTML page github.com answers with.
func treeURL(resources *Resources, name string) string {
	repo := strings.TrimSuffix(strings.TrimPrefix(
		strings.TrimSuffix(resources.Repo, "/"), "https://github.com/"), ".git")
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		repo, resources.Tag, strings.Trim(resources.Tree, "/"), name)
}

// fetchFiles returns the vendored files by the name each is written under, from whichever of the
// two sources the declaration names.
//
// The asset path downloads once and reads every project out of the one copy. The tree path fetches
// each named file, because there is no archive to read them from -- which also means a Match entry
// that upstream has moved fails as a 404 naming the file, rather than as the "release does not
// carry it" message the archive path gives.
func fetchFiles(resources *Resources) (map[string][]byte, error) {
	if resources.Tree == "" {
		archive, err := download(assetURL(resources))
		if err != nil {
			return nil, err
		}
		return readArchive(archive)
	}

	files := map[string][]byte{}
	for _, project := range resources.Projects {
		for _, match := range project.Match {
			// The basename is the key, so that the rest of Vendor writes and kustomizes these the
			// same way it does an archive's entries. Two matches whose paths differ and whose
			// basenames do not would otherwise silently become one file.
			name := filepath.Base(match)
			if _, clash := files[name]; clash {
				return nil, fmt.Errorf("two matched files are named %s", name)
			}
			content, err := download(treeURL(resources, match))
			if err != nil {
				return nil, err
			}
			files[name] = content
		}
	}
	return files, nil
}

// download fetches a URL into memory, within fetchTimeout and maxAssetBytes.
func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: fetchTimeout}

	response, err := client.Get(url) //nolint:gosec,noctx // a URL built from the pinned declaration
	if err != nil {
		return nil, fmt.Errorf("unable to fetch %s: %w", url, err)
	}
	defer response.Body.Close() //nolint:errcheck // body is read to completion, nothing to do with a close error

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
	defer gzipped.Close() //nolint:errcheck // decompressor is read to completion, nothing to do with a close error

	files := map[string][]byte{}
	reader := tar.NewReader(io.LimitReader(gzipped, maxAssetBytes))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
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
