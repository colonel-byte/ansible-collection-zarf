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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	goyaml "github.com/goccy/go-yaml"
)

// EnvBinary names the zarf to run, for a machine where it is not on PATH or where two versions are
// installed side by side. It is the same variable magefiles/pkg/gen/zarfflags reads.
const EnvBinary = "ZARF_ANSIBLE_FLAGS_BINARY"

// findImagesOutput is the part of `zarf dev find-images` output this reads: the components it
// found images for, and the images under each.
type findImagesOutput struct {
	Components []struct {
		Name   string   `yaml:"name"`
		Images []string `yaml:"images"`
	} `yaml:"components"`
}

// Images runs `zarf dev find-images` over one example package and writes what it found into the
// `images:` list of each component named in resources.json.
//
// This is zarf.sh, which does the same with yq. Keeping the list in the file rather than letting
// zarf discover it at create time is zarf's own recommendation for an air-gapped package: discovery
// reads the charts and manifests, and a package built where the charts cannot be read has to be
// told. It is also what makes a pin bump reviewable -- the image versions move in the diff.
func Images(pkg string) error {
	resources, err := Load(pkg)
	if err != nil {
		return err
	}

	found, err := findImages(filepath.Join(Root, pkg))
	if err != nil {
		return err
	}

	path := filepath.Join(Root, pkg, "zarf.yaml")
	source, err := os.ReadFile(path) //nolint:gosec // a path under Root
	if err != nil {
		return err
	}

	updated := string(source)
	for _, project := range resources.Projects {
		images, ok := found[project.Component]
		if !ok {
			return fmt.Errorf("zarf dev find-images reported no images for the component %s, which "+
				"%s names. Check that the component exists in %s and that its manifests are vendored",
				project.Component, ResourcesFile, path)
		}

		updated, err = replaceImages(updated, project.Component, images)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}

	if updated == string(source) {
		fmt.Printf("%s: images unchanged\n", path)
		return nil
	}

	fmt.Printf("%s: images updated\n", path)
	return os.WriteFile(path, []byte(updated), 0o644) //nolint:gosec // an example package is world-readable
}

// findImages runs zarf's image discovery over a package directory and returns the images per
// component.
//
// --skip-cosign is passed because discovery otherwise reaches a transparency log for each image,
// which is a network round trip per image to learn nothing this needs. Zarf writes its log to
// stderr and the document to stdout, so only stdout is parsed.
func findImages(directory string) (map[string][]string, error) {
	binary := os.Getenv(EnvBinary)
	if binary == "" {
		binary = "zarf"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return nil, fmt.Errorf("%s is not on PATH: image discovery runs the installed zarf. Set %s "+
			"to name one elsewhere", binary, EnvBinary)
	}

	command := exec.Command(binary, "dev", "find-images", directory, "--skip-cosign", "--no-color") //nolint:gosec // a binary from PATH and a path under Root
	command.Stderr = os.Stderr

	printed, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("zarf dev find-images failed: %w", err)
	}

	var output findImagesOutput
	if err := goyaml.Unmarshal(printed, &output); err != nil {
		return nil, fmt.Errorf("unable to parse what zarf dev find-images printed: %w", err)
	}

	images := map[string][]string{}
	for _, component := range output.Components {
		// Sorted, because the discovery order is the order the manifests were read in and a
		// reordered list is a diff that says nothing happened.
		sorted := append([]string(nil), component.Images...)
		sort.Strings(sorted)
		images[component.Name] = sorted
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("zarf dev find-images found no images at all in %s", directory)
	}
	return images, nil
}

// componentImagesPattern finds one component's `images:` list in a zarf.yaml, given the
// component's name.
//
// The surgery is line-based rather than a parse-and-marshal of the whole document, and that is the
// point: zarf.yaml here is a hand-written teaching artifact full of comments, and round-tripping it
// through a YAML marshaller would reformat it and drop every one of them. Only the list items under
// `images:` are rewritten; the key, its indentation and any comment above it are left alone.
//
// Every part of it matches whole lines at a known indentation, which is what keeps it inside one
// component. The gap between the component's name and its `images:` key matches only lines indented
// four spaces or more, or blank -- so it cannot run past the next `  - name:` two spaces in and
// rewrite a later component's list.
//
// The list group matches list items only, and deliberately not blank lines. Allowing a blank line
// into it makes the match run to the end of the blank line that separates this component from the
// next, and the replacement then eats the separator: the first run reflows the file and every run
// after it is a no-op, so the damage is invisible in the second diff.
//
// Group 3 is the list, and the only thing replaced.
const componentImagesPattern = `(?m)^  - name: %s\n((?:    [^\n]*\n|\n)*?)(    images:[^\n]*\n)((?:      - [^\n]*\n)*)`

// replaceImages rewrites one component's images list, and reports if it found no such component.
func replaceImages(source, component string, images []string) (string, error) {
	pattern, err := regexp.Compile(fmt.Sprintf(componentImagesPattern, regexp.QuoteMeta(component)))
	if err != nil {
		return "", err
	}

	matches := pattern.FindAllStringSubmatchIndex(source, -1)
	switch len(matches) {
	case 1:
	case 0:
		return "", fmt.Errorf("no component named %s with an images: key was found, so its image "+
			"list was not updated. A component that deploys images needs the key present, even empty",
			component)
	default:
		// Two matches mean either two components of that name -- which a flavored package has, and
		// this one does not -- or a pattern that ran past the end of the first. Either way, writing
		// to one of them arbitrarily is worse than stopping.
		return "", fmt.Errorf("%d components named %s were found, and this expects one", len(matches), component)
	}

	var rendered strings.Builder
	for _, image := range images {
		rendered.WriteString("      - " + image + "\n")
	}

	// Indices 6 and 7 are the third group: the list, which is all that is replaced.
	match := matches[0]
	return source[:match[6]] + rendered.String() + source[match[7]:], nil
}
