"""Tests of classify.py, the listing diff of a round step."""

from __future__ import annotations

import classify

BEFORE = [
    "dir\t4096\t755\t2026-11-01 10:00:00.000000000 +0000\t/home/sean/.gemini\n",
    "file\t10\t644\t2026-11-01 10:00:00.000000000 +0000\t/home/sean/.gemini/settings.json\n",
    "file\t5\t600\t2026-11-01 10:00:00.000000000 +0000\t/home/sean/.gemini/old.json\n",
    "top\tdir\t4096\t755\t2026-11-01 10:00:00.000000000 +0000\t/home/sean/.gemini\n",
    "keychain\tabsent\t-\t-\tgemini-cli-oauth\n",
    "process\t0\t-\t-\tgemini\n",
]
SESSION = "/home/sean/.gemini/tmp/proj/chats/session-2026-11-01T10-05-0123abcd.jsonl"
AFTER = [
    "dir\t4096\t700\t2026-11-01 10:05:00.000000000 +0000\t/home/sean/.gemini\n",
    "file\t12\t644\t2026-11-01 10:05:00.000000000 +0000\t/home/sean/.gemini/settings.json\n",
    "file\t900\t644\t2026-11-01 10:05:00.000000000 +0000\t%s\n" % SESSION,
    "top\tdir\t4096\t755\t2026-11-01 10:00:00.000000000 +0000\t/home/sean/.gemini\n",
    "top\tdir\t4096\t755\t2026-11-01 10:05:00.000000000 +0000\t/home/sean/.npm\n",
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
    before = classify.parse_listing(["dir\t4096\t755\tt1\t/home/sean/.gemini\n"])
    after = classify.parse_listing(["dir\t8192\t755\tt2\t/home/sean/.gemini\n"])
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
