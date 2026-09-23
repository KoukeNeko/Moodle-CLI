#!/usr/bin/env bash
# 佈建並驗收一個累積十年資料的 Moodle 站台。
#
#   make moodle-up V=v52
#   test/e2e/decade-run.sh v52
#
# 這不是單純數資料列：先由容器內的 Moodle API 建立情境，再分別以
# 資料庫真值（control plane）與 CLI 契約（system under test）核對。
set -euo pipefail

VERSION="${1:-v52}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
BIN="$REPO_DIR/bin/moodle"
COMPOSE="$REPO_DIR/test/e2e/docker-compose.yml"

case "$VERSION" in
  v45) PORT=8451 ;;
  v51) PORT=8511 ;;
  v52) PORT=8521 ;;
  *) echo "不認得的版本：$VERSION（可用 v45、v51、v52）" >&2; exit 2 ;;
esac
SERVICE="$VERSION-std"
CONTAINER=$(docker compose --project-name moodle-cli-e2e --file "$COMPOSE" ps -q "$SERVICE")
[ -n "$CONTAINER" ] || { echo "$SERVICE 沒有在跑；先執行 make moodle-up V=$VERSION" >&2; exit 2; }
[ -x "$BIN" ] || { echo "找不到 bin/moodle；先執行 make build" >&2; exit 2; }

echo "==> 1/5 佈建十年情境（$SERVICE）"
"$REPO_DIR/test/e2e/seed-masters.sh" "$SERVICE"
"$REPO_DIR/test/e2e/seed-permissions.sh" apply "$SERVICE"
"$REPO_DIR/test/e2e/seed-faculty.sh" "$SERVICE"

echo "==> 2/5 動態建立並匯出 runtime-role fixture"
"$REPO_DIR/test/e2e/role-fixture.sh" "$VERSION"

echo "==> 3/5 由 Moodle 資料庫核對 fixture 真值"
TRUTH=$(E2E_CONTAINER="$CONTAINER" \
  "$REPO_DIR/test/e2e/fixture-truth.sh" history prof1 CS1001- | tr -d '\r')
python3 -c '
import json, sys
d = json.loads(sys.argv[1])
expected = {"courses": 10, "hidden": 8, "assignments": 20, "submissions": 60}
wrong = {k: (d.get(k), v) for k, v in expected.items() if d.get(k) != v}
if wrong:
    raise SystemExit(f"fixture 不完整: {wrong}; actual={d}")
if d.get("span_days", 0) < 3280:
    raise SystemExit(f"時間跨度不足十屆: {d}")
span = d["span_days"]
print(f"  ✓ 10 屆課程、8 門封存、20 份作業、60 筆提交，跨 {span} 天")
' "$TRUTH"

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
export MOODLE_CLI_CONFIG="$WORKDIR/config.yaml"
"$BIN" site add decade "http://127.0.0.1:$PORT" >/dev/null

token_for() {
  curl -fsS "http://127.0.0.1:$PORT/login/token.php" \
    -d "username=$1" -d 'password=Student123!' -d service=moodle_mobile_app \
    | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token", ""))'
}

PROF_TOKEN=$(token_for prof1)
GRAD_TOKEN=$(token_for grad1)
[ -n "$PROF_TOKEN" ] && [ -n "$GRAD_TOKEN" ] || {
  echo "測試帳號拿不到 token" >&2; exit 1;
}

echo "==> 4/5 以 CLI 讀回十年資料"
MOODLE_WS_TOKEN="$PROF_TOKEN" "$BIN" course list --json > "$WORKDIR/prof-courses.json"
MOODLE_WS_TOKEN="$PROF_TOKEN" "$BIN" assignment list --json > "$WORKDIR/prof-assignments.json"
MOODLE_WS_TOKEN="$GRAD_TOKEN" "$BIN" course list --json > "$WORKDIR/grad-courses.json"
MOODLE_WS_TOKEN="$GRAD_TOKEN" "$BIN" assignment list --json > "$WORKDIR/grad-assignments.json"

