"""Normalise timestamps to the one form the analyzer writes.

Every timestamp in the output is UTC ISO 8601 with millisecond precision and
a trailing Z, so rows from different agents sort together as plain strings.
"""

from __future__ import annotations

import re
from datetime import datetime, timedelta, timezone

from .textlines import ASCII_WHITESPACE

OUT_FMT = "%Y-%m-%dT%H:%M:%S.%f"
_FRACTION_PATTERN = re.compile(r"\.([0-9]+)")
_EPOCH = datetime(1970, 1, 1, tzinfo=timezone.utc)


def to_utc(value: object) -> str:
    """Return `YYYY-MM-DDTHH:MM:SS.mmmZ` for an ISO 8601 string or an epoch number.

    An epoch number, or a string of digits, above 1e11 is milliseconds, any
    other seconds. Input that does not parse gives `""`, so one bad record
    never stops a timeline.
    """
    if value is None or value == "":
        return ""
    try:
        if isinstance(value, bool):
            return ""
        if isinstance(value, (int, float)):
            return _from_epoch(float(value))
        # ISO 8601 and an epoch number are ASCII, padded only by ASCII whitespace.
        text = str(value).strip(ASCII_WHITESPACE)
        if not text.isascii():
            return ""
        if text.replace(".", "", 1).isdigit():
            return _from_epoch(float(text))
        if text.endswith("Z") or text.endswith("z"):
            text = text[:-1] + "+00:00"
        # Before Python 3.11 fromisoformat wants a T and exactly three or six
        # fraction digits, so SQLite's `CURRENT_TIMESTAMP` text, Go's
        # nanoseconds and go-sqlite3's trimmed `.5-08:00` are rewritten first.
        if len(text) > 10 and text[10] == " ":
            text = text[:10] + "T" + text[11:]
        text = _FRACTION_PATTERN.sub(
            lambda match: "." + match.group(1)[:6].ljust(6, "0"), text, count=1
        )
        moment = datetime.fromisoformat(text)
        if moment.tzinfo is None:
            moment = moment.replace(tzinfo=timezone.utc)
        return format_utc(moment.astimezone(timezone.utc))
    except (ValueError, OverflowError, OSError):
        return ""


def _from_epoch(epoch: float) -> str:
    if epoch > 1e11:
        epoch = epoch / 1000.0
    return format_utc(datetime.fromtimestamp(epoch, tz=timezone.utc))


def format_utc(moment: datetime) -> str:
    """Return an aware UTC datetime as a timeline timestamp."""
    return moment.strftime(OUT_FMT)[:-3] + "Z"


def epoch_ms(value: str) -> int | None:
    """Return the epoch milliseconds of a timeline timestamp, an integer so differences are exact.

    Args:
        value: A timestamp as `to_utc` writes it.

    Returns:
        The milliseconds since the epoch, or None for an empty or unparseable value.
    """
    if not value:
        return None
    try:
        moment = datetime.strptime(value, OUT_FMT + "Z")
    except ValueError:
        return None
    return (moment.replace(tzinfo=timezone.utc) - _EPOCH) // timedelta(milliseconds=1)


def first_nonempty(*values: str | None) -> str:
    """Return the first value that is not empty, or `""`."""
    for value in values:
        if value:
            return value
    return ""
