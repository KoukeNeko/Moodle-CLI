#!/usr/bin/env python3
"""Build the public, redacted dashboard snapshot from synthetic test reports."""
from __future__ import annotations

import datetime as dt
import json
import math
import os
import pathlib
import statistics
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
OUTPUT = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else ROOT / "report-site/src/data.json"


def load(path: pathlib.Path, default):
    try:
        return json.loads(path.read_text())
    except (FileNotFoundError, json.JSONDecodeError):
        return default


def load_jsonl(path: pathlib.Path) -> list[dict]:
    """Load a strict JSONL evidence file.

    A present but malformed role matrix must fail the report build. Silently
    turning bad execution evidence into ``not-run`` would erase a failed run
    from the public record.
    """
    if not path.exists():
        return []
    rows = []
    for line_number, raw in enumerate(path.read_text().splitlines(), 1):
        if not raw.strip():
            continue
        try:
            row = json.loads(raw)
        except json.JSONDecodeError as error:
            raise SystemExit(f"{path}:{line_number}: invalid JSON: {error}") from error
        if not isinstance(row, dict):
            raise SystemExit(f"{path}:{line_number}: role cell must be a JSON object")
        rows.append(row)
    return rows


def source(label: str, files: list[str], component_ids: list[str], caveats: list[str] | None = None):
    return {
        "label": label,
        "tables": files,
        "caveats": caveats or ["Synthetic fixtures only; credentials and request payload secrets are excluded."],
        "metricDefinitions": [{
            "label": label,
            "definition": "A deterministic result emitted by the Moodle CLI test harness.",
            "componentIds": component_ids,
            "sourceLineage": [{"tables": files}],
        }],
    }


versions = []
functions = []
for key in ("v45", "v51", "v52"):
    path = ROOT / f"internal/wsregistry/data/{key}.json"
    snapshot = load(path, {})
    rows = snapshot.get("functions", [])
    versions.append({
        "version": key,
        "release": snapshot.get("moodle", {}).get("release", "unknown"),
        "functions": len(rows),
        "read": sum(row.get("effect") == "read" for row in rows),
        "write": sum(row.get("effect") == "write" for row in rows),
        "deprecated": sum(bool(row.get("deprecated")) for row in rows),
    })
    for row in rows:
        functions.append({
            "version": key,
            "function": row.get("name"),
            "component": row.get("component"),
            "effect": row.get("effect"),
            "destructive": bool(row.get("destructive")),
            "transport": ", ".join(name.upper() for name, enabled in row.get("transports", {}).items() if enabled),
            "dependency": row.get("external_dependency", "none"),
            "recipe": "schema-generated",
            "result": "registry-covered",
        })

default_role_names = [
    "site administrator", "manager", "coursecreator", "editingteacher", "teacher",
    "student", "guest", "user", "frontpage", "custom archetype", "custom no-archetype",
]
role_matrix_path = pathlib.Path(os.environ.get(
    "MOODLE_ROLE_MATRIX_REPORT", ROOT / "test/reports/role-matrix.jsonl"))
role_cells = load_jsonl(role_matrix_path)
role_outcome_names = ("passed", "expected_denied", "expected_unavailable", "failed")
allowed_role_outcomes = set(role_outcome_names)
function_index = {(row["version"], row["function"]): row for row in functions}

role_groups = {}
function_outcomes = {}
for line_number, cell in enumerate(role_cells, 1):
    missing = [key for key in ("version", "role", "function", "outcome") if not cell.get(key)]
    if missing:
        raise SystemExit(
            f"{role_matrix_path}:{line_number}: role cell is missing {', '.join(missing)}")
    version = cell["version"]
    function = cell["function"]
    outcome = cell["outcome"]
    registry_row = function_index.get((version, function))
    if registry_row is None:
        raise SystemExit(
            f"{role_matrix_path}:{line_number}: {version}/{function} is not in the generated registry")
    if outcome not in allowed_role_outcomes:
        raise SystemExit(
            f"{role_matrix_path}:{line_number}: outcome {outcome!r} is not allowed; "
            "skip is deliberately not a role-matrix result")
    # Moodle records many core functions under the broad "moodle" component.
    # The first two function-name segments retain a useful product domain
    # (core_course, mod_assign, tool_mobile, …) unless a recipe supplies one.
    domain = cell.get("domain") or "_".join(function.split("_")[:2])
    key = (version, cell["role"], domain)
    counts = role_groups.setdefault(key, {name: 0 for name in role_outcome_names})
    counts[outcome] += 1
    function_outcomes.setdefault((version, function), []).append(outcome)

if role_groups:
    role_rows = []
    for (version, role, domain), counts in sorted(role_groups.items()):
        role_rows.append({
            "version": version,
            "role": role,
            "domain": domain,
            **counts,
            "status": "failed" if counts["failed"] else "passed",
        })
    for row in functions:
        outcomes = function_outcomes.get((row["version"], row["function"]), [])
        if outcomes:
            row["result"] = "failed" if "failed" in outcomes else "executed"
else:
    role_rows = [{
        "version": version,
        "role": role,
        "domain": "all core functions",
        "passed": 0,
        "expected_denied": 0,
        "expected_unavailable": 0,
        "failed": 0,
        "status": "not-run",
    } for version in ("v45", "v51", "v52") for role in default_role_names]

