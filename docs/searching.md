# Reading and searching the output

A timeline from a busy host can run to gigabytes, because `text` holds
full tool output and `doubleagent timeline` never shortens it. Cutting the
output down to the rows and fields a question needs is the analyst's step,
done with tools already on the workstation. Each line of `timeline.jsonl`
and `sessions.jsonl` is one complete JSON record, so jq, grep, ripgrep and
DuckDB stream the file line by line and use little memory however large it
is. The fields are described in [timeline.jsonl](timeline-output.md#timelinejsonl)
and [sessions.jsonl](timeline-output.md#sessionsjsonl).

The examples run in the output directory of `timeline`, here over the
example collection ([The example collection](timeline-output.md#the-example-collection)).
A spreadsheet is the wrong viewer for these files: Excel stops at
1,048,576 rows and 32,767 characters per cell. Export a filtered, shortened
subset to CSV instead, as the last jq example does.

## jq

[jq](https://jqlang.org) reads one record at a time. Never pass `-s`
(slurp) on a large file, which loads the whole file into memory.

```sh
# Look at one record, pretty-printed
head -n 1 timeline.jsonl | jq .

# Record counts by agent and turn type
jq -r '[.agent, .turn_type] | @tsv' timeline.jsonl | sort | uniq -c | sort -rn

# Tool calls of one tool, with a few fields
jq -c 'select(.turn_type == "tool_use" and .tool_name == "Bash") | {timestamp_utc, user, text}' timeline.jsonl

# Case-insensitive regular expression over the text
jq -c 'select(.text | test("curl|wget|base64 -d"; "i")) | [.timestamp_utc, .agent, .turn_type, .text[0:120]]' timeline.jsonl

# One session as a readable transcript; line breaks in text are printed as line breaks
jq -r --arg s 11111111-2222-4333-8444-555555555555 'select(.session_id == $s) | "\(.timestamp_utc)  \(.turn_type)  \(.tool_name)\n\(.text)\n"' timeline.jsonl | less

# A tool call and its result. Ids such as call_1 repeat across agents and
# sessions, so match the session too
jq -c --arg s 11111111-2222-4333-8444-555555555555 --arg id toolu_01 'select(.session_id == $s and .tool_use_id == $id) | {turn_type, text}' timeline.jsonl

# Every per-call permission decision, with the call it belongs to
jq -r 'select(.turn_type == "system" and (.text | startswith("permission decision:"))) | [.timestamp_utc, .agent, .tool_use_id, .text] | @tsv' timeline.jsonl

# Rows recovered from a second copy, which the agent's main store no longer holds
jq -c 'select(.recovered == "yes") | {timestamp_utc, agent, turn_type, source_file}' timeline.jsonl

# Sessions with more than 3 tool calls, and their models
jq -r 'select(.tool_calls > 3) | [.first_timestamp_utc, .user, .agent, .session_id, .tool_calls, (.models | join(" "))] | @tsv' sessions.jsonl

# A CSV for a spreadsheet: filter first, and cut text to 200 characters
jq -r 'select(.agent == "codex-cli") | [.timestamp_utc, .host, .user, .agent, .session_id, .turn_type, .tool_name, .text[0:200]] | @csv' timeline.jsonl > codex.csv
```

jq's `test` uses jq's own regular expressions, which differ from the Go
regular expressions `timeline --match` takes ([Matching text](filtering.md#matching-text)):
jq has lookaround and backreferences, for instance.

## grep first, then jq

`grep -F` and ripgrep's `rg -F` scan text many times faster than jq parses
JSON, so use them to cut a large file down and jq to check the field. A
grep match can come from any field, so always confirm with a `select` on
the field you meant. Write patterns as the record stores them:
`"turn_type":"tool_use"` without spaces, a quote inside text as `\"`, a
backslash as `\\`, and a line break as the two characters `\n`.

```sh
grep -F '"turn_type":"tool_use"' timeline.jsonl | jq -r .tool_name | sort | uniq -c | sort -rn
grep -F 'rm -rf' timeline.jsonl | jq -c 'select(.text | contains("rm -rf")) | {timestamp_utc, user, agent, source_file, source_line}'
```

A long text that several rows name, such as a paste that several history
lines repeat, is written in full only in the first row that names it; the
later rows hold a pointer to that row's `source_file` and `source_line`
instead ([timeline.jsonl](timeline-output.md#timelinejsonl), under "A long
text that many rows name"). A grep for words inside such a text finds the
first row only; `timeline --match` reads every row with the text in full.

## Split or compress

One file per agent keeps each one small enough for an editor, and gzip
shrinks transcripts severalfold; `gzip -dc` feeds jq, and `rg -z` searches
the compressed file directly.

```sh
mkdir -p by-agent
jq -r .agent timeline.jsonl | sort -u | while read -r agent; do
  jq -c --arg agent "$agent" 'select(.agent == $agent)' timeline.jsonl > "by-agent/$agent.jsonl"
done
gzip -k timeline.jsonl
gzip -dc timeline.jsonl.gz | jq -c 'select(.agent == "codex-cli")'
rg -z -F 'crontab' timeline.jsonl.gz | jq -c '{timestamp_utc, agent, text}'
```

## DuckDB

[DuckDB](https://duckdb.org) runs SQL over the file without an import
step. `COPY` writes a filtered subset as CSV or Parquet.

```sql
SELECT agent, turn_type, count(*) AS n
FROM read_json_auto('timeline.jsonl') GROUP BY ALL ORDER BY n DESC;

SELECT timestamp_utc, user, agent, left(text, 120)
FROM read_json_auto('timeline.jsonl')
WHERE turn_type = 'tool_use' AND text ILIKE '%rm -rf%'
ORDER BY timestamp_utc;

COPY (SELECT * FROM read_json_auto('timeline.jsonl') WHERE agent = 'codex-cli')
TO 'codex.csv' (HEADER);
```

`read_json_auto` guesses each column's type from the records it samples;
`DESCRIBE SELECT * FROM read_json_auto('timeline.jsonl');` shows what it
chose. Over the example it reads every field as text except `source_line`.

## Filtering by time afterwards

`timestamp_utc` always has one fixed form, `2026-10-03T16:55:31.123Z`, so
plain string comparison puts times in the right order and needs no date
parsing in any tool. The `--since` and `--until` options
([Filtering the timeline](filtering.md)) do this while writing the
timeline; these recipes do it on a timeline already written.

```sh
# Everything on 1 October 2026 (UTC)
jq -c 'select(.timestamp_utc >= "2026-10-01" and .timestamp_utc < "2026-10-02")' timeline.jsonl

# A window to the second
jq -c 'select(.timestamp_utc >= "2026-10-01T10:00:00.000Z" and .timestamp_utc < "2026-10-01T10:05:00.000Z")' timeline.jsonl

# Sessions that overlap a day
jq -c 'select(.last_timestamp_utc >= "2026-10-01" and .first_timestamp_utc < "2026-10-02")' sessions.jsonl

# Records without a timestamp
jq -c 'select(.timestamp_utc == "")' timeline.jsonl
```

```sql
SELECT * FROM read_json_auto('timeline.jsonl')
WHERE timestamp_utc >= '2026-10-01' AND timestamp_utc < '2026-10-02';
```

- Write a bound either as a date alone (`"2026-10-01"`) or in the full
  form with milliseconds and `Z`. `"2026-10-01T10:00:00Z"` without `.000`
  drops the records at exactly 10:00:00, because `.` sorts before `Z`.
- End a range with "before the next day", `< "2026-10-02"`.
  `<= "2026-10-01"` keeps almost nothing from that day, since every time
  on it sorts after the bare date.
- Records without a timestamp hold `""`, which never passes a lower bound,
  so they drop out. Add `or .timestamp_utc == ""` to keep them.
- Convert a local incident window to UTC before writing the bounds.
