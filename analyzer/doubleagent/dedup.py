"""Drop rows from secondary sources that a primary store also records.

A secondary source is a file a parser reads beside the agent's own
transcript store that repeats some of its events: a per-prompt history, a
mirror or a projection (`Parser.secondary`). Every source is parsed on every
run; this module keeps each event once. A secondary row is dropped when a
primary row has the same identity (host, user, agent, turn type and text, or
the same `tool_use_id` for two tool rows that both carry one), the same scope
(session, else project, else none) and a time within the source's tolerance
(`Parser.secondary_tolerance`). A secondary row with no time matches on text
and scope alone. Matching is one-to-one, timestamped rows first.

The secondary rows no primary row took are then matched against each other,
looking under broader scopes too, so the several copies of one event that
backups and mirrors keep are shown once. A row pairs only with a row from a
different `source_file`, and a `parser:` row is never dropped or taken.

An agent can register a match form (`MATCH_FORMS`) when its secondary
source writes an event in another form than its primary store, and a wider
forward window for some rows (`MATCH_TOLERANCES`), bounded by the next
secondary row of the same text (`_Bounds`), and a time a row is matched at
in place of its own (`MATCH_TIMES`). Every secondary row kept that is
not a `system` row is marked `recovered` "yes". The full rule is in
`analyzer/docs/output.md` under "Secondary sources".
"""

from __future__ import annotations

from bisect import bisect_left, bisect_right, insort
from collections import Counter
from collections.abc import Callable, Container

from .model import Row
from .timeutil import epoch_ms

# Codex writes its history `ts` in whole seconds beside the millisecond
# times of its rollouts, and Antigravity's history line and database step for
# one prompt differ by under a second.
TOLERANCE_SECONDS = 2.0

# The match form per agent: a function of a row that returns the
# `(turn_type, text)` the row is matched under in place of its own. A
# parser registers one when its secondary source records an event in
# another form than its primary store does (Claude Code's history line
# `/model` for the transcript's tag-form `slash command:` row); the rows of
# every other agent match on their own `turn_type` and `text`.
MATCH_FORMS: dict[str, Callable[[Row], tuple[str, str]]] = {}
# The time window per agent: a function of a secondary row that returns a
# wider window in seconds for that row, or None for the source's own. A
# parser registers one when some of its secondary rows record an event
# much earlier than the primary store does (a Claude Code slash command
# typed before the picker it opens is closed and recorded).
MATCH_TOLERANCES: dict[str, Callable[[Row], float | None]] = {}
# The match time per agent: a function of a row that returns the epoch
# milliseconds the row is matched at in place of its own time, `NO_MATCH`
# for a row that pairs with no other, or None for its own time. A parser
# registers one when its secondary source records the moment the primary
# store copied an event apart from the event's own time (a Qwen Code group
# message and the `clear` written when a prompt quoted it).
MATCH_TIMES: dict[str, Callable[[Row], int | None]] = {}
NO_MATCH = -1

_Dated = list[tuple[int, int]]  # (epoch ms, row index), sorted
_Upper = Callable[[int], int | None]  # a candidate's row index -> its bound (epoch ms)

_TOOL_TYPES = ("tool_use", "tool_result")


class _Bucket:
    """The candidates of one match key: the dated ones sorted by time, the rest in input order."""

    __slots__ = ("dated", "undated")

    def __init__(self) -> None:
        self.dated: _Dated = []
        self.undated: list[int] = []


def _has_id(row: Row) -> bool:
    """Return whether a row is a tool call or result with a `tool_use_id`."""
    return row.turn_type in _TOOL_TYPES and bool(row.tool_use_id)


def _match_time(row: Row) -> int | None:
    """Return the epoch milliseconds a row is matched at, `NO_MATCH`, or None when undated."""
    hook = MATCH_TIMES.get(row.agent)
    anchor = hook(row) if hook else None
    return anchor if anchor is not None else epoch_ms(row.timestamp_utc)


