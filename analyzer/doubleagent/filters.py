"""Filter the timeline's rows by a time window (`--since`, `--until`) and by text (`--match`).

A `--since` or `--until` value is an ISO 8601 time, read by python-dateutil's
strict parser (`dateutil.parser.isoparse`), with an offset optionally after a
space and `UTC` for Z; epoch seconds or milliseconds; `today`, `yesterday` or
`now`; or a time relative to now (`45m`, `36h`, `3d`, `2w`, `6mo`, `1y`, with
an optional `ago`), months and years counted with `dateutil.relativedelta`.
Anything else is an error that lists the forms; nothing is guessed. A value
without an offset is UTC. A value naming a period (a year, a month, a day,
an hour, a minute, a second) gives its first millisecond for `--since` and
its last for `--until`. The forms and their periods are listed in
`analyzer/docs/filtering.md` under "WHEN forms".

Bounds are canonical UTC strings in the timeline's own format, so a row is
kept by a plain string comparison: since <= timestamp_utc <= until.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from datetime import UTC, datetime, timedelta

from dateutil.parser import isoparse
from dateutil.relativedelta import relativedelta

from .errors import DoubleAgentError
from .model import Row
from .timeutil import format_utc, to_utc

FORMS_HELP = (
    "accepted: ISO 8601 such as 2026-10-01, 2026-10-01T09:30[:SS][Z|+HH:MM] or "
    "2026-10-01 09:30[:SS] [+HH:MM|UTC]; 2026-10 or 2026; epoch seconds; today, yesterday, now; "
    "or a relative time such as 45m, 36h, 3d, 2w, 6mo, 1y, optionally followed by 'ago'"
)

_YEAR_PATTERN = re.compile(r"^\d{4}$")
_MONTH_PATTERN = re.compile(r"^\d{4}-\d{2}$")
# The time of day after the T or space, extended (09:30:15) or basic
# (093015) format, to find the precision the value was written to.
_TIME_PATTERN = re.compile(r"\d[T ](\d{2})(?::?(\d{2})(?::?(\d{2})(\.\d+)?)?)?", re.IGNORECASE)
# dateutil messages worth passing on; the rest describe its internals.
_RANGE_MESSAGE_PATTERN = re.compile(
    r"must be in|invalid hours|invalid minutes|out of range", re.IGNORECASE
)
# An offset after a space, which isoparse does not accept; it is moved onto
# the time before parsing.
_SPACED_OFFSET_PATTERN = re.compile(r"^(.*\d) (z|utc|[+-]\d{2}(?::?\d{2})?)$", re.IGNORECASE)
_EPOCH_PATTERN = re.compile(r"^\d{9,}(\.\d+)?$")
_RELATIVE_PATTERN = re.compile(
    r"^(\d+) ?(m|min|mins|minutes?|h|hours?|d|days?|w|weeks?|mo|months?|y|years?)(?: ago)?$",
    re.IGNORECASE,
)
_UNIT = {
    "m": "minutes", "min": "minutes", "mins": "minutes", "minute": "minutes", "minutes": "minutes",
    "h": "hours", "hour": "hours", "hours": "hours",
    "d": "days", "day": "days", "days": "days",
    "w": "weeks", "week": "weeks", "weeks": "weeks",
    "mo": "months", "month": "months", "months": "months",
    "y": "years", "year": "years", "years": "years",
}  # fmt: skip


class BoundError(DoubleAgentError):
    """A --since or --until value that is not one of the accepted forms.

    Every message ends with the accepted forms, whatever the reason, so the
    reason is given alone.
    """

    def __init__(self, reason: str) -> None:
        """Build the message from the reason and the accepted forms."""
        super().__init__("%s; %s" % (reason, FORMS_HELP))


def parse_bound(text: str, end: bool = False, now: datetime | None = None) -> str:
    """Return the canonical UTC string for a `--since` or `--until` value.

    Args:
        text: The value as given.
        end: True for `--until`, which takes the last millisecond of a period.
        now: The current time, for tests; by default the clock's.

    Raises:
        BoundError: The value is not one of the accepted forms.
    """
    text_value = " ".join(text.split())
    lowered = text_value.lower()
    now = (now or datetime.now(UTC)).astimezone(UTC)
    if lowered == "now":
        return format_utc(now)
    if lowered in ("today", "yesterday"):
        day = now.replace(hour=0, minute=0, second=0, microsecond=0)
        if lowered == "yesterday":
            day -= timedelta(days=1)
        return _bound(day, timedelta(days=1), end)
    match = _RELATIVE_PATTERN.match(text_value)
    if match:
        amounts = {unit: 0 for unit in _UNIT.values()}
        try:
            # int() refuses a string past Python's digit limit with ValueError.
            amounts[_UNIT[match.group(2).lower()]] = int(match.group(1))
            ago = relativedelta(
                years=amounts["years"],
                months=amounts["months"],
                weeks=amounts["weeks"],
                days=amounts["days"],
                hours=amounts["hours"],
                minutes=amounts["minutes"],
            )
            return format_utc(now - ago)
        except (ValueError, OverflowError):
            raise BoundError("%r is out of range" % text) from None
    if _EPOCH_PATTERN.match(text_value):
        out = to_utc(text_value)
        if not out:
            raise BoundError("%r is out of range as epoch seconds or milliseconds" % text)
        return out
    match = _SPACED_OFFSET_PATTERN.match(text_value)
    if match:
        offset = match.group(2)
        text_value = match.group(1) + ("Z" if offset.lower() == "utc" else offset)
    try:
        moment = isoparse(text_value)
    except (ValueError, OverflowError) as error:
        reason = " (%s)" % error if _RANGE_MESSAGE_PATTERN.search(str(error)) else ""
        raise BoundError("cannot read %r as a time%s" % (text, reason)) from None
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=UTC)
    moment = moment.astimezone(UTC)
    time_match = _TIME_PATTERN.search(text_value)
    if time_match:
        _hour, minute, second, fraction = time_match.groups()
        if fraction:
            return format_utc(moment)
        if second:
            return _bound(moment, timedelta(seconds=1), end)
        if minute:
            return _bound(moment, timedelta(minutes=1), end)
        return _bound(moment, timedelta(hours=1), end)
    if _YEAR_PATTERN.match(text_value):
        return _bound(moment, relativedelta(years=1), end)
    if _MONTH_PATTERN.match(text_value):
        return _bound(moment, relativedelta(months=1), end)
    return _bound(moment, timedelta(days=1), end)


def _bound(start: datetime, length, end: bool) -> str:
    """The first millisecond of the period, or with `end` its last."""
    if end:
        return format_utc(start + length - timedelta(milliseconds=1))
    return format_utc(start)


@dataclass
class RowFilter:
    """The time window and patterns of `timeline`, with counts of the rows each dropped."""

    since: str = ""
    until: str = ""
    keep_undated: bool = False
    patterns: list[re.Pattern] = field(default_factory=list)
    dropped_window: int = 0
    dropped_undated: int = 0
    dropped_match: int = 0

    @property
    def active(self) -> bool:
        """Whether a window or a pattern is set."""
        return bool(self.since or self.until or self.patterns)

    def in_window(self, row: Row) -> bool:
        """Return whether a row is inside the window, counting it when it is not."""
        if not (self.since or self.until):
            return True
        timestamp = row.timestamp_utc
        if not timestamp:
            if not self.keep_undated:
                self.dropped_undated += 1
            return self.keep_undated
        if (self.since and timestamp < self.since) or (self.until and timestamp > self.until):
            self.dropped_window += 1
            return False
        return True

    def apply(self, rows: list[Row]) -> list[Row]:
        """Return the rows inside the window that match a pattern.

        A matched `tool_use` keeps its `tool_result`, and a matched result its
        call, by session and `tool_use_id`, when the partner is inside the
        window too.
        """
        kept = [row for row in rows if self.in_window(row)]
        if not self.patterns:
            return kept
        hit = [any(pattern.search(row.text) for pattern in self.patterns) for row in kept]
        pairs = {
            (row.session_id, row.source_file if not row.session_id else "", row.tool_use_id)
            for row, is_hit in zip(kept, hit, strict=True)
            if is_hit and row.tool_use_id
        }
        out = []
        for row, is_hit in zip(kept, hit, strict=True):
            key = (row.session_id, row.source_file if not row.session_id else "", row.tool_use_id)
            if is_hit or (row.tool_use_id and key in pairs):
                out.append(row)
            else:
                self.dropped_match += 1
        return out
