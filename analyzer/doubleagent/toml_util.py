"""Read TOML documents with `tomllib`.

`load_toml` reads the text of a TOML document a parser hands it (a settings, policy or
automation file) and returns the dict `tomllib.loads` gives; a document that breaks the TOML
specification raises `tomllib.TOMLDecodeError`, a `ValueError`.
"""

from __future__ import annotations

import tomllib


def load_toml(text: str) -> dict:
    """Read a TOML document with `tomllib`."""
    return tomllib.loads(text)
