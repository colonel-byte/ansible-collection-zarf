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
	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/testrunner"
	"github.com/magefile/mage/mg"
)

type (
	Test mg.Namespace
)

// Unit runs go test over every package except the e2e suites under test/.
func (Test) Unit() error {
	return testrunner.Unit()
}

// EndToEndNonCluster runs the suite that needs no cluster: the read-only modules and the state
// redaction, through a real ansible-playbook against a stub zarf. Needs ansible-playbook.
func (Test) EndToEndNonCluster() error {
	return testrunner.RunE2ENonCluster()
}

// EndToEndCluster runs the suite that walks both module files across two k3d clusters. Needs
// k3d, a zarf on PATH, ansible-playbook, Docker, and the network to pull three packages once.
func (Test) EndToEndCluster() error {
	return testrunner.RunE2ECluster()
}

// CleanClusters removes the k3d clusters left behind by an interrupted cluster suite.
func (Test) CleanClusters() error {
	return testrunner.DeleteK3dClusters()
}
