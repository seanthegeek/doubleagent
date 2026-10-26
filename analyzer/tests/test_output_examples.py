import subprocess
import sys
import unittest
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
REGENERATOR = REPO / "analyzer" / "tests" / "regenerate_output_examples.py"


class OutputExamplesTests(unittest.TestCase):
    def test_documented_examples_match_collected_fixture(self):
        result = subprocess.run(
            [sys.executable, str(REGENERATOR), "--check"],
            cwd=REPO,
            check=False,
            encoding="utf-8",
            capture_output=True,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
