"""Tests of classify.py, the listing diff of a round step."""

from __future__ import annotations

import classify

BEFORE = [
    "dir\t4096\t755\t2026-11-01T10:00:00.0000000000Z\t/home/sean/.gemini\n",
    "file\t10\t644\t2026-11-01T10:00:00.0000000000Z\t/home/sean/.gemini/settings.json\n",
    "file\t5\t600\t2026-11-01T10:00:00.0000000000Z\t/home/sean/.gemini/old.json\n",
    "top\tdir\t4096\t755\t2026-11-01T10:00:00.0000000000Z\t/home/sean/.gemini\n",
    "keychain\tabsent\t-\t-\tgemini-cli-oauth\n",
    "process\t0\t-\t-\tgemini\n",
]
SESSION = "/home/sean/.gemini/tmp/proj/chats/session-2026-11-01T10-05-0123abcd.jsonl"
AFTER = [
    "dir\t4096\t700\t2026-11-01T10:05:00.0000000000Z\t/home/sean/.gemini\n",
    "file\t12\t644\t2026-11-01T10:05:00.0000000000Z\t/home/sean/.gemini/settings.json\n",
    "file\t900\t644\t2026-11-01T10:05:00.0000000000Z\t%s\n" % SESSION,
    "top\tdir\t4096\t755\t2026-11-01T10:00:00.0000000000Z\t/home/sean/.gemini\n",
    "top\tdir\t4096\t755\t2026-11-01T10:05:00.0000000000Z\t/home/sean/.npm\n",
    "keychain\tpresent\t-\t-\tgemini-cli-oauth\n",
    "process\t0\t-\t-\tgemini\n",
]


def difference() -> classify.Difference:
    return classify.compare(classify.parse_listing(BEFORE), classify.parse_listing(AFTER))


def test_added_removed_changed():
    found = difference()
    assert found.added == [SESSION]
    assert found.removed == ["/home/sean/.gemini/old.json"]
    assert found.changed == ["/home/sean/.gemini", "/home/sean/.gemini/settings.json"]


def test_directory_changes_only_by_mode():
    before = classify.parse_listing(["dir\t4096\t755\t2026-11-01T10:00:00Z\t/Users/sean/.gemini\n"])
    after = classify.parse_listing(["dir\t8192\t755\t2026-11-01T10:05:00Z\t/Users/sean/.gemini\n"])
    assert classify.compare(before, after).changed == []


def test_top_and_state_lines():
    found = difference()
    assert found.new_tops == ["/home/sean/.npm"]
    assert found.gone_tops == []
    assert found.states == [("keychain", "gemini-cli-oauth", "absent", "present")]


def test_group_key_under_home_and_root():
    cases = [
        ("/home/sean/.gemini/tmp/proj/logs.json", "~/.gemini/tmp"),
        ("/Users/sean/Library/Application Support/x", "~/Library/Application Support"),
        ("C:\\Users\\sean\\.gemini\\settings.json", "~/.gemini/settings.json"),
        ("/tmp/gemini/ide/server.json", "/tmp/gemini"),
        ("/home/sean/.local/bin/qwen -> /home/sean/.local/lib/qwen", "~/.local/bin"),
    ]
    for path, expected in cases:
        assert classify.group_key(path) == expected, path


def test_redact_uuid_and_hex_run():
    path = "/x/1f0e9a2c-3b4d-4e5f-8a9b-0c1d2e3f4a5b/%s.json" % ("ab" * 16)
    assert classify.redact(path) == "/x/<uuid>/<hex>.json"
    assert classify.redact("/x/session-0123abcd.jsonl") == "/x/session-0123abcd.jsonl"


def test_render_redacts_only_when_asked():
    before = classify.parse_listing(BEFORE)
    after = classify.parse_listing(
        [*AFTER, "file\t1\t600\tt\t/home/sean/.gemini/1f0e9a2c-3b4d-4e5f-8a9b-0c1d2e3f4a5b.json\n"]
    )
    found = classify.compare(before, after)
    assert "<uuid>" in classify.render(found, before, after, True)
    assert "<uuid>" not in classify.render(found, before, after, False)


def test_render_no_change():
    listing = classify.parse_listing(BEFORE)
    found = classify.compare(listing, listing)
    assert classify.render(found, listing, listing, True) == "no change\n"


