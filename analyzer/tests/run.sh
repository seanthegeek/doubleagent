#!/bin/sh
# Run the analyzer test suite with pytest from analyzer/, where pyproject.toml
# holds its settings. Arguments go to pytest: node ids such as
# tests/test_cli.py::DetectTests, and options such as -k or -x. With pytest-xdist
# installed the suite runs on every core; a later -n 0 among the arguments runs it
# in one process.
cd "$(dirname "$0")/.." || exit 2
if python3 -c 'import xdist' 2>/dev/null; then
  set -- -n auto --dist loadscope "$@"
fi
exec python3 -m pytest "$@"
