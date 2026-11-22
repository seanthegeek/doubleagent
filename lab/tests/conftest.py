"""Put `lab/` on the import path, so a test imports a tool as lab.sh runs it."""

from __future__ import annotations

import pathlib
import sys

LAB = pathlib.Path(__file__).resolve().parent.parent
if str(LAB) not in sys.path:
    sys.path.insert(0, str(LAB))
