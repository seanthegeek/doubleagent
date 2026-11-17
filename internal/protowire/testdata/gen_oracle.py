"""Write oracle.txt: wire bytes and what the Python decoder makes of them.

Run from the repository root with the analyzer's interpreter:

    analyzer/.venv/bin/python -I internal/protowire/testdata/gen_oracle.py

Each line is `label<TAB>hex bytes<TAB>expected`, where expected is the
rendering decode_test.go reproduces: `ERR` for a ValueError, else a rendering
of the decoded message (see `render`).
"""

import random
import struct
import sys
from pathlib import Path

sys.path.insert(0, str(Path("analyzer").resolve()))

from doubleagent import protobuf

SCHEMA = {
    "Outer": {
        1: ("name", "str"),
        2: ("+tags", "str"),
        3: ("count", "int"),
        4: ("flag", "bool"),
        5: ("ratio", "float"),
        6: ("wide", "double"),
        7: ("when", "ts"),
        8: ("inner", "Inner"),
        9: ("+items", "Inner"),
        10: ("+numbers", "int"),
        11: ("+flags", "bool"),
        12: ("raw", "bytes"),
        13: ("also", "Inner"),
    },
    "Inner": {1: ("label", "str"), 2: ("value", "int"), 3: ("deeper", "Inner")},
}


def render(value):
    """Return the text form of a decoded value."""
    if isinstance(value, dict):
        return "{" + ";".join(f"{key}={render(item)}" for key, item in value.items()) + "}"
    if isinstance(value, list):
        return "[" + ",".join(render(item) for item in value) + "]"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        return "f" + struct.pack(">d", value).hex()
    if isinstance(value, bytes):
        return 'x"' + value.hex() + '"'
    return 's"' + value.encode("utf-8").hex() + '"'


def outcome(data):
    """Return the rendering of decoding `data` as Outer, or ERR."""
    try:
        return render(protobuf.decode(data, "Outer", SCHEMA))
    except ValueError:
        return "ERR"


def varint(value):
    """Return the varint encoding of `value`."""
    return protobuf._write_varint(value)


def field(number, wire, payload):
    """Return one field: tag plus payload."""
    return protobuf._tag(number, wire) + payload


def delimited(number, payload):
    """Return a length-delimited field."""
    return field(number, 2, varint(len(payload)) + payload)


