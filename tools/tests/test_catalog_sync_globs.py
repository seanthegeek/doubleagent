"""Tests for the table-glob check of collectors/tests/catalog-sync.sh (`--globs FILE`)."""

from __future__ import annotations

import pathlib
import shutil
import subprocess

import pytest

SCRIPT = pathlib.Path(__file__).resolve().parents[2] / "collectors" / "tests" / "catalog-sync.sh"
SHELLS = [shell for shell in ("sh", "dash", "bash") if shutil.which(shell)]


def run_check(
    shell: str, catalog_text: str, tmp_path: pathlib.Path
) -> subprocess.CompletedProcess[str]:
    """Run the glob check of one shell over a --list-format text."""
    catalog = tmp_path / "catalog.txt"
    catalog.write_text(catalog_text, encoding="utf-8")
    return subprocess.run(
        [shell, str(SCRIPT), "--globs", str(catalog)], capture_output=True, text=True, check=False
    )


@pytest.mark.parametrize("shell", SHELLS)
@pytest.mark.parametrize(
    ("line", "reason"),
    [
        ("agent|dir\\file", "a backslash or a caret"),
        ("agent|dir/[^a]*", "a backslash or a caret"),
        ("agent|dir/[[:alpha:]]*", "character class name"),
        ("agent|dir/[[.hyphen.]]x", "collating class"),
        ("agent|dir/[[=a=]]x", "equivalence class"),
        ("agent|dir/[abc", "an unclosed bracket"),
        ("agent|dir/[!]", "an unclosed bracket"),
        ("agent|dir/[]", "an unclosed bracket"),
        ("tmp/playwright/[a-", "an unclosed bracket"),
    ],
)
def test_a_glob_the_shells_read_differently_fails(
    shell: str, line: str, reason: str, tmp_path: pathlib.Path
) -> None:
    """Each forbidden form fails the check and is named with its reason."""
    result = run_check(shell, f"# home artifacts\nagent|.fine\n{line}\n", tmp_path)
    assert result.returncode == 1
    assert line in result.stdout
    assert reason in result.stdout
    assert "agent|.fine" not in result.stdout


@pytest.mark.parametrize("shell", SHELLS)
def test_closed_bracket_lists_and_comments_pass(shell: str, tmp_path: pathlib.Path) -> None:
    """A closed list, a ] first in a list, a range and a comment holding [ pass."""
    text = (
        "# a comment with [ and ^ and \\\n"
        "\n"
        "agent|[Cc]rew[Aa][Ii]\n"
        "agent|dir/[]a]x\n"
        "agent|dir/[!]a]x\n"
        "agent|diagnostics_[0-9]*_*.json\n"
        "hermes-agent/[!.]*\n"
    )
    result = run_check(shell, text, tmp_path)
    assert result.returncode == 0
    assert result.stdout.startswith("ok")
