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

package build

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"

	"github.com/colonel-byte/ansible-collection-zarf/internal/zarfmod"
)

// ZarfBinary is the wrapper every zarf module file is a symlink to, and ZarfModulePackage is the
// main package it is built from. One binary under several names is ZEP-0072's own shape -- see
// cmd/zarfmod -- and the names are zarfmod.ModuleFiles(), so a module added to the dispatch table
// gets its symlink here with nothing to keep in step.
const (
	ZarfBinary        = "zarfmod"
	ZarfModulePackage = "./cmd/zarfmod"
)

// ZarfModuleFiles builds the zarf Ansible module wrapper for this host into build/, and links each
// module file name to it.
//
// It is built for the host rather than for a release target list because what drives them is the
// e2e suite and an operator reproducing a module run; the release builds come from
// .goreleaser.yaml. No version is stamped in: a module file reports what zarf did, and has no
// version of its own to report.
func ZarfModuleFiles() error {
	out := filepath.Join(BuildDir, ZarfBinary)
	fmt.Println("building: " + out)

	if err := os.MkdirAll(BuildDir, 0o755); err != nil {
		return err
	}
	if err := os.Remove(out); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	env := map[string]string{"CGO_ENABLED": "0"}
	if err := sh.RunWithV(env, "go", "build", "-trimpath", "-o", out, ZarfModulePackage); err != nil {
		return err
	}

	for _, name := range zarfmod.ModuleFiles() {
		link := filepath.Join(BuildDir, name)
		fmt.Println("linking: " + link)

		if err := os.Remove(link); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		// Relative, so build/ can be moved or mounted elsewhere with the links still resolving --
		// which is what the container image and the e2e suites do with it.
		if err := os.Symlink(ZarfBinary, link); err != nil {
			return err
		}
	}
	return nil
}
