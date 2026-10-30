"""Synthetic Copilot language server state for the analyzer tests."""

from __future__ import annotations

from pathlib import Path

from .common import _json_file


def build_copilot(home: Path) -> None:
    """The Copilot language server's token store, a collected-only agent no parser wants."""
    _json_file(
        home / ".config/github-copilot/apps.json",
        {"github.com:Iv1.fake": {"user": "alice", "oauth_token": "gho_FAKE_TOKEN_0000"}},
    )