def _by_text(row: Row) -> tuple[str, ...]:
    form = MATCH_FORMS.get(row.agent)
    turn_type, text = form(row) if form else (row.turn_type, row.text)
    return (row.host, row.user, row.agent, turn_type, "text", text)


def _by_id(row: Row) -> tuple[str, ...]:
    return (row.host, row.user, row.agent, row.turn_type, "id", row.tool_use_id)


def _is_parser_row(row: Row) -> bool:
    """Return whether a row is a `parser:` row, which never pairs: it describes its own file."""
    return row.turn_type == "system" and row.text.startswith("parser:")


def _scope_rank(row: Row) -> int:
    """Return 0 for a row with a `session_id`, 1 for one with only a project, else 2."""
    if row.session_id:
        return 0
    return 1 if row.project_path else 2


def _lookups(row: Row, broaden: bool) -> list[tuple[tuple, bool, bool]]:
    """Return the keys a secondary row looks under, in order.

    Each key comes with whether it is a broader scope than the row's own and
    whether only candidates without a `tool_use_id` may be taken there. A row
    with an id looks under its id, then its text; the scope is the row's own
    and, with `broaden`, then its project and its identity alone.
    """
    scopes: list[tuple] = []
    if row.session_id:
        scopes.append(("session", row.session_id))
    if row.project_path:
        scopes.append(("project", row.project_path))
    scopes.append(("any",))
    if not broaden:
        scopes = scopes[:1]
    identities = (
        [(_by_id(row), False), (_by_text(row), True)] if _has_id(row) else [(_by_text(row), False)]
    )
    return [
        ((*identity, *scope), depth > 0, idless)
        for depth, scope in enumerate(scopes)
        for identity, idless in identities
    ]


def _add(buckets: dict[tuple, _Bucket], row: Row, index: int, time_ms: int | None) -> None:
    """File a candidate row under its text and id keys at every scope, keeping times sorted.

    A row at `NO_MATCH` is filed nowhere, so no row pairs with it.
    """
    if time_ms == NO_MATCH:
        return
    identities = [_by_text(row), _by_id(row)] if _has_id(row) else [_by_text(row)]
    for identity in identities:
        for scope in (("session", row.session_id), ("project", row.project_path), ("any",)):
            bucket = buckets.setdefault((*identity, *scope), _Bucket())
            if time_ms is None:
                bucket.undated.append(index)
            else:
                insort(bucket.dated, (time_ms, index))


def _nearest(
    dated: _Dated,
    time_ms: int,
    back_ms: int,
    forward_ms: int,
    used: Container[int],
    upper: _Upper | None = None,
) -> int | None:
    """Return the index of the unused candidate nearest in time, or None.

    The candidate is at most `back_ms` before `time_ms` or `forward_ms` after
    it, the earlier on a tie, and before the bound `upper` returns for it.
    """
    best: tuple[int, int] | None = None
    position = bisect_left(dated, (time_ms - back_ms, -1))
    while position < len(dated) and dated[position][0] <= time_ms + forward_ms:
        candidate_ms, index = dated[position]
        position += 1
        if index in used:
            continue
        if upper is not None:
            limit = upper(index)
            if limit is not None and candidate_ms >= limit:
                continue
        if best is None or abs(candidate_ms - time_ms) < best[0]:
            best = (abs(candidate_ms - time_ms), index)
    return best[1] if best else None


def _earliest(bucket: _Bucket, used: Container[int]) -> int | None:
    """Return the earliest unused candidate, the undated ones last, or None."""
    for _, index in bucket.dated:
        if index not in used:
            return index
    for index in bucket.undated:
        if index not in used:
            return index
    return None


def _pair(
    bucket: _Bucket | None,
    row: Row,
    tolerance: float,
    used: Container[int],
    upper: _Upper | None = None,
    forward: float | None = None,
) -> int | None:
    """Return the unused candidate in `bucket` a secondary row pairs with under the time rule."""
    if bucket is None or _match_time(row) == NO_MATCH:
        return None
    time_ms = _match_time(row)
    if time_ms is None:
        return _earliest(bucket, used)
    back_ms = int(tolerance * 1000)
    forward_ms = back_ms if forward is None else int(forward * 1000)
    return _nearest(bucket.dated, time_ms, back_ms, forward_ms, used, upper)


