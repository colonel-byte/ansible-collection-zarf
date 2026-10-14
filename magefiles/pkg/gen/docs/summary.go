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
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/nao1215/markdown"
)

// printExcludedNote marks the chapters docs/css/print.css drops from the print page.
const printExcludedNote = "<!-- Excluded from the print page (print.html) by docs/css/print.css. -->"

// generateSummary writes docs/SUMMARY.md, the book's table of contents.
func generateSummary() error {
	fmt.Println("docs/SUMMARY.md")
	var builder strings.Builder

	md := markdown.NewMarkdown(&builder)

	// Section order matters beyond the sidebar: docs/css/print.css drops everything from the
	// first Development chapter to the end of the document out of the print page, so
	// Development has to stay ahead of the sections meant to be excluded with it. Workflows and
	// Agent sit after it for that reason, and anything added after them is excluded from print
	// too. The rule keys on the mdBook slug of the first Development chapter's own H1, so
	// reordering this list means rewording that selector with it.
	summary := []struct {
		title  string
		folder string
		regex  string
		extra  string
		note   string
		// lines supplies the section's entries itself, for a section the folder walk below
		// cannot produce. It replaces regex and folder rather than adding to them.
		lines func() ([]string, error)
	}{
		{
			// Both entries are prefix chapters: mdBook gives a line before the first list item
			// no number and places it ahead of every numbered chapter, which is where the front
			// page and the security policy belong.
			title: "Index",
			extra: "[readme](index.md)\n[security](security.md)",
		},
		{
			title:  "Guides",
			regex:  `(.+)\.md`,
			folder: "guides",
		},
		{
			title: "Ansible",
			lines: ansibleSummary,
		},
		{
			title:  "Development",
			folder: "dev",
			regex:  `(.+)\.md`,
			note:   printExcludedNote,
		},
		{
			// One chapter per file under .github/workflows/, which .github/AGENTS.md requires
			// to exist and to stay current. The folder walk is what makes a new workflow's doc
			// reach the sidebar without a hand-written index.
			title:  "Workflows",
			folder: "workflows",
			regex:  `(.+)\.md`,
			note:   printExcludedNote,
		},
		{
			title:  "Agent",
			folder: "agent",
			regex:  `(.+)\.md`,
			note:   printExcludedNote,
		},
	}

	for i, item := range summary {
		md = md.H1(item.title)
		md = md.PlainText("")
		if item.note != "" {
			md = md.PlainText(item.note)
			md = md.PlainText("")
		}
		if item.extra != "" {
			md = md.PlainText(item.extra)
		}
		if item.lines != nil {
			// Unlike the folder walk below, this error is returned rather than swallowed: a
			// section that supplies its own entries has nothing to fall back to, and an empty
			// chapter is not something anyone would notice in the diff.
			mark, err := item.lines()
			if err != nil {
				return err
			}
			for _, p := range mark {
				md = md.PlainText(p)
			}
		}
		if item.regex != "" && item.folder != "" {
			if mark, err := getMarkdown(item.regex, item.folder); err == nil {
				for _, p := range mark {
					md = md.PlainText(p)
				}
			}
		}
		if i != len(summary)-1 {
			md = md.PlainText("\n-----------\n")
		} else {
			md = md.PlainText("")
		}
	}

	if err := md.Build(); err != nil {
		return err
	}

	return os.WriteFile("docs/SUMMARY.md", []byte(builder.String()), 0o644)
}

// getMarkdown lists one folder's pages as sidebar entries.
func getMarkdown(pattern string, directory string) ([]string, error) {
	list := []string{}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return []string{}, err
	}

	entries, err := os.ReadDir(fmt.Sprintf("docs/%s", directory))
	if err != nil {
		return []string{}, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && re.MatchString(entry.Name()) {
			com := re.FindStringSubmatch(entry.Name())
			list = append(list, fmt.Sprintf("- [%s](%s/%s)", com[1], directory, entry.Name()))
		}
	}

	return list, nil
}
