#!/usr/bin/env python3
# Copyright 2026 colonel-byte
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Repad GFM tables into the aligned form ansible/AGENTS.md and docs/AGENTS.md require.

Every cell is padded to the width of the widest cell in its column, with one space of padding
inside each pipe and the delimiter row's dashes run out to the same width, so every line of the
table has the same length. This is the same shape `paddedTable` in magefiles/pkg/gen/docs produces
for the generated pages; this script holds the hand-written pages to it.

Tables inside fenced code blocks are left alone: docs/AGENTS.md and ansible/AGENTS.md document the
rule with a deliberately unaligned "not this" example, and repadding it would delete the example
the page is built around. `--self-check` asserts exactly that, and runs as its own pre-commit hook
so the guarantee is tested rather than assumed.

shortcut: only fenced code blocks are recognised, not the four-space-indented kind. No Markdown in
this tree uses one, and a table written inside one would be repadded. Teach reformat() the indented
form if one ever appears.

Usage: markdown-tables.py FILE...     rewrite in place, exit 1 if anything was not aligned
       markdown-tables.py --self-check  assert a fenced example survives a run
"""

import re
import sys

FENCE = re.compile(r"^\s*(`{3,}|~{3,})")
DELIMITER_CELL = re.compile(r"^:?-+:?$")


def split_cells(line):
    """Split a table row on its unescaped, non-code-span pipes."""
    cells, cell, in_code, i = [], [], False, 0
    while i < len(line):
        c = line[i]
        if c == "\\" and i + 1 < len(line):
            cell.append(line[i : i + 2])
            i += 2
            continue
        if c == "`":
            in_code = not in_code
        if c == "|" and not in_code:
            cells.append("".join(cell))
            cell = []
        else:
            cell.append(c)
        i += 1
    cells.append("".join(cell))
    # A row is written with a leading and trailing pipe, which bracket the cells with two empties.
    if cells and not cells[0].strip():
        cells.pop(0)
    if cells and not cells[-1].strip():
        cells.pop()
    return [c.strip() for c in cells]


def is_delimiter(cells):
    return bool(cells) and all(DELIMITER_CELL.match(c) for c in cells)


def render(block, indent):
    rows = [split_cells(line) for line in block]
    columns = max(len(r) for r in rows)
    rows = [r + [""] * (columns - len(r)) for r in rows]

    width = []
    for i in range(columns):
        cells = [r[i] for j, r in enumerate(rows) if j != 1]
        colons = rows[1][i].count(":")
        width.append(max([3, colons + 1] + [len(c) for c in cells]))

    out = []
    for j, row in enumerate(rows):
        if j == 1:
            cells = []
            for i, cell in enumerate(row):
                left, right = cell.startswith(":"), cell.endswith(":")
                dashes = "-" * (width[i] - left - right)
                cells.append(":" * left + dashes + ":" * right)
        else:
            cells = [cell.ljust(width[i]) for i, cell in enumerate(row)]
        out.append(indent + "| " + " | ".join(cells) + " |")
    return out


def reformat(text):
    lines = text.split("\n")
    out, fence, i = [], None, 0
    while i < len(lines):
        line = lines[i]
        marker = FENCE.match(line)
        if fence:
            if marker and marker.group(1)[0] == fence[0] and len(marker.group(1)) >= len(fence):
                fence = None
            out.append(line)
            i += 1
            continue
        if marker:
            fence = marker.group(1)
            out.append(line)
            i += 1
            continue

        if line.lstrip().startswith("|"):
            block = []
            while i < len(lines) and lines[i].lstrip().startswith("|"):
                block.append(lines[i])
                i += 1
            if len(block) >= 2 and is_delimiter(split_cells(block[1])):
                out.extend(render(block, block[0][: len(block[0]) - len(block[0].lstrip())]))
            else:
                out.extend(block)
            continue

        out.append(line)
        i += 1
    return "\n".join(out)


# The shape docs/AGENTS.md and ansible/AGENTS.md are written in: an unaligned table inside a fence,
# which is the example, and an aligned one outside it, which is the rule. A run must repad the
# second and leave the first byte for byte -- including inside the four-backtick fence the
# release-please section uses to show a fence.
SELF_CHECK = """## Pad table cells so the columns line up

Not this:

```markdown
| Path | What it holds |
| --- | --- |
| `.spec.config.registries[N].auth.user` | Registry username |
```

````markdown
| a | b |
| --- | --- |
````

| Path | What it holds |
| --- | --- |
| `.spec.config.registries[N].auth.user` | Registry username |
"""


def self_check():
    out = reformat(SELF_CHECK)
    fenced = SELF_CHECK[SELF_CHECK.index("```markdown") : SELF_CHECK.rindex("````markdown")]
    assert fenced in out, "the fenced example was repadded"
    assert "| a | b |" in out, "the four-backtick fenced example was repadded"
    assert "| Path | What it holds |\n| --- | --- |\n| `.spec" not in out.replace(fenced, ""), (
        "the table outside the fence was left unaligned"
    )
    assert reformat(out) == out, "a second run changed the file again"
    print("self-check passed")
    return 0


def main(paths):
    if paths == ["--self-check"]:
        return self_check()

    changed = []
    for path in paths:
        with open(path, encoding="utf-8") as f:
            text = f.read()
        formatted = reformat(text)
        if formatted != text:
            with open(path, "w", encoding="utf-8") as f:
                f.write(formatted)
            changed.append(path)
    for path in changed:
        print(f"repadded tables in {path}")
    return 1 if changed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
