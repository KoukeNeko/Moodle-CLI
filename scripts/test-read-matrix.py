#!/usr/bin/env python3
"""Offline safety and result-contract tests for the exposed-read recipe."""
from __future__ import annotations

import importlib.util
import json
import pathlib
import subprocess


ROOT = pathlib.Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("moodle_read_matrix", ROOT / "test/e2e/read-matrix.py")
assert SPEC is not None and SPEC.loader is not None
module = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(module)


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def main() -> int:
    recipe = json.loads(module.RECIPE.read_text())
    course_recipe = json.loads(module.COURSE_RECIPE.read_text())
    for version in ("v45", "v51", "v52"):
        registry = json.loads((ROOT / f"internal/wsregistry/data/{version}.json").read_text())
        selected = module.selected_functions(registry, recipe)
        require([row["name"] for row in selected] == recipe["functions"],
                f"{version}: selected read recipes drifted")
        course_selected = module.selected_course_functions(registry, course_recipe)
        require([row["name"] for row, _ in course_selected]
                == sorted(course_recipe["functions"]),
                f"{version}: course read recipes drifted")
        require(module.course_params("courseid", 42) == {"courseid": 42}
                and module.course_params("options.ids", 42) == {"options": {"ids": [42]}},
                f"{version}: course fixture binding changed")
        for invalid in (0, 1, "42", None, True):
            try:
                module.course_params("courseid", invalid)
            except ValueError:
                pass
            else:
                raise AssertionError(f"{version}: invalid fixture course ID was accepted")
        course_function = next(row for row, _ in course_selected
                               if row["name"] == "core_course_get_courses")
        good_course = subprocess.CompletedProcess([], 0, stdout=json.dumps({
            "schema_version": 1, "kind": "ws.call", "data": {
                "function": course_function["name"], "version": version, "effect": "read",
                "dry_run": False, "response": [{"id": 42}],
            },
        }))
        require(module.classify(good_course, course_function, version,
                                course_id=42, role="site_administrator") == "passed",
                f"{version}: admin fixture course assertion rejected valid result")
        for response in ([], [{"id": 43}]):
            wrong_course = subprocess.CompletedProcess([], 0, stdout=json.dumps({
                "schema_version": 1, "kind": "ws.call", "data": {
                    "function": course_function["name"], "version": version,
                    "effect": "read", "dry_run": False, "response": response,
                },
            }))
            try:
                module.classify(wrong_course, course_function, version,
                                course_id=42, role="site_administrator")
            except ValueError:
                pass
            else:
                raise AssertionError(f"{version}: missing admin fixture course accepted")
        function = selected[0]
        response = {} if function["returns"]["type"] == "object" else []
        success = subprocess.CompletedProcess([], 0, stdout=json.dumps({
            "schema_version": 1, "kind": "ws.call", "data": {
                "function": function["name"], "version": version, "effect": "read",
                "dry_run": False, "response": response,
            },
        }))
        require(module.classify(success, function, version) == "passed",
                f"{version}: a real read success was rejected")
        for code, error_code, reason, expected in (
                (5, "permission_denied", "", "expected_denied"),
                (9, "unavailable", "capability", "expected_unavailable")):
            result = subprocess.CompletedProcess([], code, stdout=json.dumps({
                "schema_version": 1, "kind": "error", "error": {
                    "code": error_code, "reason": reason,
                },
            }))
            require(module.classify(result, function, version) == expected,
                    f"{version}: {expected} was rejected")
        for bad in (
                subprocess.CompletedProcess([], 0, stdout="not-json"),
                subprocess.CompletedProcess([], 0, stdout=json.dumps({
                    "schema_version": 1, "kind": "ws.call", "data": {
                        "function": function["name"], "version": version,
                        "effect": "write", "dry_run": False, "response": response,
                    },
                })),
                subprocess.CompletedProcess([], 11, stdout=json.dumps({
                    "schema_version": 1, "kind": "error", "error": {"code": "upstream"},
                }))):
            try:
                module.classify(bad, function, version)
            except ValueError:
                pass
            else:
                raise AssertionError(f"{version}: invalid read evidence was accepted")

        secret = "TOKEN=do-not-log-this"
        unexpected = subprocess.CompletedProcess([], 11, stdout=json.dumps({
            "schema_version": 1, "kind": "error", "error": {
                "code": "upstream", "reason": None, "message": secret,
                "upstream": {"errorcode": "invalidrecord", "message": secret},
            },
        }))
        try:
            module.classify(unexpected, function, version)
        except ValueError as error:
            require("code=upstream" in str(error) and "upstream_errorcode=invalidrecord" in str(error),
                    f"{version}: safe upstream error identifiers were not retained")
            require(secret not in str(error), f"{version}: a Moodle error message leaked")
        else:
            raise AssertionError(f"{version}: unexpected upstream error was accepted")

        mutated = json.loads(json.dumps(registry))
        row = next(row for row in mutated["functions"] if row["name"] == function["name"])
        row["effect"] = "write"
        try:
            module.selected_functions(mutated, recipe)
        except ValueError as error:
            require("not a safe mobile no-arg read" in str(error),
                    f"{version}: effect drift raised the wrong error")
        else:
            raise AssertionError(f"{version}: a write entered the read recipe")

        course_mutated = json.loads(json.dumps(registry))
        course_row = next(row for row in course_mutated["functions"]
                          if row["name"] == "core_course_get_contents")
        course_row["effect"] = "write"
        try:
            module.selected_course_functions(course_mutated, course_recipe)
        except ValueError as error:
            require("not a safe mobile read" in str(error),
                    f"{version}: course effect drift raised the wrong error")
        else:
            raise AssertionError(f"{version}: a write entered the course recipe")

    print("exposed-read recipe safety and outcome contract passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
