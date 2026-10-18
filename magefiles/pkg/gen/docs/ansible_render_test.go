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

package docs

import (
	"strings"
	"testing"
	"unicode/utf8"

	goyaml "github.com/goccy/go-yaml"
)

// These helpers are pure functions that nothing else checks, and everything they get wrong is
// quiet: a generated page is written, committed, and read by somebody who has no way to tell a
// rendering bug from the source saying that. The generator has no "wrong" to fail on.

// TestRenderYAMLValueKeepsEmptyValuesReadable holds the distinction the generated tables exist to
// make: a default that is an empty string, and no default at all, are different facts and must not
// render as the same empty cell.
func TestRenderYAMLValueKeepsEmptyValuesReadable(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "no default at all",
			value: nil,
			want:  "null",
		},
		{
			name:  "an empty string default",
			value: "",
			want:  `""`,
		},
		{
			name:  "a plain string is left unquoted, because the cell is a code span already",
			value: "localhost",
			want:  "localhost",
		},
		{
			name:  "a boolean",
			value: true,
			want:  "true",
		},
		{
			name:  "a number",
			value: 31999,
			want:  "31999",
		},
		{
			name:  "an empty mapping",
			value: map[string]any{},
			want:  "{}",
		},
		{
			name:  "an empty sequence",
			value: []any{},
			want:  "[]",
		},
		{
			name: "a mapping default, which is what an engine-to-namespace variable is",
			value: goyaml.MapSlice{
				{
					Key:   "fluxcd",
					Value: "flux-system",
				},
				{
					Key:   "argocd",
					Value: "argocd",
				},
			},
			want: "fluxcd: flux-system\nargocd: argocd",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderYAMLValue(tt.value); got != tt.want {
				t.Errorf("renderYAMLValue(%#v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestEscapeAngleBracketsLeavesTheLineBreakAlone pins the ordering the comment on
// escapeAngleBrackets depends on.
//
// Prose in a description genuinely contains angle brackets -- `node-role.kubernetes.io/<profile>`,
// `<tag>`, `<redacted>` -- and mdBook reads an unescaped one as an unclosed HTML tag, which eats
// the rest of the cell. A cell's own line break is a literal `<br>` written later by
// markdown.EscapeTableCell, so this must not be able to reach one.
func TestEscapeAngleBracketsLeavesTheLineBreakAlone(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a placeholder in prose",
			in:   "node-role.kubernetes.io/<profile>",
			want: "node-role.kubernetes.io/&lt;profile&gt;",
		},
		{
			name: "the redaction placeholder the modules report",
			in:   "<redacted>",
			want: "&lt;redacted&gt;",
		},
		{
			name: "a comparison in a version constraint",
			in:   ">=1.30.0 <1.34.0",
			want: "&gt;=1.30.0 &lt;1.34.0",
		},
		{
			name: "text with no brackets is untouched",
			in:   "the zarf to run",
			want: "the zarf to run",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeAngleBrackets(tt.in); got != tt.want {
				t.Errorf("escapeAngleBrackets(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestPaddedTableAlignsEveryLine holds the one formatting rule ansible/AGENTS.md states about
// generated tables, which is the rule the pre-commit hooks would otherwise rewrite on every run:
// every line of a table is the same length, with each cell padded to its column's widest.
//
// A generator whose output the formatters rewrite produces a diff on every commit that touches
// anything, which is how a generated file stops being trusted.
func TestPaddedTableAlignsEveryLine(t *testing.T) {
	lines := paddedTable(
		[]string{"Variable", "Type", "Default"},
		[][]string{
			{"zarf_binary", "`str`", "zarf"},
			{"zarf_engine_namespaces_are_long", "`dict`", `""`},
			{"zarf_quiet", "`bool`", "true"},
		},
	)

	if len(lines) != 5 {
		t.Fatalf("the table has %d line(s), want a header, a delimiter and three rows:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}

	width := utf8.RuneCountInString(lines[0])
	for i, line := range lines {
		if n := utf8.RuneCountInString(line); n != width {
			t.Errorf("line %d is %d runes wide and the header is %d:\n%s",
				i, n, width, strings.Join(lines, "\n"))
		}
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			t.Errorf("line %d is not delimited by pipes: %q", i, line)
		}
	}

	// The delimiter row's dashes run out to the column width rather than being a fixed three.
	if !strings.HasPrefix(lines[1], "| ---") || strings.Contains(lines[1], "| --- |") {
		t.Errorf("the delimiter row is not padded to the column widths: %q", lines[1])
	}
}

// TestRenderCLIFlagKeepsNoneAsProse holds the one value in a cli_flag cell that is not a flag.
//
// "None" is the documented spelling for a parameter that renders no flag -- one the action plugin
// consumes, or one passed to zarf as an environment variable -- and dressing it as a code span
// would read as a flag literally spelled None.
func TestRenderCLIFlagKeepsNoneAsProse(t *testing.T) {
	for _, tt := range []struct {
		name string
		flag string
		want string
	}{
		{
			name: "a real flag is a code span",
			flag: "--components",
			want: "`--components`",
		},
		{
			name: "the documented spelling for no flag",
			flag: "None",
			want: "None",
		},
		{
			name: "an option that documents nothing at all",
			flag: "",
			want: "None",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderCLIFlag(namedOption{docOption: docOption{CLIFlag: tt.flag}}); got != tt.want {
				t.Errorf("renderCLIFlag(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

// TestRenderTypeNamesTheElementType holds that a list says what goes in it, since `list` alone
// tells a reader nothing they can act on.
func TestRenderTypeNamesTheElementType(t *testing.T) {
	if got, want := renderType(namedOption{docOption: docOption{Type: "str"}}), "`str`"; got != want {
		t.Errorf("renderType = %q, want %q", got, want)
	}
	if got, want := renderType(namedOption{
		docOption: docOption{
			Type:     "list",
			Elements: "str",
		},
	}), "`list` of `str`"; got != want {
		t.Errorf("renderType = %q, want %q", got, want)
	}
}
