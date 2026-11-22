# Filtering the timeline

`doubleagent timeline` can write only part of the timeline, which keeps the
output of a busy host to the incident window or to the events that mention
what you are looking for. These options choose the rows:

| Option | Keeps |
| --- | --- |
| `--since WHEN`, `--until WHEN` | Rows with `since <= timestamp_utc <= until`. Either may be given alone. |
| `--keep-undated` | With a window, also the rows that have no timestamp. |
| `--match REGEX`, `-i` | Rows whose `text` matches one of the patterns, with the rows that pair with them. |
| `--agent AGENT` | Only the rows of the parsers of this agent. |
| `--include-thinking` | Adds the agents' thinking and reasoning blocks as `thinking` rows, which are left out otherwise. |

There is no option that selects by user or host: `--user` and `--host`
only name what the input cannot say ([Inputs and detection](inputs.md)).
Select those afterwards, for example with
`jq -c 'select(.user == "bob")' timeline.jsonl`
([Reading and searching the output](searching.md)).

## The order filters apply in

1. Only the parsers of the `--agent` names run, and thinking blocks become
   rows only with `--include-thinking`.
2. The rows of secondary sources (files that repeat what an agent's main
   store records, such as a prompt history) that repeat an event already in
   the timeline are dropped ([Secondary sources](timeline-output.md#secondary-sources)).
   This happens before the window, so the `dropped:` line counts the
   duplicates of the whole input, and a copy outside the window still
   stands for its event.
3. The rows are sorted by time.
4. The time window keeps the rows inside it; without `--keep-undated`, it
   drops every row without a timestamp.
5. `--match` searches the rows the window kept. A matched row that has a
   `tool_use_id` keeps every row inside the window with the same
   `session_id` and `tool_use_id` (the same `source_file`, for rows
   without a session id): its call, its result and the rows beside them,
   such as a `permission decision:` row.
6. Each long text that several rows name is written in full in the first
   row kept that names it, so a filtered timeline holds every text its
   pointers name ([timeline.jsonl](timeline-output.md#timelinejsonl)).
7. `sessions.jsonl` is summarised from the rows kept.

So `sessions.jsonl` describes the filtered timeline, not the whole of each
session: its counts and its first and last timestamps are those of the
rows kept, and a session none of whose rows was kept is not listed. The
row counts the summary prints count the rows written.

The rows of an id a store repeats, which only damage or an edit writes,
are shown once, with the attributes of the first stored row of the id that
the window and the patterns keep, so a row of the id outside the window
does not take the other rows of the id with it
([timeline.jsonl](timeline-output.md#timelinejsonl), under "Rows of a
repeated id").

A value of `--since` or `--until` that is not one of the forms below, a
`--since` later than the `--until`, a `--match` that is not a valid
pattern, and an `--agent` that no parser reads stop the command with exit
code `2` before the input is opened or extracted. A filter that keeps no
row is not an error: the three files are written, and the command exits
`1`.

The summary on stdout records what was applied, with the window resolved
to UTC, so a relative value such as `3d` can be checked and written in the
case notes. Over the example collection with
`--since 2026-10-01 --until 2026-10-03`:

```text
window:    2026-10-01T00:00:00.000Z to 2026-10-03T23:59:59.999Z
filtered:  10 outside the window, 11 without a timestamp, 0 not matching
```

and with `--match 'README|pytest' -i`:

```text
match:     README|pytest (ignore case)
filtered:  0 outside the window, 0 without a timestamp, 323 not matching
```

A bound left out prints as `(start)` or `(end)`, and `--keep-undated` adds
`, undated rows kept` to the `window:` line.

## Rows without a timestamp

A row has an empty `timestamp_utc` when its record carries no time:
some state records, and secondary sources that keep no time of their own,
whose rows are never given the session's time. A store that records no time
per message gives its rows the session's time instead, so those rows are
kept or dropped by when the session started; the agent's research page
says which stores do. Once `--since` or `--until` is given, undated rows
are dropped and counted as `without a timestamp`, unless `--keep-undated`
is passed.

## Matching text

`--match` takes a Go regular expression, in the RE2 syntax described on
[syntax package - regexp/syntax](https://pkg.go.dev/regexp/syntax). The
pattern is searched anywhere in the row's `text`, so anchor it with `^`
and `$` for a whole match; `^` and `$` mark the start and end of the whole
text, which may hold several lines, unless the pattern starts with `(?m)`.
`.` does not match a line break unless the pattern starts with `(?s)`.
Repeat `--match` to keep rows that match any of the patterns; `-i` makes
every pattern ignore case, as `(?i)` at its start would.

RE2 has no backreferences (`\1`) and no lookaround (`(?=...)`,
`(?!...)`, `(?<=...)`, `(?<!...)`), and refuses a pattern that uses them:

```text
error: --match "(a)\\1": error parsing regexp: invalid escape sequence: `\1`
```

Only `text` is searched. A tool call's text is its most identifying
argument (the command, the file, the URL), so `--match 'rm -rf'` finds the
call and keeps its result beside it. Filter other fields with jq
afterwards. The text searched is the evidence as written: under
`--no-secrets` a pattern still matches a secret that the output then
shows as the placeholder.

### The cost of --match

A pattern is searched in each row's full text, including a long text that
several rows share, such as a paste that several prompt-history lines name,
although the output writes that text once and a pointer in the later rows
([timeline.jsonl](timeline-output.md#timelinejsonl), under "A long text
that many rows name"). Many rows naming one long text therefore cost that
text once per row. This is a deliberate choice: a pattern can then match
across the boundary between a row's own words and the shared text, and
whether a row is kept does not depend on which row holds the text. The
time to match is linear in the length of the text searched, so no pattern
can make a search take longer than that.

## WHEN forms

`WHEN` is one of the following. Surrounding whitespace is removed and each
run of whitespace inside read as one space. The day words, the units,
`ago`, `Z` and `UTC` may be in any case; the `T` between date and time is
a capital. Nothing else is accepted, and nothing is guessed. A value
without an offset is UTC, like the timeline.

| Form | Examples | Meaning |
| --- | --- | --- |
| ISO 8601 date | `2026-10-01`, basic `20261001`, week date `2026-W40-4` or `2026W404`, ordinal date `2026-274` or `2026274` | That day. A week date without its day, `2026-W40`, is the Monday of that week. |
| Year or month | `2026`, `2026-10` | That year or month. |
| Date and time, `T` or one space between them | `2026-10-01T09`, `2026-10-01T09:30`, `2026-10-01 09:30`, `2026-10-01 09:30:15`, `2026-10-01T09:30:15.250`, basic `20261001T093015` | Seconds are optional and may carry a fraction after a point. |
| Date and time with a UTC offset, directly after the time or after one space | `2026-10-01T09:30Z`, `2026-10-01T09:30+02:00`, `2026-10-01T09:30-0500`, `2026-10-01T09:30+02`, `2026-10-01 09:30 +02:00`, `2026-10-01 09:30 UTC` | Offsets are `Z`, `UTC` (after a space only), `+HH:MM`, `+HHMM` or `+HH`; `-` is west of UTC. The time is converted to UTC. |
| Epoch number | `1759312800`, `1759312800000`, `1759312800.5` | Seconds since 1970, or milliseconds when the number is above 10^11. At least nine digits, so an eight-digit number is a basic date. |
| Day word | `today`, `yesterday` | That UTC calendar day. |
| `now` | `now` | The current time. |
| Relative time | `45m`, `45min`, `45 minutes`, `36h`, `36 hours`, `3d`, `3 days ago`, `2w`, `6mo`, `6 months ago`, `1y`, `2 years` | That long before now. The space and a trailing `ago` are optional; units are `m`, `min`, `mins`, `minute`, `minutes`; `h`, `hour`, `hours`; `d`, `day`, `days`; `w`, `week`, `weeks`; `mo`, `month`, `months`; `y`, `year`, `years`. `m` is always minutes and `mo` always months. Months and years move the calendar date and keep the time of day; a day the target month lacks becomes its last day, so 31 March less `1mo` is 28 February (29 in a leap year). |

Refused, among others: a lower-case `t`, an offset straight after a date
with no time, a comma before the fraction, `24:00`, one-digit fields and
digits other than ASCII ones. A refused value stops the command with a
message that ends with the accepted forms:

```text
error: cannot read "last tuesday" as a time; accepted: ISO 8601 such as 2026-10-01, 2026-10-01T09:30[:SS][Z|+HH:MM] or 2026-10-01 09:30[:SS] [+HH:MM|UTC]; 2026-10 or 2026; epoch seconds or milliseconds; today, yesterday, now; or a relative time such as 45m, 36h, 3d, 2w, 6mo, 1y, optionally followed by 'ago'
```

## Periods and instants

An absolute value or a day word names a period as long as its last
written unit: a year, a month, a day, an hour (`T09`), a minute (`09:30`)
or a second (`09:30:15`). `--since` takes the period's first millisecond
and `--until` its last, so `--since 2026-10-01 --until 2026-10-01` selects
that whole day, `--until 2026-10` runs to the end of October, and
`--until "2026-10-01 09:30"` keeps an event at 09:30:40. A time with a
fraction of a second, `now`, a relative time and an epoch number are
instants, used as they are. Each bound reads the clock once, when the
command starts.