def _processing_order(secondary: list[tuple[Row, float]], by_scope: bool) -> list[int]:
    """Return the secondary row indices in the order the rule tests them.

    Timestamped rows go in time order, then undated rows in input order,
    grouped by scope with `by_scope`. `parser:` rows are left out.
    """
    stamped = []
    undated = []
    for index, (row, _) in enumerate(secondary):
        if _is_parser_row(row):
            continue
        time_ms = _match_time(row)
        if time_ms is None:
            undated.append(index)
        else:
            stamped.append((time_ms, index))
    if by_scope:
        undated.sort(key=lambda index: _scope_rank(secondary[index][0]))
    return [index for _, index in sorted(stamped)] + undated


class _Unavailable:
    """The candidates a row may not pair with.

    Those are the candidates already taken; with `same_file`, those from its
    own file; under an id-less key, those with a `tool_use_id`; and under a
    broader scope, those whose session or project differs from the row's.
    """

    def __init__(self, rows: list[Row], used: set[int], row: Row, same_file: bool) -> None:
        self.rows = rows
        self.used = used
        self.row = row
        self.same_file = same_file
        self.broad = False
        self.idless = False

    def __contains__(self, index: object) -> bool:
        if not isinstance(index, int) or index in self.used:
            return True
        candidate, row = self.rows[index], self.row
        if self.same_file and candidate.source_file == row.source_file:
            return True
        if self.idless and _has_id(candidate):
            return True
        if not self.broad:
            return False
        session_differs = bool(
            row.session_id and candidate.session_id and candidate.session_id != row.session_id
        )
        project_differs = bool(
            row.project_path
            and candidate.project_path
            and candidate.project_path != row.project_path
        )
        return session_differs or project_differs


def _find(
    buckets: dict[tuple, _Bucket],
    row: Row,
    tolerance: float,
    unavailable: _Unavailable,
    broaden: bool,
    upper: _Upper | None = None,
    forward: float | None = None,
) -> int | None:
    """Return the candidate a row pairs with, trying its lookups in order, or None."""
    for key, broad, idless in _lookups(row, broaden):
        unavailable.broad, unavailable.idless = broad, idless
        index = _pair(buckets.get(key), row, tolerance, unavailable, upper, forward)
        if index is not None:
            return index
    return None


def _scope(row: Row) -> tuple:
    """Return the scope a row matches under: its session, else its project, else none."""
    if row.session_id:
        return ("session", row.session_id)
    if row.project_path:
        return ("project", row.project_path)
    return ("any",)


class _Bounds:
    """The times of the secondary rows by match text and scope.

    A widened row may not take a primary row recorded at or after the next
    later secondary row of the same text that could take that same primary
    row, so the later line keeps the later record. Who could take it follows
    the candidate's scope; a row of another session never counts, and a copy
    at the same time does not bound.
    """

    __slots__ = ("by_none", "by_project_free", "by_session")

    def __init__(self, secondary: list[tuple[Row, float]]) -> None:
        self.by_session: dict[tuple, list[int]] = {}
        self.by_project_free: dict[tuple, list[int]] = {}
        self.by_none: dict[tuple, list[int]] = {}
        for row, _ in secondary:
            time_ms = _match_time(row)
            if time_ms in (None, NO_MATCH) or _is_parser_row(row):
                continue
            text = _by_text(row)
            if row.session_id:
                self.by_session.setdefault((text, row.session_id), []).append(time_ms)
            elif row.project_path:
                self.by_project_free.setdefault((text, row.project_path), []).append(time_ms)
            else:
                self.by_none.setdefault(text, []).append(time_ms)
        for index in (self.by_session, self.by_project_free, self.by_none):
            for times in index.values():
                times.sort()

    def upper(self, text: tuple, time_ms: int, candidate: Row) -> int | None:
        """Return the next time after `time_ms` of a row that could take `candidate`, or None."""
        lists = [self.by_none.get(text)]
        if candidate.session_id:
            lists.append(self.by_session.get((text, candidate.session_id)))
        if candidate.project_path:
            lists.append(self.by_project_free.get((text, candidate.project_path)))
        later: int | None = None
        for times in lists:
            if not times:
                continue
            position = bisect_right(times, time_ms)
            if position < len(times) and (later is None or times[position] < later):
                later = times[position]
        return later


