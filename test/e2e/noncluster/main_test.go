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
	os.Exit(m.Run())
}
