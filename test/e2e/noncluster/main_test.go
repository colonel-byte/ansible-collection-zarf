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

// Package noncluster holds the end-to-end tests that need no cluster: the read-only modules and
// the state redaction, driven through a real ansible-playbook against a stub zarf.
//
// Every path in this suite -- the collection, the playbooks, the stubs -- is spelled relative to
// the repository root, so TestMain moves there before anything runs.
package noncluster

import (
	"log"
	"os"
	"testing"

	"github.com/colonel-byte/ansible-collection-zarf/internal/testenv"
)

func TestMain(m *testing.M) {
	if _, _, err := testenv.Bootstrap(); err != nil {
		log.Fatal(err)
	}

	// This suite builds a real package with a real zarf for the inspect fixture, so every run left
	// a zarf staging directory behind with nothing to ever remove it -- beside zarf-e2e, the package
	// cache that is kept between runs on purpose, so build/tmp stopped being a directory a developer
	// can judge at a glance.
	runTmp, err := testenv.RunTempDir("zarf-e2e-noncluster-")
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()

	// After the run rather than deferred: os.Exit does not run deferred functions.
	if err := os.RemoveAll(runTmp); err != nil {
		log.Printf("unable to remove the suite temporary directory %s: %v", runTmp, err)
	}
	os.Exit(code)
}
