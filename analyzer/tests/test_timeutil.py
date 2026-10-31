"""Tests of timestamp normalisation."""

from doubleagent.timeutil import to_utc


def test_ascii_digits_and_padding_only():
    assert to_utc(" 1700000000\t") == "2023-11-14T22:13:20.000Z"
    assert to_utc("\n2026-10-01T08:00:00.5Z ") == "2026-10-01T08:00:00.500Z"
    for value in (
        chr(0x0663),
        "170000000" + chr(0x0663),
        "2026-10-01T08:00:00." + chr(0x0663) + "Z",
        "2026-10-01T08:00:0" + chr(0x0663) + "Z",
    ):
        assert to_utc(value) == "", repr(value)
    for space in map(chr, (0x00A0, 0x2028, 0x0085)):
        for value in (space + "1700000000", "2026-10-01T08:00:00Z" + space):
            assert to_utc(value) == "", repr(value)
