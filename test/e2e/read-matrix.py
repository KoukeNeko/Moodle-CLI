#!/usr/bin/env python3
"""Execute curated, exposed read functions for every password principal.

The allowlist is deliberately narrow: the registry must certify each recipe
as read-only, REST/mobile-exposed, and independent of external systems before
a request can reach the disposable Moodle fixture. Required course IDs come
from the database-backed role inventory, not the CLI under test. Results and
response bodies are never copied into the artifact.
"""
from __future__ import annotations

import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request


ROOT = pathlib.Path(__file__).resolve().parents[2]
PORTS = {"v45": 8451, "v51": 8511, "v52": 8521}
RECIPE = ROOT / "test/e2e/recipes/mobile-noarg-read.json"
COURSE_RECIPE = ROOT / "test/e2e/recipes/mobile-course-read.json"
PASSWORD = "Student123!"


def selected_functions(registry: dict, recipe: dict) -> list[dict]:
    if registry.get("schema_version") != 1 or recipe.get("schema_version") != 1:
        raise ValueError("registry or read recipe has an unsupported schema")
    names = recipe.get("functions")
    if not isinstance(names, list) or not names or any(not isinstance(n, str) for n in names):
        raise ValueError("read recipe requires a nonempty function-name list")
    if names != sorted(set(names)):
        raise ValueError("read recipe names must be sorted and unique")
    available = {row["name"]: row for row in registry["functions"]}
    selected = []
    for name in names:
        row = available.get(name)
        if row is None:
            raise ValueError(f"read recipe {name} is not installed")
        if (row.get("effect") != "read" or row.get("destructive") or row.get("credential")
                or row.get("external_dependency") != "none"
                or "moodle_mobile_app" not in row.get("services", [])
                or not row.get("transports", {}).get("rest")
                or row.get("parameters", {}).get("required")):
            raise ValueError(f"read recipe {name} is not a safe mobile no-arg read")
        if row.get("returns", {}).get("type") not in ("object", "array"):
            raise ValueError(f"read recipe {name} has no assertable response shape")
        selected.append(row)
    return selected


def selected_course_functions(registry: dict, recipe: dict) -> list[tuple[dict, str]]:
    if (registry.get("schema_version") != 1 or recipe.get("schema_version") != 1
            or recipe.get("fixture_binding") != "runtime-roles.fixture_course_id"):
        raise ValueError("registry or course recipe has an unsupported schema or fixture binding")
    bindings = recipe.get("functions")
    expected = {
        "core_course_get_contents": "courseid",
        "core_course_get_courses": "options.ids",
        "core_enrol_get_enrolled_users": "courseid",
    }
    if not isinstance(bindings, dict) or bindings != expected:
        raise ValueError("course recipe contains an unreviewed function or binding")
    available = {row["name"]: row for row in registry["functions"]}
    selected = []
    for name, binding in sorted(bindings.items()):
        row = available.get(name)
        properties = row.get("parameters", {}).get("properties", {}) if row else {}
        if (not row or row.get("effect") != "read" or row.get("destructive")
                or row.get("credential") or row.get("external_dependency") != "none"
                or "moodle_mobile_app" not in row.get("services", [])
                or not row.get("transports", {}).get("rest")
                or row.get("returns", {}).get("type") != "array"):
            raise ValueError(f"course recipe {name} is not a safe mobile read")
        if binding == "courseid":
            if (row["parameters"].get("required") != ["courseid"]
                    or "integer" not in properties.get("courseid", {}).get("type", [])):
                raise ValueError(f"course recipe {name} changed its courseid contract")
        elif (row["parameters"].get("required")
              or properties.get("options", {}).get("properties", {}).get("ids", {}).get("type") != "array"
              or "integer" not in properties["options"]["properties"]["ids"]["items"].get("type", [])):
            raise ValueError(f"course recipe {name} changed its options.ids contract")
        selected.append((row, binding))
    return selected


def course_params(binding: str, course_id: int) -> dict:
    if type(course_id) is not int or course_id <= 1:
        raise ValueError("runtime inventory has no valid fixture course ID")
    if binding == "courseid":
        return {"courseid": course_id}
    if binding == "options.ids":
        return {"options": {"ids": [course_id]}}
    raise ValueError("unreviewed course binding")


def classify(completed: subprocess.CompletedProcess[str], function: dict, version: str,
             *, course_id: int | None = None, role: str | None = None) -> str:
    try:
        document = json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        raise ValueError("typed WS did not emit a JSON envelope") from error
    if not isinstance(document, dict) or document.get("schema_version") != 1:
        raise ValueError("typed WS did not emit the v1 envelope")
    if completed.returncode == 0 and document.get("kind") == "ws.call":
        data = document.get("data", {})
        if (not isinstance(data, dict) or data.get("function") != function["name"]
                or data.get("version") != version or data.get("dry_run")
                or data.get("effect") != "read"):
            raise ValueError("typed WS returned the wrong function, version, or effect")
        expected_type = list if function["returns"]["type"] == "array" else dict
        if not isinstance(data.get("response"), expected_type):
            raise ValueError("typed WS response does not match the registry shape")
        if course_id is not None and role == "site_administrator":
            response = data["response"]
            if function["name"] == "core_course_get_courses":
                if [row.get("id") for row in response] != [course_id]:
                    raise ValueError("administrator did not read the fixture course")
            elif not response:
                raise ValueError("administrator received no fixture course data")
        return "passed"
    error = document.get("error", {})
    if isinstance(error, dict) and document.get("kind") == "error":
        if completed.returncode == 5 and error.get("code") == "permission_denied":
            return "expected_denied"
        if (completed.returncode == 9 and error.get("code") == "unavailable"
                and error.get("reason") == "capability"):
            return "expected_unavailable"
    # Only closed error identifiers enter CI logs. Moodle's message/hint and
    # the response body may contain fixture identity or credential material.
    def safe_code(value: object) -> str:
        return value if isinstance(value, str) and re.fullmatch(r"[A-Za-z0-9_./-]{1,80}", value) else "redacted"

    upstream = error.get("upstream", {}) if isinstance(error, dict) else {}
    if not isinstance(upstream, dict):
        upstream = {}
    raise ValueError(
        "typed WS returned an unexpected exit or outcome "
        f"(exit={completed.returncode}, code={safe_code(error.get('code') if isinstance(error, dict) else None)}, "
        f"reason={safe_code(error.get('reason') if isinstance(error, dict) else None)}, "
        f"upstream_errorcode={safe_code(upstream.get('errorcode'))})"
    )