UG1102=$(python3 -c '
import json, sys
for c in json.load(open(sys.argv[1]))["data"]:
    if c["short_name"] == "UG1102": print(c["id"]); break
' "$WORKDIR/grad-courses.json")
UG4201=$(python3 -c '
import json, sys
for c in json.load(open(sys.argv[1]))["data"]:
    if c["short_name"] == "UG4201": print(c["id"]); break
' "$WORKDIR/grad-courses.json")
[ -n "$UG1102" ] && [ -n "$UG4201" ] || { echo "學習歷程課程不完整" >&2; exit 1; }
MOODLE_WS_TOKEN="$GRAD_TOKEN" "$BIN" grade list --course "$UG1102" --json \
  > "$WORKDIR/zero-grade.json"
MOODLE_WS_TOKEN="$GRAD_TOKEN" "$BIN" grade list --course "$UG4201" --json \
  > "$WORKDIR/ungraded.json"

python3 - "$WORKDIR" <<'PY'
import json
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
load = lambda name: json.loads((root / name).read_text())["data"]

courses = load("prof-courses.json")
history = [c for c in courses if re.fullmatch(r"CS1001-\d{4}", c["short_name"])]
if len(history) != 10:
    raise SystemExit(f"CLI 只讀到 {len(history)}/10 屆課程")
if len({c["id"] for c in history}) != 10 or len({c["short_name"] for c in history}) != 10:
    raise SystemExit("CLI 把同名課程合併或重複了")
if sum(not c["visible"] for c in history) != 8:
    raise SystemExit("CLI 回報的封存／可見狀態與 fixture 不同")

history_ids = {c["id"] for c in history}
assignments = [a for a in load("prof-assignments.json") if a["course_id"] in history_ids]
if len(assignments) != 20:
    raise SystemExit(f"CLI 只讀到長期課程的 {len(assignments)}/20 份作業")

grad_assignments = load("grad-assignments.json")
by_name = {a["name"]: a for a in grad_assignments}
if not by_name.get("Ethics Declaration", {}).get("requires_statement"):
    raise SystemExit("需同意提交聲明的情境遺失")
if by_name.get("Chapter 1 Draft", {}).get("needs_hand_in") is not True:
    raise SystemExit("草稿／正式交件情境遺失")

zero_items = load("zero-grade.json")["items"]
quiz = [g for g in zero_items if g["name"] == "Academic Integrity Quiz"]
if len(quiz) != 1 or quiz[0]["grade"] != 0:
    raise SystemExit("零分被誤當成未評分")

ungraded_items = load("ungraded.json")["items"]
report = [g for g in ungraded_items if g["name"] == "Final Report"]
if len(report) != 1 or report[0]["grade"] is not None:
    raise SystemExit("未評分狀態沒有保留為 null")

print("  ✓ CLI 保留 10 門同名課程與 20 份作業的獨立身分")
print("  ✓ 封存/現行、草稿/交件、提交聲明、零分/未評分語意正確")
PY

echo "==> 5/5 核對時間視窗與停權歷程"
CAL_TRUTH=$(E2E_CONTAINER="$CONTAINER" \
  "$REPO_DIR/test/e2e/fixture-truth.sh" calendar prof1 30 | tr -d '\r')
CAL_CLI=$(MOODLE_WS_TOKEN="$PROF_TOKEN" "$BIN" calendar upcoming --json \
  | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["data"]))')
[ "$CAL_TRUTH" = "$CAL_CLI" ] || {
  echo "行事曆筆數不一致：Moodle=$CAL_TRUTH CLI=$CAL_CLI" >&2; exit 1;
}
SUSPENDED=$(E2E_CONTAINER="$CONTAINER" \
  "$REPO_DIR/test/e2e/fixture-truth.sh" suspended grad1 | tr -d '\r')
[ "${SUSPENDED:-0}" -ge 1 ] || { echo "停權選課歷程遺失" >&2; exit 1; }
printf '  ✓ 30 天行事曆 %s 筆與 Moodle 真值一致\n' "$CAL_CLI"
printf '  ✓ 保留 %s 筆停權選課，沒有誤當成「從未修課」\n' "$SUSPENDED"
echo "十年情境驗收通過（$VERSION）"