truth = load(ROOT / "test/reports/scale-v52/truth.json", {})
participant_path = ROOT / "test/reports/scale-v52/participants.tsv"
participant_rows = []
if participant_path.exists():
    for line in participant_path.read_text().splitlines()[1:]:
        fields = line.split("\t")
        if len(fields) == 4:
            participant_rows.append({
                "page": int(fields[0]), "seconds": float(fields[1]),
                "rss_kib": int(fields[2]), "rows": int(fields[3]),
            })
summary = load(ROOT / "test/reports/scale-v52/scale-summary.json", {})
if participant_rows and not summary:
    timings = sorted(row["seconds"] for row in participant_rows)
    summary = {
        "participants": sum(row["rows"] for row in participant_rows),
        "pages": len(participant_rows),
        "page_size": min(row["rows"] for row in participant_rows),
        "seconds": {
            "p50": statistics.median(timings),
            "p95": timings[math.ceil(len(timings) * .95) - 1],
            "maximum": max(timings),
        },
        "peak_rss_bytes": max(row["rss_kib"] for row in participant_rows) * 1024,
        "http_requests": None,
    }

scale_rows = []
if truth:
    scale_rows.append({
        "version": "v52",
        "status": "partial" if summary.get("http_requests") is None else "passed",
        "students": truth.get("students"),
        "courses": truth.get("academic_courses"),
        "enrolments": truth.get("enrolments", {}).get("total"),
        "span_days": truth.get("span_days"),
        "undergraduate_credits": truth.get("undergraduate", {}).get("credits_per_term"),
        "graduate_credits": truth.get("graduate", {}).get("credits_per_term"),
        "participant_pages": summary.get("pages"),
        "participant_p50_seconds": summary.get("seconds", {}).get("p50"),
        "participant_p95_seconds": summary.get("seconds", {}).get("p95"),
        "peak_rss_bytes": summary.get("peak_rss_bytes"),
        "http_requests": summary.get("http_requests"),
        "http_request_budget": summary.get("http_request_budget"),
        "postgres_bytes": summary.get("postgres_bytes"),
    })

scale_caveats = [
    "Synthetic data only. REST evidence counts POSTs during participant pagination; request bodies and credentials are excluded."
] if summary.get("http_requests") is not None else [
    "Synthetic data only. Null HTTP requests means the corrected REST-only request gate has not yet been rerun."
]

try:
    commit = subprocess.check_output(["git", "rev-parse", "--short=12", "HEAD"], cwd=ROOT, text=True).strip()
except (OSError, subprocess.CalledProcessError):
    commit = "unknown"
generated = dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
role_matrix_status = (
    "not-run" if not role_cells else
    "failed" if any(cell["outcome"] == "failed" for cell in role_cells) else
    "observed"
)
runs = [{
    "run": os.environ.get("GITHUB_RUN_ID", "local-observation"),
    "commit": os.environ.get("GITHUB_SHA", commit),
    "generated_at": generated,
    "runner": os.environ.get("RUNNER_NAME", "local"),
    "runner_image": os.environ.get("ImageOS", "self-hosted-linux-x64"),
    "registry_functions": len({row["function"] for row in functions}),
    "role_matrix": role_matrix_status,
    "scale": scale_rows[0]["status"] if scale_rows else "not-run",
}]

snapshot = {
    "id": "moodle-cli-verification-dashboard",
    "title": "Moodle CLI verification",
    "generatedAt": generated,
    "status": "observed",
    "surface": "dashboard",
    "buildStatus": "complete",
    "filters": [
        {"id": "version", "label": "Moodle version", "field": "version", "defaultValue": "all",
         "queryIds": ["versions", "functions", "roles", "scale"]},
        {"id": "component", "label": "Component", "field": "component", "defaultValue": "all",
         "queryIds": ["functions"]},
        {"id": "effect", "label": "Effect", "field": "effect", "defaultValue": "all",
         "queryIds": ["functions"]},
        {"id": "status", "label": "Result", "field": "status", "defaultValue": "all",
         "queryIds": ["roles", "scale"]},
    ],
    "queries": {
        "versions": {"rows": versions, "source": source("Generated registry snapshots",
            ["internal/wsregistry/data/v45.json", "internal/wsregistry/data/v51.json", "internal/wsregistry/data/v52.json"],
            ["registry-size", "version-coverage"])},
        "functions": {"rows": functions, "source": source("Core external-function coverage",
            ["test/e2e/export-ws-registry.php", "internal/wsregistry/data/*.json"], ["function-table"])},
        "roles": {"rows": role_rows, "source": source("Runtime role/function matrix",
            [str(role_matrix_path.relative_to(ROOT)) if role_matrix_path.is_relative_to(ROOT)
             else str(role_matrix_path)], ["role-summary", "role-table"],
            (["No role/function matrix artifact was present in this snapshot; cells are explicitly not-run, never skipped."]
             if not role_cells else
             ["Every row is an executed CLI cell. Expected denials and expected unavailability are successful security outcomes, not skips."]))},
        "scale": {"rows": scale_rows, "source": source("PostgreSQL scale acceptance",
            ["test/reports/scale-v52/truth.json", "test/reports/scale-v52/participants.tsv",
             "test/reports/scale-v52/scale-summary.json", "test/reports/scale-v52/rest-access.log"],
            ["scale-students", "scale-enrolments", "scale-latency", "scale-table"],
            scale_caveats)},
        "runs": {"rows": runs, "source": source("Run provenance", ["GitHub Actions run metadata"], ["runs-table"])},
    },
}
OUTPUT.parent.mkdir(parents=True, exist_ok=True)
OUTPUT.write_text(json.dumps(snapshot, indent=2) + "\n")
print(OUTPUT)
