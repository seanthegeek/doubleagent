#!/usr/bin/env python3
"""Extract and query the protobuf message definitions a binary embeds.

Go binaries built with protobuf-go, among others, carry the serialized FileDescriptorProto of
every message they link, which gives field names, numbers, types and enum values. Closed-source
agents such as Antigravity CLI store transcripts as protobuf blobs with no published schema, so
this is the evidence source for the Antigravity parser (`internal/parsers/antigravity`); re-run
it when a new release changes the format.

    tools/proto_descriptors.py extract ~/.local/bin/agy descriptors.json
    tools/proto_descriptors.py show descriptors.json gemini_coder.Step exa.cortex_pb.CortexStepType
    tools/proto_descriptors.py find descriptors.json Summary

Standard library only. Extraction scans for `<name>.proto` strings preceded by a field-1 length
prefix and parses forward until the data stops looking like a FileDescriptorProto, so it needs
no knowledge of the binary's layout.
"""

from __future__ import annotations

import json
import re
import sys
from collections.abc import Iterator
from pathlib import Path
from typing import Any

TYPES = {
    1: "double",
    2: "float",
    3: "int64",
    4: "uint64",
    5: "int32",
    6: "fixed64",
    7: "fixed32",
    8: "bool",
    9: "string",
    10: "group",
    11: "message",
    12: "bytes",
    13: "uint32",
    14: "enum",
    15: "sfixed32",
    16: "sfixed64",
    17: "sint32",
    18: "sint64",
}


def read_varint(data: bytes, offset: int) -> tuple[int, int]:
    """Return the varint at `offset` and the offset after it.

    Raises:
        ValueError: The varint is cut off or longer than ten bytes.
    """
    result = 0
    shift = 0
    while True:
        if offset >= len(data):
            raise ValueError("truncated varint")
        byte = data[offset]
        offset += 1
        result |= (byte & 0x7F) << shift
        shift += 7
        if not byte & 0x80:
            return result, offset
        if shift > 70:
            raise ValueError("varint too long")


def iter_fields(data: bytes) -> Iterator[tuple[int, int, Any]]:
    """Yield (field number, wire type, value) for each field of a protobuf message.

    A length-delimited value is bytes, a varint an int, a fixed width its raw bytes; the value
    is typed Any because its type follows the wire type.

    Raises:
        ValueError: The buffer is cut off or holds a group, whose wire types are not read.
    """
    offset = 0
    size = len(data)
    while offset < size:
        tag, offset = read_varint(data, offset)
        field, wire_type = tag >> 3, tag & 7
        if wire_type == 0:
            value, offset = read_varint(data, offset)
            yield field, wire_type, value
        elif wire_type in (1, 5):
            width = 8 if wire_type == 1 else 4
            yield field, wire_type, data[offset : offset + width]
            offset += width
        elif wire_type == 2:
            length, offset = read_varint(data, offset)
            if length > size - offset:
                raise ValueError("truncated length-delimited field")
            yield field, wire_type, data[offset : offset + length]
            offset += length
        else:
            raise ValueError("unsupported wire type %d" % wire_type)


def _text(data: bytes) -> str:
    """Decode a descriptor string, replacing bytes that are not UTF-8."""
    return data.decode("utf-8", "replace")


def parse_enum(data: bytes) -> dict:
    """Return the name and the number-to-name values of an EnumDescriptorProto."""
    enum = {"name": "", "values": {}}
    if not isinstance(data, bytes):
        raise ValueError("enum descriptor is not length-delimited")
    for field, _wt, value in iter_fields(data):
        if field == 1:
            enum["name"] = _text(value)
        elif field == 2:
            name, number = "", None
            for sub_field, _, sub_value in iter_fields(value):
                if sub_field == 1:
                    name = _text(sub_value)
                elif sub_field == 2:
                    number = sub_value
            enum["values"][str(number)] = name
    return enum


def parse_message(data: bytes) -> dict:
    """Return the name, fields, nested messages, enums and oneofs of a DescriptorProto."""
    message = {"name": "", "fields": [], "nested": [], "enums": [], "oneofs": []}
    if not isinstance(data, bytes):
        raise ValueError("message descriptor is not length-delimited")
    for field, _wt, value in iter_fields(data):
        if field == 1:
            message["name"] = _text(value)
        elif field == 2:
            field_descriptor = {}
            for sub_field, _, sub_value in iter_fields(value):
                if sub_field == 1:
                    field_descriptor["name"] = _text(sub_value)
                elif sub_field == 3:
                    field_descriptor["number"] = sub_value
                elif sub_field == 4:
                    field_descriptor["label"] = sub_value
                elif sub_field == 5:
                    field_descriptor["type"] = sub_value
                elif sub_field == 6:
                    field_descriptor["type_name"] = _text(sub_value)
                elif sub_field == 9:
                    field_descriptor["oneof_index"] = sub_value
            message["fields"].append(field_descriptor)
        elif field == 3:
            message["nested"].append(parse_message(value))
        elif field == 4:
            message["enums"].append(parse_enum(value))
        elif field == 8:
            for sub_field, _, sub_value in iter_fields(value):
                if sub_field == 1:
                    message["oneofs"].append(_text(sub_value))
    return message


