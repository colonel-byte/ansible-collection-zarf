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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Tests for the progress reader: what it recognises as a component boundary in zarf's log
// stream, and the heartbeats it writes from one.

func TestComponentName(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
		ok   bool
	}{
		{
			name: "a JSON record naming the component",
			line: `{"time":"2026-10-02T14:32:01Z","level":"INFO","msg":"Deploying component","name":"zarf-registry"}`,
			want: "zarf-registry",
			ok:   true,
		},
		{
			name: "a JSON record carrying the name under another key",
			line: `{"level":"INFO","msg":"deploying component","component":"zarf-agent"}`,
			want: "zarf-agent",
			ok:   true,
		},
		{
			name: "a JSON record that names no component falls back to the message",
			line: `{"level":"INFO","msg":"Deploying component"}`,
			want: "Deploying component",
			ok:   true,
		},
		{
			name: "the console rendering",
			line: `  •  Deploying Zarf component - zarf-seed-registry`,
			want: "zarf-seed-registry",
			ok:   true,
		},
		{
			name: "an unrelated record",
			line: `{"level":"INFO","msg":"Loading Zarf Package"}`,
			ok:   false,
		},
		{
			name: "an unrelated line",
			line: `  •  Waiting for the registry to be ready`,
			ok:   false,
		},
		{
			name: "a blank line",
			line: "   ",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := componentName(tt.line)
			if ok != tt.ok {
				t.Fatalf("componentName(%q) reported %v, want %v", tt.line, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("componentName(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

// TestProgressWritesHeartbeats proves the side channel: every component boundary the log stream
// announces leaves a whole, parseable status document in place.
func TestProgressWritesHeartbeats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.json")
	hb := NewHeartbeat(path)
	prog := newProgress(hb, 2)

	prog.line(`{"msg":"Deploying component","name":"zarf-injector"}`)
	first := readStatus(t, path)
	if first.Phase != "zarf-injector" || first.Index != 1 || first.Total != 2 || first.Status != "running" {
		t.Fatalf("the first heartbeat was %+v", first)
	}

	// A repeat of the same component is not a new one.
	prog.line(`{"msg":"Deploying component","name":"zarf-injector"}`)
	if again := readStatus(t, path); again.Index != 1 {
		t.Errorf("a repeated announcement advanced the index to %d", again.Index)
	}

	prog.line(`{"msg":"Deploying component","name":"zarf-registry"}`)
	second := readStatus(t, path)
	if second.Phase != "zarf-registry" || second.Index != 2 || second.Status != "running" {
		t.Fatalf("the second heartbeat was %+v", second)
	}

	prog.finish("completed", nil)
	done := readStatus(t, path)
	if done.Status != "completed" || done.Phase != "zarf-registry" {
		t.Errorf("the final heartbeat was %+v", done)
	}

	hb.Clear()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the status file survived Clear: %v", err)
	}
}

func readStatus(t *testing.T, path string) Status {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // the path is the test's own temporary directory
	if err != nil {
		t.Fatalf("unable to read the status file: %v", err)
	}
	var st Status
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("the status file did not hold one JSON document: %v (%s)", err, b)
	}
	return st
}
