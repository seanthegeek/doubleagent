"""Tests of lib/screen.sh, the readers of a rendered agent screen, on a saved screen file."""

from __future__ import annotations

import pathlib
import subprocess

ROUND = pathlib.Path(__file__).resolve().parent.parent
# A synthetic screen 60 columns wide: every full-width line is exactly 60 columns.
SCREEN = ROUND / "tests" / "screens" / "wrapped-url-60.txt"
WIDTH = 60
SHELLS = [shell for shell in ("/bin/sh", "/bin/dash") if pathlib.Path(shell).is_file()]


def run_reader(shell: str, call: str, screen: pathlib.Path = SCREEN) -> str:
    """Return what a screen.sh function prints for the screen file, run under `shell`."""
    script = '. "$1"; %s <"$2"' % call
    result = subprocess.run(
        [shell, "-c", script, "screen-test", str(ROUND / "lib" / "screen.sh"), str(screen)],
        capture_output=True,
        check=True,
        text=True,
        encoding="utf-8",
    )
    return result.stdout


def joined(shell: str) -> list[str]:
    """Return the URLs joined_urls prints for the fixture at its width."""
    return run_reader(shell, "joined_urls %d" % WIDTH).split("\n")[:-1]


def test_fixture_lines_are_full_width():
    lines = SCREEN.read_text(encoding="utf-8").split("\n")
    assert len(lines[0]) == WIDTH and len(lines[1]) == WIDTH and len(lines[2]) == WIDTH


def test_a_url_over_three_full_lines_keeps_percent_and_hash_continuations():
    expected = "https://a.example.com/path?x=%s%%2F%s#%stail-a" % ("1" * 26, "2" * 57, "3" * 59)
    for shell in SHELLS:
        assert joined(shell)[0] == expected, shell


def test_a_wrapped_tail_followed_by_text_keeps_the_tail():
    for shell in SHELLS:
        assert joined(shell)[1] == "https://b.example.com/%stail-b" % ("b" * 32), shell


def test_a_prose_word_after_a_full_width_url_line_is_joined():
    for shell in SHELLS:
        assert joined(shell)[2] == "https://c.example.com/%sEnter" % ("c" * 38), shell


def test_a_url_ending_a_short_line_is_complete():
    for shell in SHELLS:
        assert joined(shell)[3] == "https://d.example.com/docs", shell


def test_a_url_wrapped_inside_a_box_is_joined():
    expected = "https://e.example.com/auth?id=lab&type=c%spe=code&state=synthetic" % ("e" * 16)
    for shell in SHELLS:
        found = joined(shell)
        assert found[4] == expected and len(found) == 5, shell


def test_joined_urls_print_nothing_without_a_url(tmp_path):
    screen = tmp_path / "plain.txt"
    screen.write_text("> Type your message\n\nno address here\n", encoding="utf-8")
    for shell in SHELLS:
        assert run_reader(shell, "joined_urls 60", screen) == "", shell


def test_highlighted_line_is_the_first_marked_option():
    for shell in SHELLS:
        found = run_reader(shell, "highlighted_line '^ *● '")
        assert found == "● 1. Allow once\n", shell


def test_strip_box_removes_the_borders():
    for shell in SHELLS:
        lines = run_reader(shell, "strip_box").split("\n")
        assert lines[-9] == "" and lines[-8] == "Sign in to the example service", shell
