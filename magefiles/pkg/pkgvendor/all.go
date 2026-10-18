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

// LintAll runs `zarf dev lint` over every example package, including the ones that vendor nothing.
//
// Every package rather than only the ones with a resources.json: linting is the only check an
// example gets, and operator-external-secrets -- which ships its own manifests and vendors nothing -- needs it
// as much as the one that does.
func LintAll() error {
	binary, err := zarfBinary("linting a package")
	if err != nil {
		return err
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
