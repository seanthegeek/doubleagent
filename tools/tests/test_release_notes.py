"""Tests for .github/release-notes.sh, which writes a release's notes from the changelog."""

from __future__ import annotations

import pathlib
import subprocess

import pytest

SCRIPT = pathlib.Path(__file__).resolve().parents[2] / ".github" / "release-notes.sh"
BASE = "https://example.test/blob/v1/docs/"

CHANGELOG = """\
# Changelog

## [Unreleased]

- Pending work.

## [0.1.01] - 2026-11-03

- Leading zero, see [the options](collect-options.md#exit-codes).

## [0.1.0-rc.1] - 2026-11-02

- Candidate, see [an issue] ([#12]) and [the site](https://example.test/x).

## [0.1.0] - 2026-11-01

- First, see [above](#unreleased) and [testing][rel].

## [0.0.9]

- Undated.

## [0.0.8] - 2026-10-01

## [0.0.7] - 2026-09-01

- Last.

[0.1.0]: https://example.test/compare/v0.0.9...v0.1.0
[rel]: testing.md#what-ci-runs
[#12]: https://example.test/issues/12
"""

DEFINITIONS = (
    "[0.1.0]: https://example.test/compare/v0.0.9...v0.1.0\n"
    "[rel]: " + BASE + "testing.md#what-ci-runs\n"
    "[#12]: https://example.test/issues/12\n"
)


def notes(tmp_path: pathlib.Path, version: str) -> subprocess.CompletedProcess[str]:
    """Run the script for `version` over the test changelog and return the finished process."""
    changelog = tmp_path / "CHANGELOG.md"
    changelog.write_text(CHANGELOG, encoding="utf-8")
    return subprocess.run(
        ["sh", str(SCRIPT), version, str(changelog), BASE],
        capture_output=True,
        text=True,
        check=False,
    )


@pytest.mark.parametrize(
    ("version", "body"),
    [
        ("0.1.0", "- First, see [above](#unreleased) and [testing][rel].\n"),
        (
            "0.1.0-rc.1",
            "- Candidate, see [an issue] ([#12]) and [the site](https://example.test/x).\n",
        ),
        (
            "0.1.01",
            "- Leading zero, see [the options](" + BASE + "collect-options.md#exit-codes).\n",
        ),
        ("0.0.7", "- Last.\n"),
    ],
    ids=["a release", "a pre-release", "a version with a leading zero", "the last section"],
)
def test_each_version_gets_its_own_section_and_every_definition(
    tmp_path: pathlib.Path, version: str, body: str
) -> None:
    done = notes(tmp_path, version)
    assert (done.returncode, done.stderr) == (0, "")
    assert done.stdout == body + "\n" + DEFINITIONS


@pytest.mark.parametrize(
    ("version", "message"),
    [
        ("", "usage: release-notes.sh"),
        ("v0.1.0", "is not a version such as 1.2.3"),
        ("0.1.0 x", "is not a version such as 1.2.3"),
        ("0.0.9", 'has no "## [0.0.9] - YYYY-MM-DD" section'),
        ("0.2.0", 'has no "## [0.2.0] - YYYY-MM-DD" section'),
        ("0.0.8", "the [0.0.8] section of"),
    ],
    ids=["no version", "a leading v", "a trailing word", "undated", "missing", "empty"],
)
def test_a_version_without_notes_is_refused(
    tmp_path: pathlib.Path, version: str, message: str
) -> None:
    done = notes(tmp_path, version)
    assert done.returncode != 0
    assert done.stdout == ""
    assert message in done.stderr
