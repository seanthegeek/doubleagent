"""Put `lab/round/` on the import path, so a test imports a tool as round.sh runs it."""

from __future__ import annotations

import pathlib
import sys

ROUND = pathlib.Path(__file__).resolve().parent.parent
if str(ROUND) not in sys.path:
    sys.path.insert(0, str(ROUND))
