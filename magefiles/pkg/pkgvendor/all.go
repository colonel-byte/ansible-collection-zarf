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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// VendorAll vendors every example package that declares a resources.json.
//
// The per-package functions take a package name rather than being methods on a list, because the
// mage targets are the only callers and they always want all of them. Keeping the single-package
// form is what lets a failure name one package and stop, rather than carrying on into a tree whose
// pin no longer matches its manifests.
func VendorAll() error {
	return each(Vendor)
}

// ImagesAll refreshes the image lists of every example package that declares a resources.json.
func ImagesAll() error {
	return each(Images)
}

// RefreshAll moves every pin to its upstream's latest release and vendors the result.
func RefreshAll() error {
	return each(Refresh)
}

// each runs one operation over every example package that declares a resources.json.
func each(operation func(string) error) error {
	packages, err := Packages()
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("no package under %s declares a %s", Root, ResourcesFile)
	}

	for _, pkg := range packages {
		fmt.Printf("== %s\n", pkg)
		if err := operation(pkg); err != nil {
			return fmt.Errorf("%s: %w", pkg, err)
		}
	}
	return nil
}

// latestRelease is the field of GitHub's latest-release response this reads.
type latestRelease struct {
	TagName string `json:"tag_name"`
}

// pinnedTag matches the tag line in a resources.json. The file is rewritten by replacing that one
// line rather than by marshalling the struct back out, so that the key order, the indentation and
// the $schema line survive a refresh unchanged.
var pinnedTag = regexp.MustCompile(`(?m)^(\s*"tag":\s*")([^"]*)(")`)

// Refresh moves one package's pin to the upstream's latest release, then vendors it.
//
// This is what makes the pin owned rather than abandoned. magefiles/AGENTS.md asks for one of two
// owners for a pinned value, and this is the first: a mage target that discovers the current value
// and writes it back. The Renovate annotation in resources.json is the second owner, for the
// repository that has Renovate configured; this one needs nothing configured anywhere.
//
// A refresh that finds the pin already current vendors anyway. Re-vendoring is idempotent, and the
// alternative -- skipping -- would mean a tree whose manifests were deleted could not be restored
// by the command whose name says it would.
func Refresh(pkg string) error {
	resources, err := Load(pkg)
	if err != nil {
		return err
	}

	latest, err := latestTag(resources.Repo)
	if err != nil {
		return err
	}

	path := filepath.Join(Root, pkg, ResourcesFile)
	if latest == resources.Tag {
		fmt.Printf("%s: %s is already the latest release of %s\n", path, latest, resources.Repo)
		return Vendor(pkg)
	}

	source, err := os.ReadFile(path) //nolint:gosec // a path under Root
	if err != nil {
		return err
	}
	if len(pinnedTag.FindAllIndex(source, -1)) != 1 {
		return fmt.Errorf("%s does not hold exactly one \"tag\" line, so the pin was not moved", path)
	}

	updated := pinnedTag.ReplaceAll(source, []byte("${1}"+latest+"${3}"))
	if err := os.WriteFile(path, updated, 0o644); err != nil { //nolint:gosec // a declaration file is world-readable
		return err
	}

	fmt.Printf("%s: %s -> %s\n", path, resources.Tag, latest)
	return Vendor(pkg)
}

// latestTag asks GitHub for a repository's latest release tag.
//
// Unauthenticated, which is the point: the endpoint is public, and needing a token to move a pin
// is what stops a pin being moved. A rate-limited answer fails with GitHub's own status, which
// says what to wait for.
func latestTag(repo string) (string, error) {
	owner, name, err := repoParts(repo)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, name)
	client := &http.Client{Timeout: fetchTimeout}

	response, err := client.Get(url) //nolint:gosec,noctx // a URL built from the declared repository
	if err != nil {
		return "", fmt.Errorf("unable to ask %s for the latest release: %w", url, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s answered %s", url, response.Status)
	}

	var release latestRelease
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("%s named no tag in its latest release", repo)
	}
	return release.TagName, nil
}

// repoParts splits a GitHub repository URL into its owner and name.
func repoParts(repo string) (string, string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSuffix(repo, "/"), ".git")
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%q is not a github.com repository URL this can read a release from", repo)
	}
	return parts[0], parts[1], nil
}

// LintAll runs `zarf dev lint` over every example package, including the ones that vendor nothing.
//
// Every package rather than only the ones with a resources.json: linting is the only check an
// example gets, and operator-external-secrets -- which ships its own manifests and vendors nothing -- needs it
// as much as the one that does.
func LintAll() error {
	binary := os.Getenv(EnvBinary)
	if binary == "" {
		binary = "zarf"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("%s is not on PATH: linting a package runs the installed zarf. Set %s to "+
			"name one elsewhere", binary, EnvBinary)
	}

	entries, err := os.ReadDir(Root)
	if err != nil {
		return err
	}

	linted := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(Root, entry.Name())
		if _, err := os.Stat(filepath.Join(directory, "zarf.yaml")); err != nil {
			continue
		}

		fmt.Printf("== %s\n", directory)
		command := exec.Command(binary, "dev", "lint", directory, "--no-color") //nolint:gosec // a binary from PATH and a path under Root
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("%s does not lint: %w", directory, err)
		}
		linted++
	}

	if linted == 0 {
		return fmt.Errorf("no package under %s holds a zarf.yaml, so nothing was linted", Root)
	}
	fmt.Printf("linted %d example package(s)\n", linted)
	return nil
}