def hand_cases():
    """Return the hand-written (label, bytes) cases."""
    inner = lambda label, value: delimited(1, label) + field(2, 0, varint(value))  # noqa: E731
    return [
        ("an empty buffer is an empty message", b""),
        ("a string field", delimited(1, b"hello")),
        ("a string with an invalid byte", delimited(1, b"a\xffb")),
        ("a string cut off mid sequence", delimited(1, b"a\xe2\x82")),
        ("a string with an overlong lead", delimited(1, b"\xc0\x80x")),
        ("a string with a surrogate encoding", delimited(1, b"\xed\xa0\x80z")),
        ("a string with a lead above the range", delimited(1, b"\xf5\x80\x80\x80")),
        ("a string with a four byte character cut short", delimited(1, b"\xf0\x9f\x98")),
        ("a string with a four byte character", delimited(1, "x\U0001f600".encode())),
        ("a string field given as a varint", field(1, 0, varint(12345))),
        ("a repeated string", delimited(2, b"a") + delimited(2, b"b") + delimited(2, b"")),
        (
            "a plain field seen twice keeps its first place",
            delimited(1, b"a") + field(3, 0, varint(2)) + delimited(1, b"b"),
        ),
        ("a varint int", field(3, 0, varint(300))),
        ("a ten byte varint int", field(3, 0, varint(2**64 - 1))),
        ("an int from eight bytes", field(3, 1, struct.pack("<q", -5))),
        ("an int from four bytes", field(3, 5, struct.pack("<i", -7))),
        ("an int from four length bytes", delimited(3, struct.pack("<i", -9))),
        ("an int from three length bytes", delimited(3, b"abc")),
        ("an int from a cut fixed field", field(3, 1, b"abc")),
        ("a bool from a varint", field(4, 0, varint(1))),
        ("a false bool", field(4, 0, varint(0))),
        ("a bool from bytes", delimited(4, b"x")),
        ("a bool from empty bytes", delimited(4, b"")),
        ("a float from four bytes", field(5, 5, struct.pack("<f", 1.5))),
        ("a float from a varint", field(5, 0, varint(7))),
        ("a float from eight bytes", field(5, 1, struct.pack("<d", 2.5))),
        ("a float nan", field(5, 5, struct.pack("<f", float("nan")))),
        ("a float negative infinity", field(5, 5, struct.pack("<f", float("-inf")))),
        ("a double from eight bytes", field(6, 1, struct.pack("<d", -0.1))),
        ("a double from a varint", field(6, 0, varint(2**63 + 1))),
        ("a double from four bytes", field(6, 5, struct.pack("<f", 1.5))),
        (
            "a timestamp message",
            delimited(7, field(1, 0, varint(1700000000)) + field(2, 0, varint(123456789))),
        ),
        (
            "a timestamp with a half millisecond rounding",
            delimited(7, field(1, 0, varint(1700000000)) + field(2, 0, varint(5000))),
        ),
        ("a timestamp with no seconds", delimited(7, field(2, 0, varint(5)))),
        ("a timestamp with seconds only", delimited(7, field(1, 0, varint(86400)))),
        ("a timestamp in milliseconds range", delimited(7, field(1, 0, varint(1700000000000)))),
        ("a timestamp out of the date range", delimited(7, field(1, 0, varint(2**62)))),
        (
            "a timestamp near the date limit",
            delimited(7, field(1, 0, varint(253402300799)) + field(2, 0, varint(999999999))),
        ),
        (
            "a timestamp with rounding up to the next second",
            delimited(7, field(1, 0, varint(1700000000)) + field(2, 0, varint(999999999))),
        ),
        ("a timestamp from a varint", field(7, 0, varint(1700000000))),
        ("a timestamp varint in milliseconds", field(7, 0, varint(1700000000123))),
        ("a timestamp varint of zero", field(7, 0, varint(0))),
        ("a timestamp that is cut off", delimited(7, b"\x08")),
        (
            "a timestamp with other fields",
            delimited(7, field(3, 0, varint(1)) + field(1, 0, varint(5)) + delimited(2, b"zz")),
        ),
        ("a timestamp from a fixed field", field(7, 1, field(1, 0, varint(9)) + b"\x00\x00")),
        ("a nested message", delimited(8, inner(b"a", 1))),
        (
            "a nested message seen twice replaces the first",
            delimited(8, inner(b"a", 1)) + delimited(8, inner(b"b", 2)),
        ),
        ("a repeated nested message", delimited(9, inner(b"a", 1)) + delimited(9, inner(b"b", 2))),
        (
            "a malformed nested message is left out",
            delimited(8, b"\x0a\x05ab") + delimited(1, b"ok"),
        ),
        (
            "a nested message given as a varint is left out",
            field(8, 0, varint(5)) + delimited(1, b"ok"),
        ),
        ("a nested message from a fixed field", field(8, 1, b"\x10\x01\x00\x00\x00\x00\x00\x00")),
        ("a deeper nested message", delimited(8, delimited(3, delimited(3, inner(b"x", 9))))),
        ("an empty nested message", delimited(8, b"")),
        ("a packed repeated int", delimited(10, varint(1) + varint(300) + varint(2**64 - 1))),
        ("an empty packed repeated int", delimited(10, b"")),
        ("an unpacked repeated int", field(10, 0, varint(4)) + field(10, 0, varint(5))),
        (
            "a packed and an unpacked repeated int",
            field(10, 0, varint(4)) + delimited(10, varint(5) + varint(6)),
        ),
        ("a packed repeated bool", delimited(11, varint(1) + varint(0) + varint(7))),
        ("a bad packed run fails the whole message", delimited(10, b"\x80") + delimited(1, b"x")),
        (
            "a bad packed run in a nested message leaves it out",
            delimited(8, delimited(2, b"")) + delimited(9, b"\x12\x01") + delimited(1, b"y"),
        ),
        ("bytes", delimited(12, b"\x00\xff")),
        ("bytes from a varint", field(12, 0, varint(5))),
        (
            "unknown fields are skipped",
            field(99, 0, varint(1))
            + delimited(98, b"zz")
            + field(97, 1, b"12345678")
            + field(96, 5, b"1234")
            + delimited(1, b"k"),
        ),
        ("field number zero is unknown", field(0, 0, varint(1))),
        ("a huge field number", field(2**40, 0, varint(1)) + delimited(1, b"k")),
        ("a truncated varint", b"\x08\x80"),
        ("a truncated tag", b"\x80"),
        ("a truncated length-delimited field", b"\x0a\x05abc"),
        ("a length beyond the buffer", b"\x0a\xff\xff\xff\xff\x0f"),
        ("a start group wire type", b"\x0b"),
        ("an end group wire type", b"\x0c"),
        ("wire type six", b"\x0e"),
        ("wire type seven", b"\x0f"),
        ("a cut fixed32 at the end", b"\x2d\x01\x02"),
        ("a cut fixed64 at the end", b"\x31\x01\x02"),
        (
            "a cut fixed field with more fields after it is consumed",
            b"\x31\x01\x02\x03\x04\x05\x06\x07\x08\x08\x01",
        ),
        ("a ten byte tag", b"\xff\xff\xff\xff\xff\xff\xff\xff\xff\x01\x00"),
    ]


