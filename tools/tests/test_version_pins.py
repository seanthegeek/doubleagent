"""Tests that the linter versions the CI workflow, the Makefile and AGENTS.md pin agree."""

from __future__ import annotations

import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[2]

# Each Go linter: its variable in the workflow's env block and the Makefile, and its row in the
# "Quality gates" table of AGENTS.md.
GO_LINTERS = {"STATICCHECK_VERSION": "staticcheck", "GOVULNCHECK_VERSION": "govulncheck"}


def assignments(path: pathlib.Path, form: str) -> dict[str, str]:
    """Return every `NAME` and value `form` (a regex with groups name and value) finds in path."""
    text = path.read_text(encoding="utf-8")
    return {match["name"]: match["value"] for match in re.finditer(form, text, re.MULTILINE)}


def gates_table_versions() -> dict[str, str]:
    """Return the Version cell of each row of the AGENTS.md table, keyed by its Gate cell."""
    rows = {}
    for line in (ROOT / "AGENTS.md").read_text(encoding="utf-8").splitlines():
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if line.startswith("|") and len(cells) >= 2:
            rows[cells[0]] = cells[1]
    return rows


def test_the_go_linter_versions_agree_in_the_workflow_the_makefile_and_agents_md() -> None:
    workflow = assignments(
        ROOT / ".github/workflows/ci.yml", r'^  (?P<name>[A-Z_]+_VERSION): "(?P<value>[^"]+)"$'
    )
    makefile = assignments(ROOT / "Makefile", r"^(?P<name>[A-Z_]+_VERSION) = (?P<value>\S+)$")
    table = gates_table_versions()
    for variable, gate in GO_LINTERS.items():
        pins = (workflow.get(variable), makefile.get(variable), table.get(gate))
        assert pins[0] is not None and len(set(pins)) == 1, (
            "%s: ci.yml %s, Makefile %s, AGENTS.md %s" % (gate, *pins)
        )