def _candidate_bound(bounds: _Bounds, primary: list[Row], row: Row) -> _Upper | None:
    """Return the bound function of a widened row by candidate index, or None when undated."""
    time_ms = _match_time(row)
    if time_ms is None:
        return None
    text = _by_text(row)
    return lambda index: bounds.upper(text, time_ms, primary[index])


def _against_primary(
    primary: list[Row],
    secondary: list[tuple[Row, float]],
    bounds: _Bounds | None = None,
    forward: dict[int, float] | None = None,
) -> list[int]:
    """Return the indices of the secondary rows that pair with a primary row.

    Each row looks only under its own scope; a row in `forward` looks up to
    that many seconds after it, before the bound `bounds` gives.
    """
    buckets: dict[tuple, _Bucket] = {}
    for index, row in enumerate(primary):
        if not _is_parser_row(row):
            _add(buckets, row, index, _match_time(row))
    used: set[int] = set()
    matched: list[int] = []
    for secondary_index in _processing_order(secondary, by_scope=True):
        row, tolerance = secondary[secondary_index]
        window = forward.get(secondary_index) if forward else None
        upper = _candidate_bound(bounds, primary, row) if bounds and window is not None else None
        index = _find(
            buckets, row, tolerance, _Unavailable(primary, used, row, False), False, upper, window
        )
        if index is not None:
            used.add(index)
            matched.append(secondary_index)
    return matched


def _among_secondary(secondary: list[tuple[Row, float]], matched: set[int]) -> list[int]:
    """Return the indices of the untaken secondary rows that pair with another secondary row.

    A candidate is a row a primary row took or one tested before, from another
    file; each is taken once, and a row also looks under broader scopes.
    """
    rows = [row for row, _ in secondary]
    buckets: dict[tuple, _Bucket] = {}
    for index in sorted(matched):
        _add(buckets, rows[index], index, _match_time(rows[index]))
    used: set[int] = set()
    copies: list[int] = []
    for index in _processing_order(secondary, by_scope=False):
        if index in matched:
            continue
        row, tolerance = secondary[index]
        candidate_index = _find(buckets, row, tolerance, _Unavailable(rows, used, row, True), True)
        if candidate_index is not None:
            used.add(candidate_index)
            copies.append(index)
        _add(buckets, row, index, _match_time(row))
    return copies


def _forward_windows(secondary: list[tuple[Row, float]]) -> dict[int, float]:
    """Return the secondary rows whose agent's `MATCH_TOLERANCES` gives a wider window, with it."""
    windows: dict[int, float] = {}
    for index, (row, tolerance) in enumerate(secondary):
        window = MATCH_TOLERANCES.get(row.agent)
        wider = window(row) if window else None
        if wider is not None and wider > tolerance:
            windows[index] = wider
    return windows


def deduplicate(
    primary: list[Row], secondary: list[tuple[Row, float]]
) -> tuple[list[Row], Counter[str]]:
    """Return the rows to keep and the number of rows dropped per agent.

    The rows kept are every primary row, then the secondary rows with no
    match, in the order given; each secondary row comes with its tolerance in
    seconds. A secondary row kept that is not a `system` row gets `recovered`
    "yes" here.
    """
    forward = _forward_windows(secondary)
    bounds = _Bounds(secondary) if forward else None
    matched = set(_against_primary(primary, secondary, bounds, forward))
    dropped = matched | set(_among_secondary(secondary, matched))
    counts: Counter[str] = Counter(secondary[index][0].agent for index in dropped)
    survivors = [row for index, (row, _) in enumerate(secondary) if index not in dropped]
    for row in survivors:
        if row.turn_type != "system":
            row.recovered = "yes"
    return list(primary) + survivors, counts