def random_cases(count):
    """Return `count` seeded random (label, bytes) cases."""
    rng = random.Random(527)
    cases = []
    for index in range(count):
        values = {}
        if rng.random() < 0.7:
            values["name"] = rng.choice(["", "abc", "héllo", "\U0001f600", "x" * 40])
        if rng.random() < 0.5:
            values["tags"] = [rng.choice(["a", "bb", "中"]) for _ in range(rng.randint(0, 3))]
        if rng.random() < 0.6:
            values["count"] = rng.choice([0, 1, 127, 128, 2**31, 2**63, 2**64 - 1])
        if rng.random() < 0.4:
            values["flag"] = rng.random() < 0.5
        if rng.random() < 0.4:
            values["ratio"] = rng.choice([0.0, 1.5, -2.25, 3.4e38])
        if rng.random() < 0.4:
            values["wide"] = rng.choice([0.0, 1e300, -1e-300, 0.1])
        if rng.random() < 0.6:
            values["when"] = rng.choice(
                [1.7e9, 1.7e9 + 0.123456, 1.7e9 + 0.0005, 1e9, 253402300799.5, 1.0, 5e8 + 0.9996]
            )
        if rng.random() < 0.4:
            values["inner"] = {"label": "in", "value": rng.randint(0, 2**40)}
        if rng.random() < 0.4:
            values["items"] = [
                {"label": "i%d" % step, "value": step} for step in range(rng.randint(0, 3))
            ]
        if rng.random() < 0.4:
            values["numbers"] = [rng.randint(0, 2**20) for _ in range(rng.randint(0, 4))]
        if rng.random() < 0.3:
            values["flags"] = [rng.random() < 0.5 for _ in range(rng.randint(0, 4))]
        data = protobuf.encode(values, "Outer", SCHEMA)
        mutation = rng.choice(["none", "none", "cut", "flip", "append"])
        if mutation == "cut" and data:
            data = data[: rng.randrange(len(data))]
        elif mutation == "flip" and data:
            place = rng.randrange(len(data))
            flipped = data[place] ^ (1 << rng.randrange(8))
            data = data[:place] + bytes([flipped]) + data[place + 1 :]
        elif mutation == "append":
            data += bytes(rng.randrange(256) for _ in range(rng.randint(1, 6)))
        cases.append(("generated case %d (%s)" % (index, mutation), data))
    return cases


def main():
    """Write oracle.txt beside this script."""
    lines = []
    for label, data in hand_cases() + random_cases(400):
        lines.append(f"{label}\t{data.hex()}\t{outcome(data)}")
    (Path(__file__).parent / "oracle.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
