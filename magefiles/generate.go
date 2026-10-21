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
	"context"
	"fmt"

	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/devtools/dnfpins"
	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/gen/docs"
	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/gen/schema"
	"github.com/magefile/mage/mg"
)

type (
	Generate mg.Namespace
)

// Document regenerates every generated page in the book: the collection reference under
// docs/ansible, the Go reference under docs/golang, docs/index.md and docs/security.md, and
// docs/SUMMARY.md.
func (Generate) Document() error {
	return docs.GenerateDocument()
}

// Schema regenerates the JSON Schema documents under schema/ from the Go types in api/. Run it
// after changing a type or a doc comment there; the schema's descriptions are those comments.
func (Generate) Schema() error {
	return schema.Generate()
}

// DnfPins queries the AlmaLinux 10 repodata over HTTP and rewrites the pinned versions of the
// packages the management node image installs with dnf -- ansible-core and bash-completion --
// in containers/ansible/Dockerfile and in the zarf-ansible build_args in .goreleaser.yaml.
//
// Dependabot's docker ecosystem moves the two FROM digests and nothing else, so these two pins
// are this target's to move. Run it, then read the diff: a version that jumped a minor line is
// a change to what the image installs, not a refresh.
func (Generate) DnfPins(ctx context.Context) error {
	fmt.Println("Querying AlmaLinux 10 repodata for latest package versions...")
	pins, err := dnfpins.QueryLatestAlmaLinuxPackages(ctx)
	if err != nil {
		return fmt.Errorf("querying AlmaLinux packages: %w", err)
	}

	fmt.Printf("Discovered versions:\n  ansible-core:    %s\n  bash-completion: %s\n",
		pins.AnsibleCore, pins.BashCompletion)

	if err := dnfpins.UpdateDockerfileAnsiblePins(dnfpins.AnsibleDockerfilePath, pins.AnsibleCore, pins.BashCompletion); err != nil {
		return fmt.Errorf("updating %s: %w", dnfpins.AnsibleDockerfilePath, err)
	}
	fmt.Printf("Updated %s\n", dnfpins.AnsibleDockerfilePath)

	if err := dnfpins.UpdateGoreleaserDnfPins(dnfpins.GoreleaserConfigPath, pins); err != nil {
		return fmt.Errorf("updating %s: %w", dnfpins.GoreleaserConfigPath, err)
	}
	fmt.Printf("Updated %s\n", dnfpins.GoreleaserConfigPath)

	return nil
}