def test_malformed_lines_are_skipped():
    listing = classify.parse_listing(["garbage\n", "a\tb\n", "\n"])
    assert listing.entries == {} and listing.tops == set() and listing.states == {}


def test_human_size():
    cases = [
        ("0", "0B"),
        ("1023", "1023B"),
        ("2048", "2.0KB"),
        ("-", "-"),
        (str(3 * 1024**3), "3.0GB"),
    ]
    for size, expected in cases:
        assert classify.human_size(size) == expected, size


ROOTS = """# synthetic roots
~/.gemini
<tmp>/gemini-client-error-*.json
linux:~/.local/share/keyrings
windows:%APPDATA%/example-agent
keychain:gemini-cli-oauth
process:gemini
"""
TOP_BEFORE = [
    "base\ttmp\t/tmp\n",
    "top\tdir\t4096\t755\tt\t/home/sean/.config\n",
    "top\tdir\t4096\t755\tt\t/home/sean/.local/share/applications\n",
    "top\tdir\t4096\t755\tt\t/tmp/systemd-private\n",
]
TOP_AFTER = [
    *TOP_BEFORE,
    "top\tfile\t300\t644\tt\t/tmp/gemini-client-error-Turn.run-2026-11-01T10-00-00.json\n",
    "top\tfile\t300\t644\tt\t/tmp/other-error.json\n",
    "top\tdir\t4096\t700\tt\t/home/sean/.local/share/keyrings\n",
    "top\tdir\t4096\t755\tt\t/home/sean/.npm\n",
    "top\tdir\t4096\t755\tt\t/home/sean/.gemini\n",
]


def test_roots_leave_covered_top_names_out():
    before = classify.parse_listing(TOP_BEFORE)
    after = classify.parse_listing(TOP_AFTER)
    found = classify.drop_covered_tops(classify.compare(before, after), after, ROOTS)
    assert found.new_tops == ["/home/sean/.npm", "/tmp/other-error.json"]
    text = classify.render(found, before, after, True)
    assert "gemini-client-error" not in text and "+ /home/sean/.npm" in text


def test_without_roots_every_new_top_name_shows():
    before = classify.parse_listing(TOP_BEFORE)
    after = classify.parse_listing(TOP_AFTER)
    assert len(classify.compare(before, after).new_tops) == 5


def test_listing_home_finds_the_home_past_other_paths():
    cases = [
        (["/var/folders/ab/T/tmp-1", "/Users/sean/.npm", "/home/other/x"], "/Users/sean"),
        (["C:\\Users\\lab\\AppData\\Local\\Temp\\x"], "C:/Users/lab"),
        (["/tmp/y", "/var/folders/ab/T/tmp-1"], ""),
        ([], ""),
    ]
    for paths, expected in cases:
        assert classify.listing_home(paths) == expected, paths


def test_windows_roots_expand_their_variables_and_a_prefix_counts():
    globs = classify.root_globs(ROOTS, "C:/Users/sean", ["C:/Users/sean/AppData/Local/Temp"])
    cases = [
        ("C:\\Users\\sean\\AppData\\Roaming\\example-agent", True),
        ("C:\\Users\\sean\\AppData\\Local\\example-agent", False),
        ("C:\\Users\\sean\\.gemini\\settings.json", True),
        ("C:\\Users\\sean\\AppData\\Local\\Temp\\gemini-client-error-1.json", True),
        ("C:\\Users\\sean\\AppData\\Local\\Temp\\gemini-other.json", False),
        ("C:\\Users\\sean\\.geminix", False),
        ("C:\\Users\\sean\\.local\\share\\keyrings", False),
    ]
    for path, expected in cases:
        assert classify.covered(path, globs) is expected, path


def test_each_windows_variable_expands():
    roots = (
        "windows:%USERPROFILE%/a\nwindows:%LOCALAPPDATA%/b\nwindows:%APPDATA%/c\n"
        "windows:%TEMP%/d\nwindows:%TMP%/e\nwindows:%PROGRAMDATA%/f\nwindows:%OTHER%/g\n"
    )
    globs = classify.root_globs(roots, "C:/Users/sean", ["C:/T1", "C:/T2"])
    assert globs == [
        "C:/Users/sean/a",
        "C:/Users/sean/AppData/Local/b",
        "C:/Users/sean/AppData/Roaming/c",
        "C:/T1/d",
        "C:/T2/d",
        "C:/T1/e",
        "C:/T2/e",
        "C:/ProgramData/f",
    ]


