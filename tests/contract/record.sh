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
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT

COURSE=$("$REPO_DIR/bin/moodle" course list --json \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"][0]["id"])')

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

# Record assignment.show from one that has an attachment, so the recording
# covers that field on every version. Picking whichever happened to be
# unsubmitted left one version's fixture silently empty.
WITH_ATTACHMENT=""
ATTACHMENT=""
for candidate in $("$REPO_DIR/bin/moodle" assignment list --json \
  | python3 -c 'import json,sys;[print(a["id"]) for a in json.load(sys.stdin)["data"]]'); do
  url=$("$REPO_DIR/bin/moodle" assignment show "$candidate" --json \
    | python3 -c 'import json,sys;a=json.load(sys.stdin)["data"]["attachments"];print(a[0]["url"] if a else "")')
  if [ -n "$url" ]; then WITH_ATTACHMENT="$candidate"; ATTACHMENT="$url"; break; fi
done
if [ -z "$ATTACHMENT" ]; then
  echo "no assignment has an attachment; re-seed the site" >&2
  exit 1
fi

# The seeded general forum and its one thread. The announcements forum has no
# discussions a student can see, so recording from it would prove nothing.
FORUM=$("$REPO_DIR/bin/moodle" forum list --json \
  | python3 -c 'import json,sys;f=[x for x in json.load(sys.stdin)["data"] if x["kind"]=="general"];print(f[0]["id"] if f else "")')
if [ -z "$FORUM" ]; then echo "no general forum; re-seed the site" >&2; exit 1; fi
DISCUSSION=$("$REPO_DIR/bin/moodle" forum discussions "$FORUM" --json \
  | python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];print(d[0]["id"] if d else "")')
if [ -z "$DISCUSSION" ]; then echo "no discussion; re-seed the site" >&2; exit 1; fi

for kind in doctor site.inspect auth.status course.list \
            assignment.list assignment.show assignment.status assignment.submit \
            grade.list grade.overview calendar.upcoming file.download resolve \
            api.functions api.call \
            forum.list forum.discussions forum.thread; do
  case "$kind" in
    doctor)            args=(doctor) ;;
    site.inspect)      args=(site inspect) ;;
    auth.status)       args=(auth status) ;;
    course.list)       args=(course list) ;;
    grade.list)        args=(grade list --course "$COURSE") ;;
    grade.overview)    args=(grade overview) ;;
    calendar.upcoming) args=(calendar upcoming) ;;
    # A real download, into a scratch directory that is thrown away: the point
    # is that the recorded document comes from bytes that actually moved.
    file.download)     args=(file download "$ATTACHMENT" --dir "$SCRATCH" --force) ;;
    # Parsing only; recorded against an address this site really produced.
    resolve)           args=(resolve "$ATTACHMENT") ;;
    api.functions)     args=(api functions --match core_webservice) ;;
    # A plain read through the escape hatch; nothing here needs --allow-write.
    api.call)          args=(api call core_webservice_get_site_info) ;;
    forum.list)        args=(forum list) ;;
    forum.discussions) args=(forum discussions "$FORUM") ;;
    forum.thread)      args=(forum read "$DISCUSSION") ;;
    assignment.list)   args=(assignment list) ;;
    assignment.show)   args=(assignment show "$WITH_ATTACHMENT") ;;
    assignment.status) args=(assignment status "$ASSIGNMENT") ;;
    assignment.submit) args=(assignment submit "$ASSIGNMENT" "$SAMPLE" --dry-run) ;;
  esac
  # doctor exits non-zero when a check fails, which is still a valid document.
  "$REPO_DIR/bin/moodle" "${args[@]}" --json --pretty > "$OUT/$VERSION.$kind.json" || true
  echo "recorded $OUT/$VERSION.$kind.json"
done
