"""Read TOML documents with `tomllib`.

Reads the text of a TOML document a parser hands it (a settings, policy or automation file)
and returns the dict `tomllib.loads` gives. A parser calls `load_toml` for a document whose
top level it needs and `load_config_toml` for one whose tables it needs; both are
`tomllib.loads`, and both raise `tomllib.TOMLDecodeError`, a `ValueError`, on a document that
breaks the TOML specification.
"""

from __future__ import annotations

import tomllib


def load_toml(text: str) -> dict:
    """Read a TOML document with `tomllib`."""
    return tomllib.loads(text)


def load_config_toml(text: str) -> dict:
    """Read a `config.toml` document with `tomllib`."""
    return tomllib.loads(text)
