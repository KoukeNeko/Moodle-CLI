#!/usr/bin/env bash
# End-to-end scale acceptance: PostgreSQL truth is the control plane and the
# compiled CLI is the system under test. Reports contain synthetic data only.
set -euo pipefail

VERSION="${1:-v52}"
SEED="${E2E_SEED:-20260923}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
COMPOSE="$REPO_DIR/test/scale/docker-compose.yml"
PROJECT=moodle-cli-scale
REPORT="$REPO_DIR/test/reports/scale-$VERSION"
mkdir -p "$REPORT"

case "$VERSION" in
  v45) PORT=9451 ;;
  v51) PORT=9511 ;;
  v52) PORT=9521 ;;
  *) echo "unknown version: $VERSION" >&2; exit 2 ;;
esac

cleanup() {
  "$REPO_DIR/scripts/moodle-scale-env.sh" down "$VERSION" --purge || true
}
trap cleanup EXIT INT TERM

if [ "${SCALE_SKIP_UP:-0}" != 1 ]; then
  "$REPO_DIR/scripts/moodle-scale-env.sh" up "$VERSION"
fi
CONTAINER=$(docker compose --project-name "$PROJECT" --file "$COMPOSE" ps -q moodle)
POSTGRES=$(docker compose --project-name "$PROJECT" --file "$COMPOSE" ps -q postgres)
PROXY=$(docker compose --project-name "$PROJECT" --file "$COMPOSE" ps -q proxy)
test -n "$CONTAINER" && test -n "$POSTGRES" && test -n "$PROXY"

docker cp "$REPO_DIR/test/scale/seed.php" "$CONTAINER:/tmp/scale-seed.php"
docker cp "$REPO_DIR/test/scale/truth.php" "$CONTAINER:/tmp/scale-truth.php"

echo "==> seed 50k students / 1001 courses / 2.37m enrolments"
if [ "${SCALE_SKIP_SEED:-0}" != 1 ]; then
  /usr/bin/time -f '%e\t%M' -o "$REPORT/seed-metrics.tsv" \
    timeout 10800 docker exec "$CONTAINER" php -d memory_limit=1024M \
      /tmp/scale-seed.php "$SEED" > "$REPORT/seed.jsonl"
fi

echo "==> independent SQL truth"
/usr/bin/time -f '%e\t%M' -o "$REPORT/truth-metrics.tsv" \
  timeout 120 docker exec "$CONTAINER" php /tmp/scale-truth.php \
  > "$REPORT/truth.json"

cd "$REPO_DIR"
make build

token_for() {
  curl -fsS "http://127.0.0.1:$PORT/login/token.php" \
    -d "username=$1" -d "password=$2" -d service=moodle_mobile_app \
    | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "token" in d, d; print(d["token"])'
}

run_workload() {
  local username="$1" expected_groups="$2" expected_courses="$3" credits="$4"
  local work config token output metrics
  work=$(mktemp -d /tmp/moodle-cli-scale.XXXXXX)
  config="$work/config.yaml"
  output="$REPORT/workload-$username.json"
  metrics="$REPORT/workload-$username.metrics"
  token=$(token_for "$username" 'Student123!')
  MOODLE_CLI_CONFIG="$config" "$REPO_DIR/bin/moodle" \
    site add scale "http://127.0.0.1:$PORT" >/dev/null
  MOODLE_CLI_CONFIG="$config" "$REPO_DIR/bin/moodle" \
    site academic configure --yes >/dev/null
  /usr/bin/time -f '%e\t%M' -o "$metrics" timeout 120 env \
    MOODLE_CLI_CONFIG="$config" MOODLE_WS_TOKEN="$token" \
    "$REPO_DIR/bin/moodle" workload validate --require-minimum --json > "$output"
  python3 - "$output" "$expected_groups" "$expected_courses" "$credits" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1]))
groups = doc["data"]["groups"]
assert doc["data"]["all_meet_minimum"] is True, doc
assert len(groups) == int(sys.argv[2]), len(groups)
assert sum(len(group["courses"]) for group in groups) == int(sys.argv[3])
expected = [float(value) for value in sys.argv[4].split(',')]
assert sorted(group["credits"] for group in groups) == sorted(expected)
PY
}

echo "==> CLI workload invariants"
run_workload scaleug00001 9 57 '0,21,21,21,21,21,21,21,21'
run_workload scalegr00001 5 9 '0,6,6,6,6'

