"""Tests for the table-glob check of collectors/tests/catalog-sync.sh (`--globs FILE`)."""

from __future__ import annotations

import pathlib
import shutil
import subprocess
import tempfile

SCRIPT = pathlib.Path(__file__).resolve().parents[2] / "collectors" / "tests" / "catalog-sync.sh"
SHELLS = [shell for shell in ("sh", "dash", "bash") if shutil.which(shell)]


def run_check(shell: str, catalog_text: str) -> subprocess.CompletedProcess[str]:
    """Run the glob check of one shell over a --list-format text."""
    with tempfile.TemporaryDirectory() as directory:
        catalog = pathlib.Path(directory) / "catalog.txt"
        catalog.write_text(catalog_text, encoding="utf-8")
        return subprocess.run(
            [shell, str(SCRIPT), "--globs", str(catalog)],
            capture_output=True,
            text=True,
            check=False,
        )


FORBIDDEN_GLOBS = [
    ("agent|dir\\file", "a backslash or a caret"),
    ("agent|dir/[^a]*", "a backslash or a caret"),
    ("agent|dir/[[:alpha:]]*", "character class name"),
    ("agent|dir/[[.hyphen.]]x", "collating class"),
    ("agent|dir/[[=a=]]x", "equivalence class"),
    ("agent|dir/[abc", "an unclosed bracket"),
    ("agent|dir/[!]", "an unclosed bracket"),
    ("agent|dir/[]", "an unclosed bracket"),
    ("tmp/playwright/[a-", "an unclosed bracket"),
]


def test_a_glob_the_shells_read_differently_fails() -> None:
    """Each forbidden form fails the check and is named with its reason."""
    for shell in SHELLS:
        for line, reason in FORBIDDEN_GLOBS:
            result = run_check(shell, f"# home artifacts\nagent|.fine\n{line}\n")
            context = f"{shell}: {line}"
            assert result.returncode == 1, context
            assert line in result.stdout, context
            assert reason in result.stdout, context
            assert "agent|.fine" not in result.stdout, context


def test_closed_bracket_lists_and_comments_pass() -> None:
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
    for shell in SHELLS:
        result = run_check(shell, text)
        assert result.returncode == 0, shell
        assert result.stdout.startswith("ok"), shell
