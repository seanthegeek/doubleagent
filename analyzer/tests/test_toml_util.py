"""Tests of `toml_util`: the TOML rules the parsers rely on `tomllib` for."""

from doubleagent.toml_util import load_toml
from fixtures import CODEX_AUTOMATION_TOML, CODEX_SESSION


def raises_value_error(text):
    """Return True when `load_toml(text)` raises a ValueError."""
    try:
        load_toml(text)
    except ValueError:
        return True
    return False


def test_load_toml_reads_automation_file():
    multi = CODEX_AUTOMATION_TOML.replace(
        'prompt = "Check the nightly build and \\"summarise\\" failures"',
        'prompt = """\nline one\\tand \\u00e9\nline two"""\nnote = \'C:\\path\' # x',
    )
    expected = {
        "version": 1,
        "id": "check-the-nightly-build",
        "kind": "heartbeat",
        "name": "Check the nightly build",
        "prompt": 'Check the nightly build and "summarise" failures',
        "status": "ACTIVE",
        "rrule": "RRULE:FREQ=WEEKLY;BYHOUR=9;BYMINUTE=0;BYDAY=MO,TU,WE,TH,FR",
        "target_thread_id": CODEX_SESSION,
        "created_at": 1791991549000,
        "updated_at": 1791991551000,
    }
    assert load_toml(CODEX_AUTOMATION_TOML) == expected
    got = load_toml(multi)
    assert got["prompt"] == "line one\tand \u00e9\nline two"
    assert got["note"] == "C:\\path"
    assert raises_value_error('name = """never closed\n')


def test_toml_line_ends_at_lf_or_crlf_only():
    # A U+2028 inside a string or a quoted key is part of it, not a line end.
    assert load_toml('prompt = "a\u2028b"\n') == {"prompt": "a\u2028b"}
    assert load_toml('[projects."/srv/a\u2028b"]\ntrust_level = "trusted"\n') == {
        "projects": {"/srv/a\u2028b": {"trust_level": "trusted"}}
    }
    with_lf = 'id = "x"\nprompt = """a\nb"""\n[projects."/srv/proj"]\ntrust_level = "trusted"\n'
    with_crlf = with_lf.replace("\n", "\r\n")
    assert load_toml(with_crlf) == load_toml(with_lf)
    assert load_toml(with_lf) == {
        "id": "x",
        "prompt": "a\nb",
        "projects": {"/srv/proj": {"trust_level": "trusted"}},
    }


def test_toml_trims_space_and_tab_only():
    # TOML whitespace is space and tab: a U+00A0 ending a multi-line string's first line
    # is the string's, and one beside a key or value is an error.
    assert load_toml('prompt = """x\u00a0\nend"""\n') == {"prompt": "x\u00a0\nend"}
    for text in (
        "k\u00a0= 1\n",
        "k = \u00a01\n",
        "flag = true\u00a0\n",
        '[a]\n\u00a0trust_level = "trusted"\n',
        "[a\u00a0.b]\nk = 1\n",
    ):
        assert raises_value_error(text), text
    # A line of U+00A0, or a key line led by one, is no blank and no key.
    for text in ("\u00a0\nk = 1\n", "\u00a0# note\nk = 1\n", "\u00a0k = 1\n"):
        assert raises_value_error(text), text
    # An integer is ASCII digits with no leading zero and no padding but space and tab.
    assert load_toml("a = 1_000\nb = -5\nc = 0\nd = 7 \t# c\n") == {
        "a": 1000,
        "b": -5,
        "c": 0,
        "d": 7,
    }
    for text in ("k = 1\u00a0\n", "k = 0012\n", "k = \u0661\n", "k = 1__0\n"):
        assert raises_value_error(text), text
    # Spaces before a multi-line string's first line end are the string's.
    assert load_toml('p = """a  \nb"""\nq = """\t\nc"""\n') == {"p": "a  \nb", "q": "\t\nc"}


def test_toml_line_ending_backslash():
    # The backslash removes spaces, tabs and line ends up to the next character, a
    # no-break space included in what stays; a backslash an earlier one escapes is a
    # backslash, not a continuation.
    cases = {
        'p = """a\\  \n  \n  b"""\n': "ab",
        'p = """a\\\n\u00a0b"""\n': "a\u00a0b",
        'p = """a\\\\\nb"""\n': "a\\\nb",
        'p = """a\\\\\\\n  b"""\n': "a\\b",
    }
    # A backslash and a space, a tab or a CR with no line end after it is a bad escape.
    for text in ('p = "a\\ b"\n', 'p = "a\\\tb"\n', 'p = "a\\\rb"\n', 'p = """a\\ b"""\n'):
        assert raises_value_error(text), text
    for text, want in cases.items():
        assert load_toml(text) == {"p": want}, text