def parse_file_fields(data: bytes, start: int):
    """Parse FileDescriptorProto fields from `start` until the stream stops looking like one.

    It stops at a field outside 1..13, a wire type other than varint or length-delimited, or a
    second `name`.
    """
    i, end = start, len(data)
    seen_name = False
    fields = []
    while i < end:
        try:
            tag, j = read_varint(data, i)
        except ValueError:
            break
        field, wire_type = tag >> 3, tag & 7
        if field < 1 or field > 13 or wire_type not in (0, 2):
            break
        if field == 1 and seen_name:
            break
        if wire_type == 0:
            value, j = read_varint(data, j)
            fields.append((field, value))
            i = j
            continue
        try:
            length, j = read_varint(data, j)
        except ValueError:
            break
        if length > end - j or length > 50_000_000:
            break
        chunk = data[j : j + length]
        if field == 1:
            if not chunk.endswith(b".proto"):
                break
            seen_name = True
        fields.append((field, chunk))
        i = j + length
    return fields


def extract(binary: Path) -> dict:
    """Return the file descriptors embedded in `binary`, keyed by `.proto` file name."""
    data = binary.read_bytes()
    results: dict = {}
    for match in re.finditer(rb"\.proto", data):
        end = match.end()
        for back in range(7, 200):
            start = end - back
            if start < 2 or not re.fullmatch(rb"[A-Za-z0-9_./\-]+", data[start:end]):
                break
            for prefix_size in (1, 2):
                length_start = start - prefix_size
                if length_start < 1:
                    continue
                try:
                    length, _ = read_varint(data, length_start)
                except ValueError:
                    continue
                if length != end - start or data[length_start - 1] != 0x0A:
                    continue
                fields = parse_file_fields(data, length_start - 1)
                if not any(field == 4 for field, _ in fields):
                    continue
                descriptor = {"name": "", "package": "", "messages": [], "enums": []}
                try:
                    for field, value in fields:
                        if field == 1:
                            descriptor["name"] = _text(value)
                        elif field == 2:
                            descriptor["package"] = _text(value)
                        elif field == 4:
                            descriptor["messages"].append(parse_message(value))
                        elif field == 5:
                            descriptor["enums"].append(parse_enum(value))
                except ValueError:
                    continue
                key = descriptor["name"]
                if key not in results or len(json.dumps(descriptor)) > len(
                    json.dumps(results[key])
                ):
                    results[key] = descriptor
    return results


def build_index(descriptors: dict) -> dict:
    """Map every message and enum's full name to (kind, descriptor, file name)."""
    index = {}

    def walk(prefix, message, fname):
        full = prefix + message["name"]
        index[full] = ("msg", message, fname)
        for nested in message["nested"]:
            walk(full + ".", nested, fname)
        for enum in message["enums"]:
            index[full + "." + enum["name"]] = ("enum", enum, fname)

    for fname, descriptor in descriptors.items():
        pkg = descriptor["package"] + "." if descriptor["package"] else ""
        for message in descriptor["messages"]:
            walk(pkg, message, fname)
        for enum in descriptor["enums"]:
            index[pkg + enum["name"]] = ("enum", enum, fname)
    return index


def show(index: dict, name: str) -> None:
    """Print the fields of a message or the values of an enum."""
    kind, descriptor, fname = index[name]
    if kind == "enum":
        print("enum %s  [%s]" % (name, fname))
        for number, value_name in sorted(
            descriptor["values"].items(), key=lambda item: int(item[0])
        ):
            print("  %s = %s" % (number, value_name))
        return
    print("message %s  [%s]" % (name, fname))
    for field in sorted(descriptor["fields"], key=lambda field: field.get("number", 0)):
        label = {3: "repeated "}.get(field.get("label"), "")
        type_name = field.get("type_name", "").lstrip(".") or TYPES.get(field.get("type"), "?")
        oneof = " oneof#%d" % field["oneof_index"] if "oneof_index" in field else ""
        print(
            "  %4d %s%s: %s%s"
            % (field.get("number", 0), label, field.get("name"), type_name, oneof)
        )
    if descriptor["oneofs"]:
        print("  oneofs: %s" % descriptor["oneofs"])


def main(argv) -> int:
    """Run the `extract`, `show` or `find` command and return the exit status."""
    if len(argv) < 3:
        print(__doc__)
        return 1
    command = argv[1]
    if command == "extract":
        descriptors = extract(Path(argv[2]))
        Path(argv[3]).write_text(json.dumps(descriptors))
        print("%d descriptor files" % len(descriptors))
        return 0
    descriptors = json.loads(Path(argv[2]).read_text())
    index = build_index(descriptors)
    if command == "show":
        for name in argv[3:]:
            if name in index:
                show(index, name)
            else:
                print("not found: %s" % name)
            print()
        return 0
    if command == "find":
        query = argv[3].lower()
        for k in sorted(index):
            if query in k.lower():
                print(k)
        return 0
    print(__doc__)
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
