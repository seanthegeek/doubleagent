"""Write the zstd rollouts the codex package's tests read.

Run from this directory with a Python that has the zstandard package:
`python -I gen_zst.py`. Each line written is a JSON object, except where a
file's name says otherwise.

- two-lines.jsonl.zst: one frame holding two lines.
- block-per-line.jsonl.zst: one frame holding five lines, each flushed as
  its own block, so a cut keeps the lines of the blocks before it.
- two-frames.jsonl.zst: three lines in one frame and two in a second.
"""

import json

import zstandard

LINES = [
    {
        "timestamp": "2025-01-01T00:00:0%d.000Z" % i,
        "type": "event_msg",
        "payload": {"type": "task_started", "turn_id": "turn-%d" % i},
    }
    for i in range(5)
]


def line(record: dict) -> bytes:
    """Return a record as one compact JSON line."""
    return (json.dumps(record, separators=(",", ":")) + "\n").encode()


with open("two-lines.jsonl.zst", "wb") as handle:
    handle.write(zstandard.ZstdCompressor().compress(b"".join(map(line, LINES[:2]))))

compressor = zstandard.ZstdCompressor().compressobj()
blob = b""
for record in LINES:
    blob += compressor.compress(line(record)) + compressor.flush(zstandard.COMPRESSOBJ_FLUSH_BLOCK)
blob += compressor.flush()
with open("block-per-line.jsonl.zst", "wb") as handle:
    handle.write(blob)

with open("two-frames.jsonl.zst", "wb") as handle:
    context = zstandard.ZstdCompressor()
    handle.write(context.compress(b"".join(map(line, LINES[:3]))))
    handle.write(context.compress(b"".join(map(line, LINES[3:]))))
