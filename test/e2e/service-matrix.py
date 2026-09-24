#!/usr/bin/env python3
"""Execute every unavailable mobile-service boundary for runtime principals.

The generated core registry says which functions are absent from the official
mobile service. Each password principal actually calls ``moodle ws call`` for
each such function. The CLI must reject the call as unavailable before it
validates parameters or sends the function to Moodle. A changed service
exposure therefore fails this test instead of accidentally executing a write.

This is one safe subset of the full role/function matrix. Guest is covered by
the separate credential preflight until the public guest transport exists.
"""
from __future__ import annotations

import json
import os
import pathlib
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request


ROOT = pathlib.Path(__file__).resolve().parents[2]
PORTS = {"v45": 8451, "v51": 8511, "v52": 8521}
PASSWORD = "Student123!"


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
        raise RuntimeError(f"{username}: Moodle did not issue a disposable mobile token")
    return token


def main() -> int:
    version = sys.argv[1] if len(sys.argv) > 1 else "v52"
    if version not in PORTS:
        raise SystemExit("usage: service-matrix.py [v45|v51|v52] [inventory] [output]")
    inventory_path = pathlib.Path(sys.argv[2]) if len(sys.argv) > 2 else (
        ROOT / f"test/reports/runtime-roles-{version}.json")
    output = pathlib.Path(sys.argv[3]) if len(sys.argv) > 3 else (
        ROOT / f"test/reports/role-matrix-service-{version}.jsonl")
    binary = ROOT / "bin/moodle"
    if not binary.is_file() or not os.access(binary, os.X_OK):
        raise SystemExit("bin/moodle is missing; run make build first")

    inventory = json.loads(inventory_path.read_text())
    registry = json.loads((ROOT / f"internal/wsregistry/data/{version}.json").read_text())
    if inventory.get("schema_version") != 1 or registry.get("schema_version") != 1:
        raise SystemExit("runtime inventory or registry has an unsupported schema")
    if not inventory.get("moodle", {}).get("release", "").startswith(
        {"v45": "4.5.", "v51": "5.1.", "v52": "5.2."}[version]
    ):
        raise SystemExit("runtime inventory does not match the selected Moodle version")
    principals = [role for role in inventory["roles"]
                  if role.get("credential_kind") == "password"]
    guests = [role for role in inventory["roles"]
              if role.get("credential_kind") == "guest"]
    if len(guests) != 1 or guests[0].get("shortname") != "guest":
        raise SystemExit("expected one explicit guest principal")
    if len(principals) + 1 != inventory.get("principal_count"):
        raise SystemExit("one or more runtime principals have unsupported credentials")
    names = [role.get("shortname") for role in principals]
    if len(names) != len(set(names)) or any(not role.get("username") for role in principals):
        raise SystemExit("runtime principal metadata is incomplete or duplicated")

    # These functions are provably outside the token's service. Pass {} on
    # purpose: the service gate must run before schema validation, and a write
    # can never reach Moodle just because this test generated test parameters.
    unavailable = sorted(
        function["name"] for function in registry["functions"]
        if "moodle_mobile_app" not in function.get("services", [])
    )
    if not unavailable:
        raise SystemExit("registry unexpectedly contains no unavailable functions")

    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="moodle-service-matrix-") as directory, \
            tempfile.TemporaryDirectory(prefix=".moodle-service-matrix-", dir=output.parent) as artifact_dir:
        work = pathlib.Path(directory)
        environment = os.environ.copy()
        environment["MOODLE_CLI_CONFIG"] = str(work / "config.yaml")
        subprocess.run([
            str(binary), "site", "add", "service-matrix",
            f"http://127.0.0.1:{PORTS[version]}",
        ], env=environment, stdout=subprocess.DEVNULL, check=True, timeout=30)

        # The self-hosted runner mounts /tmp and the workspace on different
        # filesystems. Stage beside the final artifact for a real atomic rename.
        temporary_output = pathlib.Path(artifact_dir) / output.name
        count = 0
        with temporary_output.open("w") as target:
            for principal in principals:
                role = principal["shortname"]
                token = token_for(PORTS[version], principal["username"])
                role_environment = {**environment, "MOODLE_WS_TOKEN": token}
                for function in unavailable:
                    completed = subprocess.run([
                        str(binary), "ws", "call", function,
                        "--params-json", "{}", "--read-only", "--json",
                    ], env=role_environment, text=True, capture_output=True, timeout=30)
                    try:
                        document = json.loads(completed.stdout)
                    except json.JSONDecodeError:
                        document = {}
                    if not isinstance(document, dict):
                        document = {}
                    error = document.get("error", {})
                    if not isinstance(error, dict):
                        error = {}
                    if (completed.returncode != 9 or document.get("kind") != "error"
                            or error.get("code") != "unavailable"
                            or error.get("reason") != "capability"):
                        # No stdout/stderr is echoed: Moodle and CLI responses
                        # could contain credential-bearing diagnostics.
                        raise RuntimeError(json.dumps({
                            "version": version, "role": role, "function": function,
                            "exit": completed.returncode,
                            "code": error.get("code"), "reason": error.get("reason"),
                        }))
                    target.write(json.dumps({
                        "version": version,
                        "role": role,
                        "function": function,
                        "outcome": "expected_unavailable",
                        "recipe": "mobile-service-boundary",
                        "cli_exit": 9,
                    }, separators=(",", ":")) + "\n")
                    count += 1
                print(f"  ✓ {version}/{role}: {len(unavailable)} unavailable functions", file=sys.stderr)
        expected = len(principals) * len(unavailable)
        if count != expected:
            raise SystemExit(f"service matrix has {count}/{expected} cells")
        temporary_output.replace(output)

    print(f"  ✓ {count} executed CLI cells; redacted artifact: {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
