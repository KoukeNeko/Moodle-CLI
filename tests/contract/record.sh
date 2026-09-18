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

# A file to describe in the recorded submission plan. The plan is a dry run, so
# nothing is ever submitted by recording.
SAMPLE=$(mktemp -d)/report.pdf
printf '%%PDF-1.4 sample\n' > "$SAMPLE"
# Pick one this account has not handed in: the plan for an assignment that is
# already submitted is a conflict, not a plan.
ASSIGNMENT=""
for candidate in $("$REPO_DIR/bin/moodle" assignment list --json \
  | python3 -c 'import json,sys;[print(a["id"]) for a in json.load(sys.stdin)["data"]]'); do
  state=$("$REPO_DIR/bin/moodle" assignment status "$candidate" --json \
    | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["status"])')
  if [ "$state" != "submitted" ]; then ASSIGNMENT="$candidate"; break; fi
done
if [ -z "$ASSIGNMENT" ]; then
  echo "every assignment is already submitted for this account;" >&2
  echo "run 'make moodle-purge V=$VERSION && make moodle-up V=$VERSION' first" >&2
  exit 1
fi

for kind in doctor site.inspect auth.status course.list \
            assignment.list assignment.show assignment.status assignment.submit; do
  case "$kind" in
    doctor)            args=(doctor) ;;
    site.inspect)      args=(site inspect) ;;
    auth.status)       args=(auth status) ;;
    course.list)       args=(course list) ;;
    assignment.list)   args=(assignment list) ;;
    assignment.show)   args=(assignment show "$ASSIGNMENT") ;;
    assignment.status) args=(assignment status "$ASSIGNMENT") ;;
    assignment.submit) args=(assignment submit "$ASSIGNMENT" "$SAMPLE" --dry-run) ;;
  esac
  # doctor exits non-zero when a check fails, which is still a valid document.
  "$REPO_DIR/bin/moodle" "${args[@]}" --json --pretty > "$OUT/$VERSION.$kind.json" || true
  echo "recorded $OUT/$VERSION.$kind.json"
done
