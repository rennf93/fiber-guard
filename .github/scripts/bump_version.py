"""
Version bump helper script for fiber-guard.

fiber-guard is a Go module: there is no version file to update, the git
tag is the version. This script only inserts a version scaffold block at the
top of CHANGELOG.md so the release entry follows the house format; fill in
the TITLE and CONTENT placeholders (and the section headings) after running
it. The version itself is applied with `git tag vX.Y.Z` at release time.

Usage:
    python3 .github/scripts/bump_version.py <version>
    make bump-version VERSION=x.y.z

No external dependencies required, stdlib only.
"""

from __future__ import annotations

import re
import sys
from datetime import datetime, timezone
from pathlib import Path

# Resolve project root relative to this script's location
PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent

VERSION_PATTERN = re.compile(r"^\d+\.\d+\.\d+$")


def _insert_changelog_scaffold(path: Path, version: str, label: str) -> bool:
    """Insert a version scaffold block into a changelog file."""
    if not path.exists():
        print(f"  ERROR: Could not find {path.relative_to(PROJECT_ROOT)}")
        return False
    content = path.read_text()
    today = datetime.now(tz=timezone.utc).strftime("%Y-%m-%d")
    header = f"v{version} ({today})"

    # Check if this version already has an entry
    if f"v{version} (" in content:
        print(f"  {label}: v{version} entry already exists")
        return True

    scaffold = (
        f"{header}\n"
        f"-------------------\n"
        f"\n"
        f"TITLE (v{version})\n"
        f"------------\n"
        f"\n"
        f"CONTENT\n"
        f"\n"
        f"___\n"
        f"\n"
    )

    # Find the first existing version entry to insert before it
    version_header_pattern = re.compile(r"^v\d+\.\d+\.\d+ \(", re.MULTILINE)
    match = version_header_pattern.search(content)
    if match:
        insert_pos = match.start()
        new_content = content[:insert_pos] + scaffold + content[insert_pos:]
    else:
        # No existing entries, append at end
        new_content = content.rstrip() + "\n\n" + scaffold

    path.write_text(new_content)
    print(f"  {label}: added v{version} scaffold")
    return True


def update_changelog(version: str) -> bool:
    """Update CHANGELOG.md."""
    changelog = PROJECT_ROOT / "CHANGELOG.md"
    return _insert_changelog_scaffold(changelog, version, "CHANGELOG.md")


def main() -> int:
    if len(sys.argv) != 2:
        print("Usage: bump_version.py <version>")
        print("  version must be in X.Y.Z format")
        return 1

    version = sys.argv[1]

    if not VERSION_PATTERN.match(version):
        print(f"Error: '{version}' is not a valid version. Expected format: X.Y.Z")
        return 1

    print(f"Bumping version to {version}...\n")
    print(
        "  Note: fiber-guard has no version file; the git tag is the version.\n"
        "  This only scaffolds CHANGELOG.md.\n"
    )

    all_ok = update_changelog(version)

    print()
    if all_ok:
        print("Version bump complete.")
    else:
        print("Version bump completed with errors.")
    return 0 if all_ok else 1


if __name__ == "__main__":
    sys.exit(main())
