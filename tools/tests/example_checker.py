"""Check that every research document's H1 names its agent: the form every drift checker takes.

A drift checker lives at `tools/check_<name>.py`, states each rule as one sentence here, and
yields one `Finding` per disagreement from `run(root)`; `main_with_baseline` does the rest.
This example is the copy to start from; the tests run it on a synthetic repository.

- `title-mismatch`: a research document whose first heading is not `# <file stem>`.

A real checker's usage line reads `Usage: tools/check_<name>.py [--update-baseline]`.
"""

from __future__ import annotations

import pathlib
import sys
from collections.abc import Iterator

import drift_common

CHECKER = "check_example"


def run(root: pathlib.Path) -> Iterator[drift_common.Finding]:
    """Yield a `title-mismatch` finding per research document whose H1 is not its stem."""
    for agent, path in drift_common.agent_documents(root).items():
        sections = drift_common.research_sections(path)
        intro = sections.get("")
        title = intro.prose[0] if intro and intro.prose else (1, "")
        if title[1] != "# %s" % agent:
            relative = path.relative_to(root).as_posix()
            yield drift_common.Finding(agent, "title-mismatch", path.name, relative, title[0])


def main(arguments: list[str] | None = None, root: pathlib.Path | None = None) -> int:
    """Run the check against the baseline and return the exit status."""
    summary = (__doc__ or "").split("\n")[0]
    return drift_common.main_with_baseline(CHECKER, run, arguments, root, summary)


if __name__ == "__main__":
    sys.exit(main())
