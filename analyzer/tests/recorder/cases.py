"""Describe the values a case holds, as plain JSON-ready dicts.

Every case has `node` (the pytest node id, or `recorder::<what>` for the driver's), `seq` (the
order cases ended in: a parse ends when its rows run out, a command after the calls it made) and, when a `mock.patch` or `monkeypatch` was active, `patched`
(the patched targets). `tree` is the hash of a tree object of the root holding only the entries
the case's paths lie under, and `times` that of their times; both are null when no path lies
under the root. A path in case text starts with the root token. The `result` of a parse or an
input case is stored as a blob of its JSON, named by `result_blob`, since most repeat.

- Parser case (`parsers/<agent>.jsonl`): `method` (`parse`, `wants`, `secondary`,
  `secondary_tolerance`), `agent`, `class`, `module`, `instance` (`<class>#<n>`, the n-th
  instance of that class seen) and `call` (its place among that instance's calls, counted as a
  parse starts running: some parsers cache across calls), `artifact` (`disk_path`, `original`, `rel`, `agent`, `manifest`,
  `home`: `disk_path`, `original`, `user`, `host`, `inferred`, `project`, `staged_paths`; a
  stand-in carrying only some attributes has only those and `stand_in` true), `options` (parse
  only), `tree`, and `result`: for parse `rows` (every field, in yield order), `completed`
  (the generator ran to its end) and `error` (`type`, `text`) or null; otherwise `value`, or
  `error`. A repeated wants-style call identical but for `call` is written once.
- Input case (`inputs.jsonl`): the arguments `path`, `work_dir`, `host`, the `tree`, and
  `result` (`kind`, `source`, `root`, `host`, `notes`, `problems`, `summary`, `homes`, and
  `artifacts`, each naming its home by index into `homes`, or holding it when it is none of
  them) or `error`.
- Command case (`commands/<command>.jsonl`, the first argument when it is a command, else
  `other`): `argv`, `cwd`, `clock` (each current time the run read, ISO 8601), `tree` (before
  the run), `stdout`, `stderr`, `exit` (the return value or the `SystemExit` code) or `error`,
  `outputs` (root-relative path, a name not UTF-8 as `hex:<bytes>`, -> blob hash, or
  `{"link": target}`, for each file the run created or changed) and `removed` (the paths it
  deleted).
"""

from __future__ import annotations

import dataclasses
from typing import Any

from doubleagent.inputs import Artifact, Collection, Home

ARTIFACT_FIELDS = ("disk_path", "original", "rel", "agent", "manifest")
HOME_FIELDS = ("disk_path", "original", "user", "host", "inferred", "project", "staged_paths")


def home_fields(home: Any) -> dict[str, Any]:
    """Return the fields of a `Home`, or of a stand-in, as far as it has them."""
    found = {name: getattr(home, name) for name in HOME_FIELDS if hasattr(home, name)}
    if not isinstance(home, Home):
        found["stand_in"] = True
    return found


def artifact_fields(artifact: Any) -> dict[str, Any]:
    """Return the fields of an `Artifact`, its home's included, or a stand-in's as far as present."""
    found: dict[str, Any] = {
        name: getattr(artifact, name) for name in ARTIFACT_FIELDS if hasattr(artifact, name)
    }
    if hasattr(artifact, "home"):
        found["home"] = home_fields(artifact.home)
    if not isinstance(artifact, Artifact):
        found["stand_in"] = True
    return found


def row_fields(row: Any) -> Any:
    """Return a row's fields; something that is not a dataclass is kept as its repr."""
    if dataclasses.is_dataclass(row) and not isinstance(row, type):
        return dataclasses.asdict(row)
    return {"repr": repr(row)}


def error_fields(error: BaseException) -> dict[str, str]:
    """Return an exception's qualified type and text."""
    kind = type(error)
    return {"type": "%s.%s" % (kind.__module__, kind.__qualname__), "text": str(error)}


def collection_fields(collection: Collection) -> dict[str, Any]:
    """Return what `open_input` gave: the collection's description, homes and artifacts."""
    homes = list(collection.homes)
    artifacts = []
    for artifact in collection.artifacts:
        fields = artifact_fields(artifact)
        index = next((i for i, home in enumerate(homes) if home is artifact.home), None)
        if index is not None:
            fields["home"] = index
        artifacts.append(fields)
    return {
        "kind": collection.kind,
        "source": collection.source,
        "root": collection.root,
        "host": collection.host,
        "notes": collection.notes,
        "problems": collection.problems,
        "summary": collection.summary,
        "homes": [home_fields(home) for home in homes],
        "artifacts": artifacts,
    }