def test_one_platform_never_covers_another():
    roots = (
        "windows:%LOCALAPPDATA%/qwen-code\nlinux:~/.config/only-linux\n"
        "macos:~/Library/only-mac\nunix:~/.unix-agent\n"
    )
    linux = classify.root_globs(roots, "/home/lab", ["/tmp"])
    macos = classify.root_globs(roots, "/Users/lab", ["/tmp"])
    windows = classify.root_globs(roots, "C:/Users/lab", ["C:/Users/lab/AppData/Local/Temp"])
    assert linux == ["/home/lab/.config/only-linux", "/home/lab/.unix-agent"]
    assert macos == ["/Users/lab/Library/only-mac", "/Users/lab/.unix-agent"]
    assert windows == ["C:/Users/lab/AppData/Local/qwen-code"]
    assert not classify.covered("/home/lab/.cache/qwen-code", linux)
    assert not classify.covered("C:\\Users\\lab\\.config\\only-linux", windows)
    assert classify.listing_platform("") == ""


def test_main_reads_the_roots_file(tmp_path, capsys):
    (tmp_path / "before.txt").write_text("".join(TOP_BEFORE), encoding="utf-8")
    (tmp_path / "after.txt").write_text("".join(TOP_AFTER), encoding="utf-8")
    (tmp_path / "roots").write_text(ROOTS, encoding="utf-8")
    arguments = ["--roots", str(tmp_path / "roots"), str(tmp_path / "before.txt")]
    assert classify.main([*arguments, str(tmp_path / "after.txt")]) == 0
    printed = capsys.readouterr().out
    assert "gemini-client-error" not in printed and "/tmp/other-error.json" in printed


def test_the_base_line_names_the_temporary_directory():
    base = "base\ttmp\t/var/folders/ab/T\n"
    before = classify.parse_listing([base, "top\tdir\t1\t755\tt\t/Users/lab/.npm\n"])
    after = classify.parse_listing(
        [
            base,
            "top\tdir\t1\t755\tt\t/Users/lab/.npm\n",
            "top\tdir\t1\t755\tt\t/tmp/gemini\n",
            "top\tdir\t1\t755\tt\t/var/folders/ab/T/gemini\n",
        ]
    )
    assert after.bases == {"tmp": "/var/folders/ab/T"} and after.entries == {}
    found = classify.drop_covered_tops(classify.compare(before, after), after, "<tmp>/gemini\n")
    assert found.new_tops == ["/tmp/gemini"]


def test_a_listing_without_a_base_line_hides_no_temporary_name():
    before = classify.parse_listing(["top\tdir\t1\t755\tt\t/Users/lab/.npm\n"])
    after = classify.parse_listing(
        [
            "top\tdir\t1\t755\tt\t/Users/lab/.npm\n",
            "top\tdir\t1\t755\tt\t/tmp/gemini\n",
            "top\tdir\t1\t755\tt\t/var/folders/ab/T/gemini\n",
        ]
    )
    found = classify.drop_covered_tops(classify.compare(before, after), after, "<tmp>/gemini\n")
    assert found.new_tops == ["/tmp/gemini", "/var/folders/ab/T/gemini"]


def test_case_is_folded_on_windows_and_macos_only():
    roots = (
        "windows:%APPDATA%/example-agent\nmacos:~/Library/Application Support/example-agent\n"
        "linux:~/.config/example-agent\n"
    )
    cases = [
        ("C:\\Users\\lab\\AppData\\Roaming\\Example-Agent", True),
        ("/Users/lab/Library/Application Support/EXAMPLE-AGENT", True),
        ("/home/lab/.config/Example-Agent", False),
    ]
    for path, expected in cases:
        listing = classify.parse_listing(["top\tdir\t1\t755\tt\t%s\n" % path])
        found = classify.drop_covered_tops(
            classify.compare(classify.Listing(), listing), listing, roots
        )
        assert (found.new_tops == []) is expected, path


def test_main_without_roots_shows_every_top_name(tmp_path, capsys):
    (tmp_path / "before.txt").write_text("".join(TOP_BEFORE), encoding="utf-8")
    (tmp_path / "after.txt").write_text("".join(TOP_AFTER), encoding="utf-8")
    assert classify.main([str(tmp_path / "before.txt"), str(tmp_path / "after.txt")]) == 0
    assert "gemini-client-error" in capsys.readouterr().out
