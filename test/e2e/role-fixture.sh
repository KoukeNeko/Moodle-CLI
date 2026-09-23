#!/usr/bin/env bash
# Seed every runtime role and write its machine-readable fixture contract.
set -euo pipefail

VERSION="${1:-v52}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
COMPOSE="$REPO_DIR/test/e2e/docker-compose.yml"
SERVICE="$VERSION-std"
OUTPUT="${2:-$REPO_DIR/test/reports/runtime-roles-$VERSION.json}"

case "$VERSION" in
  v45 | v51 | v52) ;;
  *) echo "unknown version: $VERSION (expected v45|v51|v52)" >&2; exit 2 ;;
esac

CONTAINER=$(docker compose --project-name moodle-cli-e2e --file "$COMPOSE" ps -q "$SERVICE")
[ -n "$CONTAINER" ] || { echo "$SERVICE is not running" >&2; exit 2; }

mkdir -p "$(dirname -- "$OUTPUT")"
docker cp "$REPO_DIR/test/e2e/seed-role-matrix.php" \
  "$CONTAINER:/seed-role-matrix.php" >/dev/null
docker exec "$CONTAINER" php /seed-role-matrix.php > "$OUTPUT"

python3 - "$OUTPUT" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
data = json.loads(path.read_text())
roles = data.get("roles", [])
runtime = [r for r in roles if not r.get("is_site_administrator")]
if data.get("schema_version") != 1:
    raise SystemExit("runtime-role fixture has an unsupported schema")
if data.get("runtime_role_count") != len(runtime):
    raise SystemExit("runtime-role count does not match the exported rows")
if data.get("principal_count") != len(roles):
    raise SystemExit("principal count does not match the exported rows")

shortnames = [r.get("shortname") for r in roles]
if len(shortnames) != len(set(shortnames)):
    raise SystemExit("runtime-role fixture contains duplicate shortnames")
required = {
    "manager", "coursecreator", "editingteacher", "teacher", "student",
    "guest", "user", "frontpage", "matrixteacher", "matrixblank",
    "site_administrator",
}
missing = sorted(required.difference(shortnames))
if missing:
    raise SystemExit(f"runtime-role fixture is missing: {', '.join(missing)}")

by_name = {r["shortname"]: r for r in roles}
if by_name["matrixteacher"].get("archetype") != "teacher":
    raise SystemExit("matrixteacher did not preserve its teacher archetype")
if by_name["matrixblank"].get("archetype") != "":
    raise SystemExit("matrixblank unexpectedly has an archetype")
if by_name["guest"].get("credential_kind") != "guest":
    raise SystemExit("guest was incorrectly modelled as a login account")
if not by_name["site_administrator"].get("is_site_administrator"):
    raise SystemExit("site administrator was incorrectly modelled as a role")
for name in ("manager", "coursecreator"):
    if by_name[name]["assignment"].get("context_level") != 10:
        raise SystemExit(f"{name} fixture is not assigned at system context")
for name in ("editingteacher", "teacher", "student", "matrixteacher", "matrixblank"):
    if by_name[name]["assignment"].get("context_level") != 50:
        raise SystemExit(f"{name} fixture is not a real course enrolment")

print(
    f"  ✓ discovered {len(runtime)} runtime roles and {len(roles)} principals; "
    "custom archetype and no-archetype canaries present"
)
PY

printf '  ✓ runtime-role fixture: %s\n' "$OUTPUT"
