#!/usr/bin/env python3
"""Contract tests for the redacted dashboard evidence builder."""
from __future__ import annotations

import json
import os
import pathlib
import subprocess
import sys
import tempfile


ROOT = pathlib.Path(__file__).resolve().parents[1]
BUILDER = ROOT / "scripts/build-report-data.py"


def build(output: pathlib.Path, matrix: pathlib.Path | None = None) -> subprocess.CompletedProcess[str]:
    env = os.environ.copy()
    if matrix is not None:
        env["MOODLE_ROLE_MATRIX_REPORT"] = str(matrix)
    return subprocess.run(
        [sys.executable, str(BUILDER), str(output)],
        cwd=ROOT,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="moodle-report-test-") as directory:
        temp = pathlib.Path(directory)

        default_output = temp / "default.json"
        result = build(default_output, temp / "absent.jsonl")
        require(result.returncode == 0, result.stderr or result.stdout)
        default = json.loads(default_output.read_text())
        require(default["queries"]["runs"]["rows"][0]["role_matrix"] == "not-run",
                "missing role evidence was not reported as not-run")
        placeholders = default["queries"]["roles"]["rows"]
        require(len(placeholders) == 33 and all(row["status"] == "not-run" for row in placeholders),
                "the explicit 3-version role placeholders changed")

        matrix = temp / "role-matrix.jsonl"
        cells = [
            {"version": "v52", "role": "student",
             "function": "core_webservice_get_site_info", "outcome": "passed"},
            {"version": "v52", "role": "guest",
             "function": "core_webservice_get_site_info", "outcome": "expected_unavailable"},
            {"version": "v52", "role": "student",
             "function": "core_course_get_courses", "outcome": "expected_denied"},
        ]
        matrix.write_text("".join(json.dumps(cell) + "\n" for cell in cells))
        observed_output = temp / "observed.json"
        result = build(observed_output, matrix)
        require(result.returncode == 0, result.stderr or result.stdout)
        observed = json.loads(observed_output.read_text())
        require(observed["queries"]["runs"]["rows"][0]["role_matrix"] == "observed",
                "executed evidence was not marked observed")
        rows = observed["queries"]["roles"]["rows"]
        require(len(rows) == 3, f"expected three role/domain rows, got {rows!r}")
        course = next(row for row in rows
                      if row["role"] == "student" and row["domain"] == "core_course")
        require(course["expected_denied"] == 1 and course["status"] == "passed",
                "expected denial was not preserved as a successful security outcome")

        invalid = temp / "invalid.jsonl"
        invalid.write_text(json.dumps({
            "version": "v52", "role": "student",
            "function": "core_webservice_get_site_info", "outcome": "skip",
        }) + "\n")
        result = build(temp / "invalid.json", invalid)
        require(result.returncode != 0 and "skip is deliberately not" in result.stderr,
                "the report builder accepted a skipped role/function cell")

    print("report evidence contract passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
