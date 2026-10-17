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

package zarfmod

import (
	"strings"
	"testing"
)

// FuzzComponentName holds the one invariant progress's state machine rests on: a line that
// announces a component names one.
//
// progress.line treats a non-empty current as "a component is in flight", and progress.finish
// returns early without it, so a ("", true) answer would set current back to empty and drop the
// last heartbeat of the run. Nothing would crash and no test would fail -- the progress display
// would simply stop reporting. componentName reads a subprocess's stdout in a format the operator
// can override, which is why this is worth a fuzzer rather than another table row.
func FuzzComponentName(f *testing.F) {
	for _, seed := range []string{
		`{"time":"2026-10-02T14:32:01Z","level":"INFO","msg":"Deploying component","name":"zarf-registry"}`,
		`{"level":"INFO","msg":"deploying component","component":"zarf-agent"}`,
		`{"level":"INFO","msg":"Deploying component"}`,
		`{"msg":"Deploying component","name":""}`,
		`  •  Deploying Zarf component - zarf-seed-registry`,
		`{"level":"INFO","msg":"Loading Zarf Package"}`,
		`{`,
		`{not json at all: deploying component "x"`,
		"   ",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, line string) {
		name, ok := componentName(line)
		if ok && strings.TrimSpace(name) == "" {
			t.Fatalf("componentName(%q) announced a component and named nothing", line)
		}
	})
}
