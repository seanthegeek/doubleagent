"""Record the Python analyzer's behaviour while its test suite runs, as a corpus for the Go replay.

Run `pytest -n 0 -p recorder.plugin` from `analyzer/` with `PARITY_CORPUS` naming the output
directory (absent, empty or an earlier corpus). The suite runs unchanged; each call of a parser's
`parse`, `wants`, `secondary` and `secondary_tolerance`, of `inputs.open_input` and of `cli.main`
becomes a case, and the driver adds the full fixture home and the example collection at the end.
Temporary directories are made under a root padded to `ROOT_LENGTH` bytes inside a work
directory (`PARITY_RECORD_WORK`, default the corpus path with `.work` added), removed at the end.
The corpus holds only text files sorted for a stable diff, no time of the recording, and blobs:

- `corpus.json`: `format`; `root_length` and `root_placeholder`, the text of exactly that many
  bytes standing for the root inside blobs, so offsets in a binary file survive; `root_token`
  and `repo_token`, standing for the root and the repository in cases, trees and times.
- `blobs/<sha256>`: a file's bytes, named by their hash after that substitution.
- `trees/<sha256>.json`: one directory, `{"entries": [...]}` sorted by name bytes, each with
  `name`, `kind` (`file`, `dir`, `link`, `fifo`, `socket`, `char`, `block`), `mode` (permission
  bits) and `blob` (null when never readable), `tree` or `target`.
- `times/<sha256>.json`: `{"mtime_ns": {...}}`, the modification time of every entry of one
  snapshot by root-relative path (`hex:<bytes>` when not UTF-8), apart from the trees so that
  the fixture trees each test rebuilds share their tree objects.
- `parsers/<agent>.jsonl`, `commands/<command>.jsonl`, `inputs.jsonl`: one case per line in call
  order, the fields in `cases.py`; `summary.json`: the counts and the tests without a case.

A text that is not valid UTF-8 (a lone surrogate from JSON, a file name's bytes) is written as
`{"hex": ...}`: UTF-8 with surrogates encoded as themselves for text, raw bytes for a name. Every
test seeds the temporary names alike, so a fixture that writes its own path repeats its bytes.
"""
