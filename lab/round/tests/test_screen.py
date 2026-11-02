"""Tests of lib/screen.sh, the readers of a rendered agent screen, on a saved screen file."""

from __future__ import annotations

import pathlib
import shutil
import subprocess

ROUND = pathlib.Path(__file__).resolve().parent.parent
SCREEN = ROUND / "tests" / "screens" / "wrapped-url.txt"
SHELLS = [shell for shell in ("/bin/sh", "/bin/dash", shutil.which("dash")) if shell]


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


def available_shells() -> list[str]:
    """Return the shells of SHELLS present on this machine, each once."""
    found: list[str] = []
    for shell in SHELLS:
        if pathlib.Path(shell).is_file() and shell not in found:
            found.append(shell)
    return found


def test_joined_urls_rejoin_a_wrapped_url():
    expected = [
        "https://login.example.com/o/auth?client_id=lab-client&response_type=code&state=synthetic",
        "https://example.org/docs",
        "http://example.net/first/part",
    ]
    for shell in available_shells():
        assert run_reader(shell, "joined_urls").split("\n")[:-1] == expected, shell


def test_joined_urls_print_nothing_without_a_url(tmp_path):
    screen = tmp_path / "plain.txt"
    screen.write_text("> Type your message\n\nno address here\n", encoding="utf-8")
    for shell in available_shells():
        assert run_reader(shell, "joined_urls", screen) == "", shell


def test_highlighted_line_is_the_first_marked_option():
    for shell in available_shells():
        found = run_reader(shell, "highlighted_line '^ *● '")
        assert found == "● 1. Allow once\n", shell


def test_strip_box_removes_the_borders():
    for shell in available_shells():
        lines = run_reader(shell, "strip_box").split("\n")
        assert lines[0] == "" and lines[1] == "Sign in to the example service", shell
