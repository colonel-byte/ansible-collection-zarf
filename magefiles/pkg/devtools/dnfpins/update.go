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

// This file rewrites the pins in place: the ARG lines in the management node image's Dockerfile
// and the build_args the dockers_v2 pipe passes it. Each rewriter is anchored to the line it
// replaces so a pin update never reformats the file around it.
//
// Both files carry the same two versions, and they have to agree: the ARG lines are the default
// a local `docker build` gets, and the build_args are what the release build passes. A rewrite
// that moved one and not the other would leave the released image and the one a contributor
// builds on different versions of ansible-core, which is why one task writes both.

package dnfpins

import (
	"fmt"
	"os"
	"regexp"
)

// UpdateDockerfileAnsiblePins updates the ARG lines in containers/ansible/Dockerfile.
func UpdateDockerfileAnsiblePins(path, ansibleVer, bashVer string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)

	reAnsible := regexp.MustCompile(`(?m)^ARG ANSIBLE_CORE_VERSION="[^"]*"`)
	if !reAnsible.MatchString(text) {
		return fmt.Errorf("did not find ARG ANSIBLE_CORE_VERSION in %s", path)
	}
	text = reAnsible.ReplaceAllString(text, fmt.Sprintf(`ARG ANSIBLE_CORE_VERSION="%s"`, ansibleVer))

	reBash := regexp.MustCompile(`(?m)^ARG BASH_COMPLETION_VERSION="[^"]*"`)
	if !reBash.MatchString(text) {
		return fmt.Errorf("did not find ARG BASH_COMPLETION_VERSION in %s", path)
	}
	text = reBash.ReplaceAllString(text, fmt.Sprintf(`ARG BASH_COMPLETION_VERSION="%s"`, bashVer))

	return os.WriteFile(path, []byte(text), 0o644)
}

// UpdateGoreleaserDnfPins updates the zarf-ansible build_args in .goreleaser.yaml.
func UpdateGoreleaserDnfPins(path string, pins *AlmaLinuxPins) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)

	reAnsibleBlock := regexp.MustCompile(`(?s)(- id: zarf-ansible\n.*?build_args:.*?\n)(\s+ANSIBLE_CORE_VERSION:\s*)[^\n]+(\n\s+BASH_COMPLETION_VERSION:\s*)[^\n]+`)
	if !reAnsibleBlock.MatchString(text) {
		return fmt.Errorf("did not find zarf-ansible build_args in %s", path)
	}
	text = reAnsibleBlock.ReplaceAllString(text, fmt.Sprintf("${1}${2}%s${3}%s", pins.AnsibleCore, pins.BashCompletion))

	return os.WriteFile(path, []byte(text), 0o644)
}
