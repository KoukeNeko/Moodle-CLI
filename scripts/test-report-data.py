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
          roles: pathlib.Path | None = None,
          preflight: pathlib.Path | None = None,
          evidence_run: str | None = None,
          supplemental_dir: pathlib.Path | None = None) -> subprocess.CompletedProcess[str]:
    env = os.environ.copy()
    if matrix is not None:
        env["MOODLE_ROLE_MATRIX_REPORT"] = str(matrix)
    if roles is not None:
        env["MOODLE_RUNTIME_ROLES_DIR"] = str(roles)
    if preflight is not None:
        env["MOODLE_ROLE_PREFLIGHT_DIR"] = str(preflight)
    if evidence_run is not None:
        env["MOODLE_EVIDENCE_RUN_ID"] = evidence_run
        env["MOODLE_EVIDENCE_SHA"] = "a" * 40
    if supplemental_dir is not None:
        env["MOODLE_ROLE_MATRIX_DIR"] = str(supplemental_dir)
        env["MOODLE_SUPPLEMENTAL_RUN_ID"] = "789"
        env["MOODLE_SUPPLEMENTAL_SHA"] = "b" * 40
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
        require(all(row["recipe"] == "not-implemented"
                    for row in default["queries"]["functions"]["rows"]),
                "generated parameter schemas were misreported as executable recipes")
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

        preflight_dir = temp / "preflight"
        preflight_dir.mkdir()
        preflight_rows = [
            {"schema_version": 1, "version": "v52", "role": "student",
             "credential": "issued", "function": "core_webservice_get_site_info",
             "outcome": "passed", "cli_exit": 0},
            {"schema_version": 1, "version": "v52", "role": "guest",
             "credential": "unavailable", "function": "core_webservice_get_site_info",
             "outcome": "expected_unavailable", "cli_exit": 4},
        ]
        preflight_file = preflight_dir / "role-preflight-v52.jsonl"
        preflight_file.write_text("".join(json.dumps(row) + "\n" for row in preflight_rows))
        preflight_output = temp / "preflight.json"
        result = build(preflight_output, roles=roles_dir, preflight=preflight_dir,
                       evidence_run="123456")
        require(result.returncode == 0, result.stderr or result.stdout)
        preflight_snapshot = json.loads(preflight_output.read_text())
        require(preflight_snapshot["queries"]["runs"]["rows"][0]["role_matrix"] == "partial",
                "credential preflight was misrepresented as complete matrix coverage")
        run = preflight_snapshot["queries"]["runs"]["rows"][0]
        require(run["run"] == "123456" and run["commit"] == "a" * 40
                and run["report_commit"] != "a" * 40,
                "republication misattributed evidence to the report renderer")
        require(sum(row["passed"] + row["expected_unavailable"]
                    for row in preflight_snapshot["queries"]["roles"]["rows"]) == 2,
                "executed preflight cells were not counted")
        preflight_function = next(row for row in preflight_snapshot["queries"]["functions"]["rows"]
                                  if row["version"] == "v52"
                                  and row["function"] == "core_webservice_get_site_info")
        require(preflight_function["recipe"] == "credential-preflight"
                and preflight_function["result"] == "executed",
                "actual preflight recipe was not attributed to its function")
        require(any("role-preflight-v52.jsonl" in source
                    for source in preflight_snapshot["queries"]["roles"]["source"]["tables"]),
                "role evidence lacks the preflight source path")

        service_fragment = preflight_dir / "role-matrix-service-v52.jsonl"
        service_row = {
            "version": "v52", "role": "student",
            "function": "auth_email_get_signup_settings",
            "outcome": "expected_unavailable", "recipe": "mobile-service-boundary",
            "cli_exit": 9,
        }
        service_fragment.write_text(json.dumps(service_row) + "\n")
        fragment_output = temp / "fragments.json"
        result = build(fragment_output, roles=roles_dir, preflight=preflight_dir)
        require(result.returncode == 0, result.stderr or result.stdout)
        fragment_snapshot = json.loads(fragment_output.read_text())
        require(sum(row["passed"] + row["expected_unavailable"]
                    for row in fragment_snapshot["queries"]["roles"]["rows"]) == 3,
                "service-boundary fragment was not merged with preflight evidence")
        fragment_function = next(row for row in fragment_snapshot["queries"]["functions"]["rows"]
                                 if row["version"] == "v52"
                                 and row["function"] == "auth_email_get_signup_settings")
        require(fragment_function["recipe"] == "mobile-service-boundary"
                and fragment_function["result"] == "executed",
                "executed service-boundary recipe was not attributed")

        service_fragment.write_text(json.dumps({
            "version": "v52", "role": "student",
            "function": "core_webservice_get_site_info",
            "outcome": "expected_unavailable", "recipe": "mobile-service-boundary",
            "cli_exit": 9,
        }) + "\n")
        result = build(temp / "invalid-service.json", roles=roles_dir,
                       preflight=preflight_dir)
        require(result.returncode != 0 and "invalid mobile-service boundary evidence" in result.stderr,
                "service fragment claimed an exposed function was unavailable")
        service_fragment.write_text(json.dumps(service_row) + "\n")

        supplemental_dir = temp / "supplemental"
        supplemental_dir.mkdir()
        (supplemental_dir / "role-matrix-service-v52.jsonl").write_text(json.dumps({
            "version": "v52", "role": "student",
            "function": "core_cohort_get_cohorts",
            "outcome": "expected_unavailable", "recipe": "mobile-service-boundary",
            "cli_exit": 9,
        }) + "\n")
        supplement_output = temp / "supplement.json"
        result = build(supplement_output, roles=roles_dir, preflight=preflight_dir,
                       supplemental_dir=supplemental_dir)
        require(result.returncode == 0, result.stderr or result.stdout)
        supplemental_snapshot = json.loads(supplement_output.read_text())
        require(sum(row["passed"] + row["expected_unavailable"]
                    for row in supplemental_snapshot["queries"]["roles"]["rows"]) == 4,
                "supplemental recipe fragment replaced rather than extended primary evidence")
        run = supplemental_snapshot["queries"]["runs"]["rows"][0]
        require(run["supplemental_run"] == "789" and run["supplemental_commit"] == "b" * 40,
                "supplemental evidence provenance is missing")

        preflight_rows[1]["cli_exit"] = 0
        preflight_file.write_text("".join(json.dumps(row) + "\n" for row in preflight_rows))
        result = build(temp / "bad-preflight.json", roles=roles_dir, preflight=preflight_dir)
        require(result.returncode != 0 and "preflight outcome/exit mismatch" in result.stderr,
                "malformed credential preflight was accepted as execution evidence")

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
