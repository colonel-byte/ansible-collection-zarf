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

//go:build mage
// +build mage

package main

import (
	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/pkgvendor"
	"github.com/magefile/mage/mg"
)

type (
	Examples mg.Namespace
)

// Vendor downloads the pinned upstream release for every example package that declares one and
// rewrites what follows from the pin: the manifests under the package's upstream/ tree, a
// kustomization.yaml per project, and the package version in its zarf.yaml.
//
// It reaches the network for a release asset. Run it when a pin in a package's resources.json
// moves, and follow it with examples:images.
func (Examples) Vendor() error {
	return pkgvendor.VendorAll()
}

// Images rewrites each example package's component image lists from what `zarf dev find-images`
// discovers in its manifests. Needs a zarf on PATH; set ZARF_ANSIBLE_FLAGS_BINARY to name one
// elsewhere.
func (Examples) Images() error {
	return pkgvendor.ImagesAll()
}

// Lint runs `zarf dev lint` over every example package, which is the only thing that checks them:
// nothing builds or deploys an example, because creating one reaches the network.
func (Examples) Lint() error {
	return pkgvendor.LintAll()
}
