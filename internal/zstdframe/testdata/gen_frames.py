"""Write the zstd frames under frames/ and the table frames.txt that describes them.

Run from the repository root with the analyzer's interpreter:

    analyzer/.venv/bin/python -I internal/zstdframe/testdata/gen_frames.py

Each line of frames.txt is `name<TAB>sha256 of the decoded bytes<TAB>decoded size<TAB>shape`.
"""

import hashlib
import json
import random
from pathlib import Path

import zstandard

HERE = Path(__file__).parent
FRAMES = HERE / "frames"


def jsonl(size, seed):
    """Return about `size` bytes of seeded JSON lines, cut to exactly `size` bytes."""
    rng = random.Random(seed)
    words = ["alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "été"]
    out = bytearray()
    count = 0
    while len(out) < size:
        record = {
            "n": count,
            "role": rng.choice(["user", "assistant", "tool"]),
            "text": " ".join(rng.choice(words) for _ in range(rng.randint(3, 40))),
            "token": "%032x" % rng.getrandbits(128),
        }
        out += json.dumps(record, ensure_ascii=False).encode() + b"\n"
        count += 1
    return bytes(out[:size])


def compress(data, level=3, checksum=False, content_size=True):
    """Return one frame of `data`."""
    return zstandard.ZstdCompressor(
        level=level, write_checksum=checksum, write_content_size=content_size
    ).compress(data)


def skippable(payload):
    """Return a skippable frame carrying `payload`."""
    return b"\x50\x2a\x4d\x18" + len(payload).to_bytes(4, "little") + payload


def far_reference():
    """Return a frame whose last bytes repeat bytes 9 MiB back, with a 128 MiB window."""
    chunk = random.Random(9).randbytes(4096)
    data = chunk + bytes(9 << 20) + chunk
    params = zstandard.ZstdCompressionParameters.from_level(
        19, window_log=27, enable_ldm=True, ldm_hash_log=20, ldm_min_match=64
    )
    return data, zstandard.ZstdCompressor(compression_params=params).compress(data)


def wide_window(data):
    """Return `data` as a frame that announces a 128 MiB window."""
    params = zstandard.ZstdCompressionParameters.from_level(19, window_log=27)
    return zstandard.ZstdCompressor(compression_params=params).compress(data)


def too_wide(data):
    """Return `data` as a frame without a content size whose header announces a 256 MiB window.

    The compressor shrinks the window to fit small input, so the window byte is rewritten:
    exponent 18 gives 2^(10+18) bytes. Python's streaming decoders refuse such a frame.
    """
    frame = bytearray(compress(data, 3, content_size=False))
    assert frame[4] == 0x00, "a window byte follows only when the single-segment bit is clear"
    frame[5] = 18 << 3
    return bytes(frame)


def cases():
    """Return the (name, shape, decoded bytes, frame bytes) cases."""
    out = []
    for size in (0, 1, 1000, 70000, 300000):
        data = jsonl(size, size)
        for level in (1, 3, 19):
            out.append((f"size{size}-level{level}", "one frame", data, compress(data, level)))
        out.append((f"size{size}-checksum", "checksum", data, compress(data, 3, checksum=True)))
        out.append(
            (f"size{size}-nosize", "no content size", data, compress(data, 3, content_size=False))
        )
    first, second, third = jsonl(5000, 1), jsonl(120000, 2), jsonl(300, 3)
    out.append(
        (
            "multi-frame",
            "three frames",
            first + second + third,
            compress(first, 1, True) + compress(second, 3) + compress(third, 19, False, False),
        )
    )
    out.append(
        (
            "skippable-between",
            "a skippable frame between two frames",
            first + third,
            compress(first) + skippable(b"skip me") + compress(third),
        )
    )
    wide = jsonl(300000, 7)
    out.append(("wide-window", "announces a 128 MiB window, matches near", wide, wide_window(wide)))
    data, frame = far_reference()
    out.append(("far-reference", "128 MiB window, a match 9 MiB back", data, frame))
    small = jsonl(5000, 8)
    out.append(("too-wide-window", "announces a 256 MiB window", small, too_wide(small)))
    return out


def main():
    """Write the frames and frames.txt."""
    FRAMES.mkdir(exist_ok=True)
    lines = []
    for name, shape, data, frame in cases():
        (FRAMES / f"{name}.zst").write_bytes(frame)
        digest = hashlib.sha256(data).hexdigest()
        lines.append(f"{name}\t{digest}\t{len(data)}\t{shape}")
    (HERE / "frames.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