echo "==> 50,000 participant pagination"
orientation=$(docker exec "$CONTAINER" php -r \
  'define("CLI_SCRIPT",true);require("/var/www/html/config.php");echo $DB->get_field("course","id",["shortname"=>"SCALE-ORIENTATION"]);')
work=$(mktemp -d /tmp/moodle-cli-participants.XXXXXX)
config="$work/config.yaml"
token=$(token_for admin 'Admin123!')
MOODLE_CLI_CONFIG="$config" "$REPO_DIR/bin/moodle" \
  site add scale "http://127.0.0.1:$PORT" >/dev/null
: > "$work/ids.txt"
printf 'page\tseconds\trss_kib\trows\n' > "$REPORT/participants.tsv"
requests_before=$(docker logs "$PROXY" 2>&1 \
  | grep -c 'POST /webservice/rest/server.php' || true)
for page in $(seq 0 49); do
  offset=$((page * 1000))
  params=$(printf '{"courseid":%s,"options":[{"name":"limitfrom","value":%d},{"name":"limitnumber","value":1000}]}' \
    "$orientation" "$offset")
  /usr/bin/time -f '%e\t%M' -o "$work/time" timeout 120 env \
    MOODLE_CLI_CONFIG="$config" MOODLE_WS_TOKEN="$token" \
    "$REPO_DIR/bin/moodle" participant list --params-json "$params" --json \
    > "$work/page.json"
  rows=$(python3 - "$work/page.json" "$work/ids.txt" <<'PY'
import json, sys
rows = json.load(open(sys.argv[1]))["data"]["response"]
with open(sys.argv[2], "a") as out:
    for row in rows:
        out.write(str(row["id"]) + "\n")
print(len(rows))
PY
)
  read -r seconds rss < "$work/time"
  printf '%d\t%s\t%s\t%s\n' "$page" "$seconds" "$rss" "$rows" >> "$REPORT/participants.tsv"
done
requests_after=$(docker logs "$PROXY" 2>&1 \
  | grep -c 'POST /webservice/rest/server.php' || true)
http_requests=$((requests_after - requests_before))
docker logs "$PROXY" 2>&1 \
  | grep 'POST /webservice/rest/server.php' > "$REPORT/rest-access.log" || true

python3 - "$work/ids.txt" "$REPORT/participants.tsv" "$http_requests" \
  "$REPORT/scale-summary.json" <<'PY'
import json, math, pathlib, statistics, sys
ids = [line.strip() for line in open(sys.argv[1]) if line.strip()]
assert len(ids) == 50000, len(ids)
assert len(set(ids)) == 50000, "participant pagination duplicated users"
rows = [line.rstrip().split("\t") for line in open(sys.argv[2])][1:]
seconds = sorted(float(row[1]) for row in rows)
rss = [int(row[2]) for row in rows]
counts = [int(row[3]) for row in rows]
assert counts == [1000] * 50, counts
assert max(seconds) <= 120, max(seconds)
assert max(rss) <= 1024 * 1024, max(rss)
requests = int(sys.argv[3])
# Every process discovers capabilities once and performs one participant call.
# Permit one additional request per page for a redirect, credential refresh or
# another bounded transport concern.  This remains O(pages): a per-participant
# N+1 regression would produce about 50,000 requests and fail by a wide margin.
# Only REST POSTs are counted; proxy healthchecks are intentionally excluded.
minimum_requests = len(rows) * 2
maximum_requests = len(rows) * 3
assert minimum_requests <= requests <= maximum_requests, {
    "requests": requests,
    "minimum": minimum_requests,
    "maximum": maximum_requests,
}
summary = {
    "schema_version": 1,
    "participants": len(ids),
    "pages": 50,
    "page_size": 1000,
    "seconds": {
        "p50": statistics.median(seconds),
        "p95": seconds[math.ceil(len(seconds) * .95) - 1],
        "maximum": max(seconds),
    },
    "peak_rss_bytes": max(rss) * 1024,
    "http_requests": requests,
    "http_request_budget": maximum_requests,
}
pathlib.Path(sys.argv[4]).write_text(json.dumps(summary, indent=2) + "\n")
PY

data_kib=$(docker exec "$POSTGRES" du -sk /var/lib/postgresql/data | awk '{print $1}')
test "$data_kib" -le $((8 * 1024 * 1024))
python3 - "$REPORT/scale-summary.json" "$data_kib" <<'PY'
import json, sys
path = sys.argv[1]
doc = json.load(open(path))
doc["postgres_bytes"] = int(sys.argv[2]) * 1024
open(path, "w").write(json.dumps(doc, indent=2) + "\n")
PY
cat "$REPORT/scale-summary.json"
