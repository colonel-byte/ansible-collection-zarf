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
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/zarf-dev/zarf/src/config"
)

// TestLineWriterSplits proves the progress reader sees whole lines however zarf's log handler
// happens to chunk its writes. Running zarf as a child got this from bufio.Scanner over a pipe;
// linked in, the writes arrive as the handler makes them and nothing promises one ends on a
// newline.
func TestLineWriterSplits(t *testing.T) {
	cases := []struct {
		name   string
		writes []string
		want   []string
	}{
		{
			name:   "one write per line",
			writes: []string{"first\n", "second\n"},
			want:   []string{"first", "second"},
		},
		{
			name:   "several lines in one write",
			writes: []string{"first\nsecond\nthird\n"},
			want:   []string{"first", "second", "third"},
		},
		{
			name:   "a line split across writes",
			writes: []string{"fir", "st\nsec", "ond\n"},
			want:   []string{"first", "second"},
		},
		{
			name:   "an empty line is a line",
			writes: []string{"first\n\nsecond\n"},
			want:   []string{"first", "", "second"},
		},
		{
			name:   "a trailing line with no newline still reports",
			writes: []string{"first\nhalf"},
			want:   []string{"first", "half"},
		},
		{
			name:   "nothing written reports nothing",
			writes: nil,
			want:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			w := &lineWriter{
				onLine: func(line string) {
					got = append(got, line)
				},
				mirror: io.Discard,
			}

			for _, write := range tc.writes {
				n, err := w.Write([]byte(write))
				if err != nil {
					t.Fatalf("Write(%q) returned %v", write, err)
				}
				// A short write would make the log handler think it failed.
				if n != len(write) {
					t.Errorf("Write(%q) reported %d bytes, want %d", write, n, len(write))
				}
			}
			w.flush()

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("reported lines %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLineWriterMirrorsEverything proves stderr gets what zarf wrote, byte for byte. A human
// reading a failed task's output should see a half-written line, not have it held back waiting for
// a newline that never comes.
func TestLineWriterMirrorsEverything(t *testing.T) {
	var mirror strings.Builder
	w := &lineWriter{mirror: &mirror}

	for _, write := range []string{"fir", "st\nsec", "ond"} {
		if _, err := w.Write([]byte(write)); err != nil {
			t.Fatalf("Write(%q) returned %v", write, err)
		}
	}

	if got := mirror.String(); got != "first\nsecond" {
		t.Errorf("mirrored %q, want %q", got, "first\nsecond")
	}
}

// TestLinkedZarfVersionIsFilledBeforeAnythingReadsIt pins the reason this package has an init.
//
// zarf stamps CLIVersion in with ldflags at release time, so linked in as a library it reads
// "unset-development-only" unless something fills it. That string is not merely unhelpful in a
// result -- config.GetInitPackageName builds the init package filename from it, so leaving it
// would send zarf's package search after a file no operator ever staged.
//
// This deliberately does not call the setup itself. runInit reports the version before it asks
// the runner to do anything, so filling it in on the way into Run was a silent wrong answer in
// the result; reading it cold here is what proves that cannot come back.
func TestLinkedZarfVersionIsFilledBeforeAnythingReadsIt(t *testing.T) {
	if config.CLIVersion == config.UnsetCLIVersion || config.CLIVersion == "" {
		t.Fatalf("the linked-in zarf reports %q, so zarf will look for an init package named "+
			"after it rather than after a real release", config.CLIVersion)
	}
	// The init package name is the thing that actually breaks, so assert on it rather than only
	// on the version it is built from.
	if name := config.GetInitPackageName(); !strings.Contains(name, config.CLIVersion) {
		t.Errorf("the init package name is %q, which does not carry the version %q",
			name, config.CLIVersion)
	}
}
