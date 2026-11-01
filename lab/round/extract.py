#!/usr/bin/env python3
"""Extract a pulled collection archive through the analyzer's safe reader.

    PYTHONPATH=analyzer $ROUND_PYTHON lab/round/extract.py ARCHIVE DEST

The archive is built in the VM from directories the agent under test controls, so it is never
unpacked with tar or unzip: `doubleagent.inputs.extract_archive` creates no link or device member
and refuses or drops a member with an absolute path, a `..` component or a drive letter. Exits 1
with the reason on stderr when the archive cannot be extracted.
"""

from __future__ import annotations

import sys
from pathlib import Path

from doubleagent.errors import InputError
from doubleagent.inputs import extract_archive


def main(arguments: list[str]) -> int:
    """Run the command line; return the exit status."""
    if len(arguments) != 2:
        print("usage: extract.py ARCHIVE DEST", file=sys.stderr)
        return 2
    destination = Path(arguments[1])
    destination.mkdir(parents=True, exist_ok=True)
    try:
        extract_archive(Path(arguments[0]), destination)
    except (InputError, OSError) as error:
        print("extract.py: %s" % error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
