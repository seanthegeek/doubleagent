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
    return run_over(tmp_path, CHANGELOG, version)


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


FENCED_CHANGELOG = """\
# Changelog

## [0.2.0] - 2026-11-05

- Before the fence.

```text
## [0.1.0] - 2026-11-01
[inside](inside.md)
[def]: inside.md
```

~~~~
## [9.9.9] - 2026-01-01
~~~
still inside
~~~~

- After the fence.

## [0.1.0] - 2026-11-01

- Old.
"""


def run_over(tmp_path: pathlib.Path, text: str, version: str) -> subprocess.CompletedProcess[str]:
    """Run the script for `version` over the changelog `text` and return the finished process."""
    changelog = tmp_path / "CHANGELOG.md"
    changelog.write_text(text, encoding="utf-8")
    return subprocess.run(
        ["sh", str(SCRIPT), version, str(changelog), BASE],
        capture_output=True,
        text=True,
        check=False,
    )


def test_a_heading_inside_a_fenced_block_neither_ends_nor_starts_a_section(
    tmp_path: pathlib.Path,
) -> None:
    done = run_over(tmp_path, FENCED_CHANGELOG, "0.2.0")
    assert (done.returncode, done.stderr) == (0, "")
    start = FENCED_CHANGELOG.index("- Before the fence.")
    end = FENCED_CHANGELOG.index("- After the fence.\n") + len("- After the fence.\n")
    assert done.stdout == FENCED_CHANGELOG[start:end]
    assert run_over(tmp_path, FENCED_CHANGELOG, "9.9.9").returncode != 0
    old = run_over(tmp_path, FENCED_CHANGELOG, "0.1.0")
    assert (old.returncode, old.stdout) == (0, "- Old.\n")


@pytest.mark.parametrize(
    ("line", "want"),
    [
        ("[a](notes.md)", "[a](" + BASE + "notes.md)"),
        ("[a](/root.md)", "[a](/root.md)"),
        ("[a](//host/x)", "[a](//host/x)"),
        ("[a](#anchor)", "[a](#anchor)"),
        ("[a](<spaced name.md>)", "[a](<" + BASE + "spaced name.md>)"),
        ("[a](<#anchor>)", "[a](<#anchor>)"),
        ("[a](<https://example.test/a b>)", "[a](<https://example.test/a b>)"),
        ("[a](<unclosed.md)", "[a](<unclosed.md)"),
        ("[a](HTTPS://example.test/)", "[a](HTTPS://example.test/)"),
        ("`[a](notes.md)` and [b](b.md)", "`[a](notes.md)` and [b](" + BASE + "b.md)"),
        ("``x ` [a](notes.md)``", "``x ` [a](notes.md)``"),
        ("`unclosed [a](notes.md)", "`unclosed [a](" + BASE + "notes.md)"),
    ],
    ids=[
        "a relative target",
        "a root-relative target",
        "a scheme-relative target",
        "a fragment",
        "an angle-bracket target",
        "an angle-bracket fragment",
        "an angle-bracket target with a scheme",
        "an unclosed angle bracket",
        "an upper-case scheme",
        "a code span beside a link",
        "a code span of two backticks holding one",
        "an unclosed backtick",
    ],
)
def test_only_a_relative_target_outside_code_is_made_absolute(
    tmp_path: pathlib.Path, line: str, want: str
) -> None:
    done = run_over(tmp_path, "## [1.0.0] - 2026-11-05\n\n" + line + "\n", "1.0.0")
    assert (done.returncode, done.stderr) == (0, "")
    assert done.stdout == want + "\n"


COMMENTED_CHANGELOG = """\
# Changelog

## [0.2.0] - 2026-11-05

- Before the comment.

<!--
## [0.1.0] - 2026-11-01
## [9.9.9] - 2026-01-01
[inside](inside.md)
[def]: inside.md
-->
  <!-- one line, [kept](kept.md) -->

- After the comment, [rel](rel.md).

## [0.1.0] - 2026-11-01

- Old.

[out]: <spaced name.md>
[far]: <https://example.test/x>
"""


def test_a_heading_inside_an_html_comment_neither_ends_nor_starts_a_section(
    tmp_path: pathlib.Path,
) -> None:
    done = run_over(tmp_path, COMMENTED_CHANGELOG, "0.2.0")
    assert (done.returncode, done.stderr) == (0, "")
    start = COMMENTED_CHANGELOG.index("- Before the comment.")
    end = COMMENTED_CHANGELOG.index("- After the comment,")
    body = COMMENTED_CHANGELOG[start:end] + "- After the comment, [rel](" + BASE + "rel.md).\n"
    definitions = "[out]: <" + BASE + "spaced name.md>\n[far]: <https://example.test/x>\n"
    assert done.stdout == body + "\n" + definitions
    assert run_over(tmp_path, COMMENTED_CHANGELOG, "9.9.9").returncode != 0
    old = run_over(tmp_path, COMMENTED_CHANGELOG, "0.1.0")
    assert (old.returncode, old.stdout) == (0, "- Old.\n\n" + definitions)
