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


def build(output: pathlib.Path, matrix: pathlib.Path | None = None,
          roles: pathlib.Path | None = None) -> subprocess.CompletedProcess[str]:
    env = os.environ.copy()
    if matrix is not None:
        env["MOODLE_ROLE_MATRIX_REPORT"] = str(matrix)
    if roles is not None:
        env["MOODLE_RUNTIME_ROLES_DIR"] = str(roles)
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
        require(all(row["not_run"] > 0 for row in placeholders),
                "not-run placeholders do not expose their function denominator")

        roles_dir = temp / "roles"
        roles_dir.mkdir()
        discovered = [
            {"shortname": name, "is_site_administrator": name == "site_administrator"}
            for name in [
                "manager", "coursecreator", "editingteacher", "teacher", "student",
                "guest", "user", "frontpage", "matrixteacher", "matrixblank",
                "plugin_reviewer", "site_administrator",
            ]
        ]
        (roles_dir / "runtime-roles-v52.json").write_text(json.dumps({
            "schema_version": 1,
            "moodle": {"release": "5.2.3", "version": "2025041403"},
            "runtime_role_count": 11,
            "principal_count": 12,
            "roles": discovered,
        }))
        dynamic_output = temp / "dynamic.json"
        result = build(dynamic_output, temp / "absent.jsonl", roles_dir)
        require(result.returncode == 0, result.stderr or result.stdout)
        dynamic = json.loads(dynamic_output.read_text())
        dynamic_rows = dynamic["queries"]["roles"]["rows"]
        require(len(dynamic_rows) == 34,
                f"runtime inventory did not replace the v52 fallback: {len(dynamic_rows)}")
        require(any(row["version"] == "v52" and row["role"] == "plugin_reviewer"
                    for row in dynamic_rows),
                "runtime plugin role is absent from dashboard placeholders")
        require(dynamic["queries"]["runs"]["rows"][0]["runtime_roles"] == {"v52": 12},
                "run provenance did not record the runtime principal count")

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
        result = build(observed_output, matrix, roles_dir)
        require(result.returncode == 0, result.stderr or result.stdout)
        observed = json.loads(observed_output.read_text())
        require(observed["queries"]["runs"]["rows"][0]["role_matrix"] == "partial",
                "partial execution evidence was not marked partial")
        rows = observed["queries"]["roles"]["rows"]
        course = next(row for row in rows
                      if row["role"] == "student" and row["domain"] == "core_course")
        require(course["expected_denied"] == 1 and course["status"] == "partial"
                and course["not_run"] > 0,
                "expected denial or the unexecuted domain denominator was lost")
        require(sum(row["passed"] + row["expected_denied"]
                    + row["expected_unavailable"] + row["failed"] for row in rows) == 3,
                "the report did not preserve the exact executed-cell numerator")
        require(sum(row["not_run"] for row in rows) > 25000,
                "partial evidence hid the unexecuted role/function cells")

        unknown_role = temp / "unknown-role.jsonl"
        unknown_role.write_text(json.dumps({
            "version": "v52", "role": "not_installed",
            "function": "core_webservice_get_site_info", "outcome": "passed",
        }) + "\n")
        result = build(temp / "unknown-role.json", unknown_role, roles_dir)
        require(result.returncode != 0 and "is not in" in result.stderr,
                "the report builder accepted evidence for a role absent from runtime inventory")

        invalid = temp / "invalid.jsonl"
        invalid.write_text(json.dumps({
            "version": "v52", "role": "student",
            "function": "core_webservice_get_site_info", "outcome": "skip",
        }) + "\n")
        result = build(temp / "invalid.json", invalid)
        require(result.returncode != 0 and "skip is deliberately not" in result.stderr,
                "the report builder accepted a skipped role/function cell")

        duplicate = temp / "duplicate.jsonl"
        duplicate.write_text("".join(json.dumps(cells[0]) + "\n" for _ in range(2)))
        result = build(temp / "duplicate.json", duplicate, roles_dir)
        require(result.returncode != 0 and "duplicate role/function cell" in result.stderr,
                "the report builder double-counted a repeated role/function cell")

    print("report evidence contract passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
