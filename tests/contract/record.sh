#!/usr/bin/env bash
# Record real Moodle output for the contract tests.
#
#   make moodle-up V=v52 && tests/contract/record.sh v52 8521
#
# The recorded files are committed: they are the evidence that the published
# schemas match what a real Moodle actually sends.
set -euo pipefail
VERSION="${1:?usage: record.sh <v45|v51|v52> <port>}"
PORT="${2:?usage: record.sh <version> <port>}"

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
OUT="$REPO_DIR/tests/contract/testdata"
mkdir -p "$OUT"

MOODLE_WS_TOKEN=$(curl -fsS "http://localhost:$PORT/login/token.php" \
  -d username=student1 -d 'password=Student123!' -d service=moodle_mobile_app \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')
export MOODLE_WS_TOKEN
export MOODLE_CLI_CONFIG
MOODLE_CLI_CONFIG=$(mktemp -d)/config.yaml

"$REPO_DIR/bin/moodle" site add rec "http://localhost:$PORT" >/dev/null

for kind in doctor site.inspect auth.status course.list; do
  case "$kind" in
    doctor)       args=(doctor) ;;
    site.inspect) args=(site inspect) ;;
    auth.status)  args=(auth status) ;;
    course.list)  args=(course list) ;;
  esac
  # doctor exits non-zero when a check fails, which is still a valid document.
  "$REPO_DIR/bin/moodle" "${args[@]}" --json --pretty > "$OUT/$VERSION.$kind.json" || true
  echo "recorded $OUT/$VERSION.$kind.json"
done
