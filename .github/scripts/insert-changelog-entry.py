#!/usr/bin/env python3
"""Insert a bullet into the [Unreleased] -> ### Changed section of CHANGELOG.md.

Used by .github/workflows/dependency-version-check.yml to record dependency
bumps without rewriting the file's structure. The entry becomes the first
bullet under ### Changed; a missing section is created.
"""

import sys
from pathlib import Path


def main() -> None:
    if len(sys.argv) != 2:
        sys.exit("usage: insert-changelog-entry.py ENTRY")
    entry = sys.argv[1] + "\n"

    changelog = Path("CHANGELOG.md")
    lines = changelog.read_text().splitlines(keepends=True)

    unre = next(
        (i for i, line in enumerate(lines) if line.rstrip() == "## [Unreleased]"),
        None,
    )
    if unre is None:
        sys.exit("CHANGELOG.md has no '## [Unreleased]' section")

    changed = None
    for i in range(unre + 1, len(lines)):
        if lines[i].startswith("## "):
            break
        if lines[i].strip() == "### Changed":
            changed = i
            break

    if changed is not None:
        lines.insert(changed + 1, entry)
    else:
        pos = unre + 1
        if pos < len(lines) and not lines[pos].strip():
            pos += 1
        blocked = "### Changed\n" + entry
        if pos == len(lines) or lines[pos].startswith("## "):
            blocked += "\n"
        lines.insert(pos, blocked)

    changelog.write_text("".join(lines))


if __name__ == "__main__":
    main()