def token_for(port: int, username: str) -> str:
    payload = urllib.parse.urlencode({
        "username": username,
        "password": PASSWORD,
        "service": "moodle_mobile_app",
    }).encode()
    request = urllib.request.Request(f"http://127.0.0.1:{port}/login/token.php", data=payload)
    with urllib.request.urlopen(request, timeout=20) as response:
        document = json.load(response)
    token = document.get("token")
    if not isinstance(token, str) or not token:
        raise ValueError("Moodle did not issue a disposable mobile token")
    return token


def main() -> int:
    version = sys.argv[1] if len(sys.argv) > 1 else "v52"
    if version not in PORTS:
        raise SystemExit("usage: read-matrix.py [v45|v51|v52] [inventory] [output]")
    inventory_path = pathlib.Path(sys.argv[2]) if len(sys.argv) > 2 else (
        ROOT / f"test/reports/runtime-roles-{version}.json")
    output = pathlib.Path(sys.argv[3]) if len(sys.argv) > 3 else (
        ROOT / f"test/reports/role-matrix-read-{version}.jsonl")
    binary = ROOT / "bin/moodle"
    if not binary.is_file() or not os.access(binary, os.X_OK):
        raise SystemExit("bin/moodle is missing; run make build first")

    inventory = json.loads(inventory_path.read_text())
    registry = json.loads((ROOT / f"internal/wsregistry/data/{version}.json").read_text())
    recipe = json.loads(RECIPE.read_text())
    course_recipe = json.loads(COURSE_RECIPE.read_text())
    if (inventory.get("schema_version") != 1 or
            not inventory.get("moodle", {}).get("release", "").startswith(
                {"v45": "4.5.", "v51": "5.1.", "v52": "5.2."}[version])):
        raise SystemExit("runtime inventory does not match the selected Moodle version")
    functions = selected_functions(registry, recipe)
    course_functions = selected_course_functions(registry, course_recipe)
    course_id = inventory.get("fixture_course_id")
    course_params("courseid", course_id)
    principals = [row for row in inventory["roles"] if row.get("credential_kind") == "password"]
    guests = [row for row in inventory["roles"] if row.get("credential_kind") == "guest"]
    if (len(guests) != 1 or guests[0].get("shortname") != "guest"
            or len(principals) + 1 != inventory.get("principal_count")):
        raise SystemExit("one or more runtime principals have unsupported credentials")
    names = [row.get("shortname") for row in principals]
    if (len(names) != len(set(names)) or "site_administrator" not in names
            or any(not row.get("username") for row in principals)):
        raise SystemExit("runtime principal metadata is incomplete or duplicated")

    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="moodle-read-matrix-") as directory, \
            tempfile.TemporaryDirectory(prefix=".moodle-read-matrix-", dir=output.parent) as artifact_dir:
        work = pathlib.Path(directory)
        environment = os.environ.copy()
        environment["MOODLE_CLI_CONFIG"] = str(work / "config.yaml")
        subprocess.run([
            str(binary), "site", "add", "read-matrix",
            f"http://127.0.0.1:{PORTS[version]}",
        ], env=environment, stdout=subprocess.DEVNULL, check=True, timeout=30)
        temporary_output = pathlib.Path(artifact_dir) / output.name
        with temporary_output.open("w") as target:
            for principal in principals:
                role = principal["shortname"]
                token = token_for(PORTS[version], principal["username"])
                role_environment = {**environment, "MOODLE_WS_TOKEN": token}
                for function, recipe_name, parameters, bound_course in (
                    [(row, "mobile-noarg-read", {}, None) for row in functions]
                    + [(row, "mobile-course-read", course_params(binding, course_id), course_id)
                       for row, binding in course_functions]
                ):
                    completed = subprocess.run([
                        str(binary), "ws", "call", function["name"],
                        "--params-json", json.dumps(parameters, separators=(",", ":")),
                        "--read-only", "--json",
                    ], env=role_environment, text=True, capture_output=True, timeout=60)
                    try:
                        outcome = classify(completed, function, version,
                                           course_id=bound_course, role=role)
                    except ValueError as error:
                        # Never print the response, stdout, stderr, or token:
                        # Moodle errors can contain user or credential data.
                        raise RuntimeError(f"{version}/{role}/{function['name']}: {error}") from error
                    if role == "site_administrator" and outcome != "passed":
                        raise RuntimeError(f"{version}/{role}/{function['name']}: administrator read failed")
                    target.write(json.dumps({
                        "version": version,
                        "role": role,
                        "function": function["name"],
                        "outcome": outcome,
                        "recipe": recipe_name,
                        "cli_exit": completed.returncode,
                    }, separators=(",", ":")) + "\n")
                print(f"  ✓ {version}/{role}: {len(functions) + len(course_functions)} exposed reads",
                      file=sys.stderr)
        temporary_output.replace(output)

    print(f"  ✓ {len(principals) * (len(functions) + len(course_functions))} executed CLI cells; redacted artifact: {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
