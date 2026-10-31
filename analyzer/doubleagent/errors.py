"""Define the analyzer's own exceptions and the message for a file that cannot be read."""

from __future__ import annotations

from pathlib import Path


class DoubleAgentError(RuntimeError):
    """An error the command reports as `error: <message>` with exit code 2."""


class InputError(DoubleAgentError):
    """An input that is neither a file nor a directory, or an archive that cannot be extracted."""


def unreadable_message(path, error: OSError) -> str:
    """Return `<file name> could not be read (<reason>)` for the OSError that stopped a read.

    The reason is the system's own text (`Permission denied`), or the
    error's whole text when it carries none.
    """
    return "%s could not be read (%s)" % (Path(path).name, error.strerror or str(error))


__all__ = ["DoubleAgentError", "InputError", "unreadable_message"]
