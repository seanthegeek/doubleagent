"""Write dedup_cases.jsonl: the deduplications the Gemini CLI parser's Python tests run.

Run from the repository root with the analyzer's interpreter:

    analyzer/.venv/bin/python -I internal/parser/geminicli/testdata/dedup_cases.py

It runs the tests named in `TESTS` with the analyzer's `deduplicate` wrapped, and writes one
line per call: the test, the primary rows that share a match form or a tool_use_id with a
secondary row (no other primary row can take one), the secondary rows with their tolerance,
the indexes of the secondary rows kept, in order, and the rows dropped per agent. Empty row
fields are left out.
"""

import json
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path("analyzer").resolve()))
sys.path.insert(0, str(Path("analyzer/tests").resolve()))

import doubleagent.parsers  # noqa: F401  (fills the deduplicator's hook tables)
from doubleagent import dedup
from doubleagent.model import TIMELINE_COLUMNS
from parsers import test_gemini_cli as module

TESTS = (
    "test_chat_checkpoint",
    "test_tool_checkpoint_of_another_session_pairs_with_nothing",
    "test_error_report_of_another_project_is_not_dropped",
    "test_equal_copies_across_files_are_kept_once",
    "test_relocated_project_still_pairs_on_the_session",
    "test_context_row_the_session_file_lacks_is_kept_unrecovered",
    "test_tool_checkpoint_rows",
    "test_error_report_rows",
)
OUTPUT = Path(__file__).with_name("dedup_cases.jsonl")


def fields(row):
    """Return a row's non-empty columns, keeping `source_line` always."""
    values = {name: getattr(row, name) for name in TIMELINE_COLUMNS}
    return {key: value for key, value in values.items() if value or key == "source_line"}


def form(row):
    """Return the turn type and text a row is matched under."""
    hook = dedup.MATCH_FORMS.get(row.agent)
    return hook(row) if hook else (row.turn_type, row.text)


def inputs(test, primary, secondary):
    """Return the arguments of one call as a JSON-ready dict, before the call marks any row."""
    forms = {form(row) for row, _ in secondary}
    ids = {row.tool_use_id for row, _ in secondary if row.tool_use_id}
    relevant = [row for row in primary if form(row) in forms or row.tool_use_id in ids]
    return {
        "test": test,
        "primary": [fields(row) for row in relevant],
        "secondary": [dict(fields(row), tolerance=tolerance) for row, tolerance in secondary],
    }


def main():
    """Run the tests and write the cases."""
    calls = []
    current = [""]
    original = module.deduplicate

    def recording(primary, secondary, *args):
        call = inputs(current[0], primary, secondary)
        kept, dropped = original(primary, secondary, *args)
        positions = {id(row): index for index, (row, _) in enumerate(secondary)}
        call["kept"] = [positions[id(row)] for row in kept[len(primary) :]]
        call["dropped"] = dict(sorted(dropped.items()))
        calls.append(call)
        return kept, dropped

    module.deduplicate = recording
    for name in TESTS:
        current[0] = name
        suite = unittest.TestSuite([module.GeminiCliTests(name)])
        if not unittest.TextTestRunner(stream=sys.stderr, verbosity=0).run(suite).wasSuccessful():
            raise SystemExit("failed: " + name)
    with OUTPUT.open("w", encoding="utf-8", newline="\n") as handle:
        for call in calls:
            handle.write(json.dumps(call, ensure_ascii=False, sort_keys=True) + "\n")


main()
