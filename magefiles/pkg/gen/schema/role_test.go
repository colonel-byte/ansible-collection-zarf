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

package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/colonel-byte/ansible-collection-zarf/api/colonel-byte.dev/v1alpha1/prerequisites"
)

// Tests that hold the colonel_byte.zarf.prerequisites role against the format it checks.

// roleTaskFiles are the colonel_byte.zarf.prerequisites role's task files, which are the only
// consumer of the requires half of the format that this repository ships.
var roleTaskDir = filepath.Join("ansible", "colonel_byte", "zarf", "roles", "prerequisites", "tasks")

// roleTaskFiles is every task file in the role, found rather than listed. A list would go stale
// the next time the role is split into another file, and it would go stale silently: the test
// would still pass, over less of the role than it claims to read.
func roleTaskFiles(t *testing.T) []string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(roleTaskDir, "*.yml"))
	if err != nil {
		t.Fatalf("unable to list the role's task files: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no task files under %s, so this test proves nothing", roleTaskDir)
	}
	return files
}

// TestEveryRequirementIsNamedByTheRole holds the format against the role that checks it.
//
// This is the one drift in the prerequisites format that nothing else catches. Add a section to
// Requires, run `generate:schema`, and TestCommittedSchemasAreCurrent and
// TestExampleDocumentsMatchTheTypes both pass: the schema describes the new field, the examples
// decode, and the role goes on ignoring it. A package then declares a prerequisite, the role
// reports success, and nothing checked it -- which is the silent success the whole format exists
// to prevent, reached through the format rather than through a cluster.
//
// What it proves is narrower than its name suggests, and deliberately so: it holds that every
// field name under `requires` appears somewhere in the role's tasks, not that the role checks it
// correctly. A field named `version` added to a new section would pass for free, because `version`
// is already there. The case it does catch is the one that actually happens -- a new section or a
// new field under a new name, which no expression in the role mentions -- and when it fires, the
// fix is to check the field or to record it in zarf_prerequisites_skipped. Naming it as unchecked
// is a legitimate way to pass this test, because an unchecked field an operator is told about is
// the honest outcome; silently ignoring one is not.
//
// provides is not covered. Nothing in this repository reads it: it is declared for an orchestrator
// ordering a bundle, which is the half of the contract this collection does not implement.
func TestEveryRequirementIsNamedByTheRole(t *testing.T) {
	var tasks string
	for _, path := range roleTaskFiles(t) {
		source, err := os.ReadFile(path) //nolint:gosec // a path from the glob above
		if err != nil {
			t.Fatalf("unable to read %s: %v", path, err)
		}
		tasks += string(source)
	}

	names := jsonFieldNames(reflect.TypeFor[prerequisites.Requires]())
	if len(names) == 0 {
		t.Fatal("no json field names found on prerequisites.Requires, so this test proves nothing")
	}

	for _, name := range names {
		// Matched as a whole word rather than as a substring. `names` is a substring of
		// `namespace`, which the role uses everywhere, so a plain Contains passed for a field no
		// expression in the role mentioned -- the one thing this test exists to catch.
		if !mentionsField(tasks, name) {
			t.Errorf("no task in the prerequisites role mentions %q, which requires.%s declares. "+
				"Either check it, or append it to zarf_prerequisites_skipped so an operator is "+
				"told it was not.", name, name)
		}
	}
}

// mentionsField reports whether the role's tasks name this JSON field as a field rather than as
// part of a longer word. `names` inside `namespace` is not a mention of `names`.
func mentionsField(tasks, name string) bool {
	for _, index := range regexp.MustCompile(regexp.QuoteMeta(name)).FindAllStringIndex(tasks, -1) {
		before := byte(' ')
		if index[0] > 0 {
			before = tasks[index[0]-1]
		}
		after := byte(' ')
		if index[1] < len(tasks) {
			after = tasks[index[1]]
		}
		if !isFieldChar(before) && !isFieldChar(after) {
			return true
		}
	}
	return false
}

// isFieldChar reports whether a byte could be part of a longer identifier. An underscore counts,
// so `package_requirements` is not a mention of `packages`.
func isFieldChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		return true
	default:
		return false
	}
}
