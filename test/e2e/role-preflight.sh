#!/usr/bin/env bash
# Prove that every discovered test principal can reach the typed CLI transport.
#
# This is intentionally a preflight, not the all-functions matrix: it executes
# one harmless core read for every password principal and an explicit
# credential-unavailable CLI path for guest. Tokens, passwords, response
# payloads, and usernames never enter the emitted artifact.
set -euo pipefail

VERSION="${1:-v52}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
BIN="$REPO_DIR/bin/moodle"
INVENTORY="${2:-$REPO_DIR/test/reports/runtime-roles-$VERSION.json}"
OUTPUT="${3:-$REPO_DIR/test/reports/role-preflight-$VERSION.jsonl}"

case "$VERSION" in
  v45) PORT=8451 ;;
  v51) PORT=8511 ;;
  v52) PORT=8521 ;;
  *) echo "unknown version: $VERSION (expected v45|v51|v52)" >&2; exit 2 ;;
esac

[ -x "$BIN" ] || { echo "bin/moodle is missing; run make build first" >&2; exit 2; }
[ -f "$INVENTORY" ] || { echo "runtime-role inventory is missing: $INVENTORY" >&2; exit 2; }

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
export MOODLE_CLI_CONFIG="$WORKDIR/config.yaml"
"$BIN" site add role-preflight "http://127.0.0.1:$PORT" >/dev/null
mkdir -p "$(dirname -- "$OUTPUT")"
: > "$OUTPUT"

python3 - "$INVENTORY" > "$WORKDIR/principals.tsv" <<'PY'
import json
import pathlib
import sys

document = json.loads(pathlib.Path(sys.argv[1]).read_text())
for role in document["roles"]:
    print("\t".join((
        role["shortname"],
        role["credential_kind"],
        role.get("username") or "-",
    )))
PY

emit() {
  python3 - "$VERSION" "$1" "$2" "$3" "$4" >> "$OUTPUT" <<'PY'
import json
import sys

print(json.dumps({
    "schema_version": 1,
    "version": sys.argv[1],
    "role": sys.argv[2],
    "credential": sys.argv[3],
    "function": "core_webservice_get_site_info",
    "outcome": sys.argv[4],
    "cli_exit": int(sys.argv[5]),
}, separators=(",", ":")))
PY
}

while IFS=$'\t' read -r ROLE KIND USERNAME; do
  if [ "$KIND" = "guest" ]; then
    set +e
    env -u MOODLE_WS_TOKEN -u MOODLE_SESSION \
      "$BIN" ws call core_webservice_get_site_info --params-json '{}' --json \
      > "$WORKDIR/guest.json" 2> "$WORKDIR/guest.stderr"
    CODE=$?
    set -e
    python3 - "$WORKDIR/guest.json" "$CODE" <<'PY'
import json
import pathlib
import sys

document = json.loads(pathlib.Path(sys.argv[1]).read_text())
if int(sys.argv[2]) != 4 or document.get("kind") != "error":
    raise SystemExit("guest did not receive the CLI authentication exit")
error = document.get("error", {})
if error.get("code") != "authentication" or error.get("reason") != "credential_missing":
    raise SystemExit("guest did not receive the explicit credential-missing result")
PY
    emit "$ROLE" "unavailable" "expected_unavailable" "$CODE"
    continue
  fi

  [ "$KIND" = "password" ] && [ "$USERNAME" != "-" ] || {
    echo "$ROLE has unsupported credential metadata: $KIND" >&2
    exit 1
  }
  TOKEN=$(curl -fsS "http://127.0.0.1:$PORT/login/token.php" \
    -d "username=$USERNAME" -d 'password=Student123!' -d service=moodle_mobile_app \
    | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token", ""))')
  [ -n "$TOKEN" ] || { echo "$ROLE could not obtain a disposable mobile token" >&2; exit 1; }

  MOODLE_WS_TOKEN="$TOKEN" \
    "$BIN" ws call core_webservice_get_site_info --params-json '{}' --json \
    > "$WORKDIR/call.json" 2> "$WORKDIR/call.stderr"
  python3 - "$WORKDIR/call.json" "$USERNAME" "$VERSION" <<'PY'
import json
import pathlib
import sys

document = json.loads(pathlib.Path(sys.argv[1]).read_text())
if document.get("schema_version") != 1 or document.get("kind") != "ws.call":
    raise SystemExit("typed WS preflight did not return the v1 ws.call envelope")
data = document.get("data", {})
if data.get("function") != "core_webservice_get_site_info" or data.get("dry_run"):
    raise SystemExit("typed WS preflight did not execute core_webservice_get_site_info")
if data.get("version") != sys.argv[3]:
    raise SystemExit("typed WS preflight selected the wrong Moodle registry version")
response = data.get("response")
if not isinstance(response, dict) or response.get("username") != sys.argv[2]:
    raise SystemExit("typed WS preflight token does not belong to the fixture identity")
PY
  emit "$ROLE" "issued" "passed" 0
  unset TOKEN
done < "$WORKDIR/principals.tsv"

python3 - "$INVENTORY" "$OUTPUT" <<'PY'
import json
import pathlib
import sys

inventory = json.loads(pathlib.Path(sys.argv[1]).read_text())
rows = [json.loads(line) for line in pathlib.Path(sys.argv[2]).read_text().splitlines() if line]
expected = {role["shortname"] for role in inventory["roles"]}
actual = {row["role"] for row in rows}
if len(rows) != len(expected) or actual != expected:
    raise SystemExit(f"preflight principals differ: missing={sorted(expected-actual)}, extra={sorted(actual-expected)}")
if any(row["outcome"] not in {"passed", "expected_unavailable"} for row in rows):
    raise SystemExit("preflight contains an unsupported outcome")
if any(key in row for row in rows for key in ("token", "password", "username", "response")):
    raise SystemExit("preflight artifact contains credential or response material")
guest = next(row for row in rows if row["role"] == "guest")
if guest["outcome"] != "expected_unavailable" or guest["credential"] != "unavailable":
    raise SystemExit("guest was not recorded as explicitly credential-unavailable")
passed = sum(row["outcome"] == "passed" for row in rows)
print(f"  ✓ typed WS credential preflight: {passed} authenticated principals, 1 explicit guest unavailable")
PY

printf '  ✓ redacted role preflight: %s\n' "$OUTPUT"
