"""Tests for tools/proto_descriptors.py over a synthetic binary holding one file descriptor."""

from __future__ import annotations

import pathlib

import proto_descriptors


def varint(value: int) -> bytes:
    """Return the protobuf varint encoding of a non-negative integer."""
    encoded = bytearray()
    while True:
        byte = value & 0x7F
        value >>= 7
        if value:
            encoded.append(byte | 0x80)
        else:
            encoded.append(byte)
            return bytes(encoded)


def length_field(number: int, payload: bytes) -> bytes:
    """Return a length-delimited field."""
    return varint(number << 3 | 2) + varint(len(payload)) + payload


def varint_field(number: int, value: int) -> bytes:
    """Return a varint field."""
    return varint(number << 3) + varint(value)


def field_descriptor(name: str, number: int, kind: int, type_name: str = "") -> bytes:
    """Return a FieldDescriptorProto with a name, number, label, type and type name."""
    encoded = length_field(1, name.encode()) + varint_field(3, number) + varint_field(4, 1)
    encoded += varint_field(5, kind)
    return encoded + (length_field(6, type_name.encode()) if type_name else b"")


def file_descriptor() -> bytes:
    """Return a FileDescriptorProto `demo.proto` of package `demo` with a message and an enum."""
    enum_values = length_field(1, b"KIND_NONE") + varint_field(2, 0)
    enum_values_two = length_field(1, b"KIND_TOOL") + varint_field(2, 7)
    enum = (
        length_field(1, b"Kind") + length_field(2, enum_values) + length_field(2, enum_values_two)
    )
    message = length_field(1, b"Step")
    message += length_field(2, field_descriptor("text", 1, 9))
    message += length_field(2, field_descriptor("kind", 2, 14, ".demo.Kind"))
    message += length_field(4, enum)
    return (
        length_field(1, b"demo.proto")
        + length_field(2, b"demo")
        + length_field(4, message)
        + length_field(5, length_field(1, b"Kind") + length_field(2, enum_values))
    )


def test_varints_round_trip_and_a_cut_one_is_refused():
    for value in (0, 1, 127, 128, 300, 2**35):
        assert proto_descriptors.read_varint(varint(value), 0) == (value, len(varint(value)))
    for broken in (b"", b"\x80", b"\xff" * 11):
        try:
            proto_descriptors.read_varint(broken, 0)
        except ValueError:
            continue
        raise AssertionError("accepted %r" % broken)


def test_fields_give_each_wire_type_its_value():
    data = varint_field(1, 150) + length_field(2, b"ab") + b"\x1d" + b"wxyz" + b"\x21" + b"8bytes!!"
    assert list(proto_descriptors.iter_fields(data)) == [
        (1, 0, 150),
        (2, 2, b"ab"),
        (3, 5, b"wxyz"),
        (4, 1, b"8bytes!!"),
    ]


def test_a_length_past_the_end_is_refused():
    try:
        list(proto_descriptors.iter_fields(b"\x12\x05ab"))
    except ValueError as error:
        assert "truncated" in str(error)
    else:
        raise AssertionError("a cut field was accepted")


def test_extract_finds_the_descriptor_among_other_bytes(tmp_path: pathlib.Path):
    binary = tmp_path / "agent.bin"
    binary.write_bytes(b"\x00junk.proto\x01" + file_descriptor() + b"\xff\xfe trailing")
    descriptors = proto_descriptors.extract(binary)
    assert list(descriptors) == ["demo.proto"]
    descriptor = descriptors["demo.proto"]
    assert descriptor["package"] == "demo"
    step = descriptor["messages"][0]
    assert [field["name"] for field in step["fields"]] == ["text", "kind"]
    assert step["enums"][0]["values"] == {"0": "KIND_NONE", "7": "KIND_TOOL"}
    assert descriptor["enums"][0]["name"] == "Kind"


def test_show_prints_a_message_and_a_nested_enum(tmp_path: pathlib.Path, capsys):
    binary = tmp_path / "agent.bin"
    binary.write_bytes(file_descriptor())
    index = proto_descriptors.build_index(proto_descriptors.extract(binary))
    assert sorted(index) == ["demo.Kind", "demo.Step", "demo.Step.Kind"]
    proto_descriptors.show(index, "demo.Step")
    proto_descriptors.show(index, "demo.Step.Kind")
    assert capsys.readouterr().out.splitlines() == [
        "message demo.Step  [demo.proto]",
        "     1 text: string",
        "     2 kind: demo.Kind",
        "enum demo.Step.Kind  [demo.proto]",
        "  0 = KIND_NONE",
        "  7 = KIND_TOOL",
    ]
