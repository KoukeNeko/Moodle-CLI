#!/usr/bin/env bash
# 交作業流程的真站驗收。
#
#   make moodle-up V=v52
#   test/e2e/accept-assignment.sh 8521 "$(docker compose --project-name moodle-cli-e2e \
#       --file test/e2e/docker-compose.yml ps -q v52-std)"
#
# 單元測試用假站，只能照這個專案「想像中的 Moodle」回答。這支腳本問的是真的
# Moodle：三種作業設定各自的結果，是不是真的跟我們回報的一樣。
set -euo pipefail
PORT="${1:?usage: accept-assignment.sh <port> <container>}"
CONTAINER="${2:?usage: accept-assignment.sh <port> <container>}"

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
BIN="$REPO_DIR/bin/moodle"
[ -x "$BIN" ] || { echo "先 make build" >&2; exit 2; }

# 交件不可逆，所以每次跑都用一個全新的學生帳號。
USER="acc$(date +%s)"

# 建帳號與選課都用 Moodle 自己的 API。moosh 在 SQLite 上會因為 mdl_sessions.sid
# 衝突而半途失敗，而且做完事情也可能回非零；seed.php 已經為了同樣的理由不用它。
docker exec -w /var/www/html "$CONTAINER" php -r '
define("CLI_SCRIPT", true);
require("/var/www/html/config.php");
require_once($CFG->dirroot . "/user/lib.php");
require_once($CFG->dirroot . "/enrol/manual/lib.php");
$username = $argv[1];
$user = (object) [
    "username" => $username, "auth" => "manual", "confirmed" => 1,
    "mnethostid" => $CFG->mnet_localhost_id, "email" => "$username@example.com",
    "firstname" => "Acc", "lastname" => "Test", "password" => "Student123!",
];
$user->id = user_create_user($user, true, false);
$course = $DB->get_record("course", ["shortname" => "CS204"], "*", MUST_EXIST);
$manual = $DB->get_record("enrol",
    ["courseid" => $course->id, "enrol" => "manual"], "*", MUST_EXIST);
$role = $DB->get_record("role", ["shortname" => "student"], "*", MUST_EXIST);
// 起始日往前挪：Moodle 只認已經開始的選課。
enrol_get_plugin("manual")->enrol_user($manual, $user->id, $role->id, time() - DAYSECS);
' "$USER" >/dev/null 2>&1 || { echo "無法建立測試學生 $USER" >&2; exit 1; }

MOODLE_CLI_CONFIG=$(mktemp -d)/config.yaml
export MOODLE_CLI_CONFIG
MOODLE_WS_TOKEN=$(curl -fsS "http://127.0.0.1:$PORT/login/token.php" \
  -d "username=$USER" -d 'password=Student123!' -d service=moodle_mobile_app \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')
export MOODLE_WS_TOKEN
"$BIN" site add acc "http://127.0.0.1:$PORT" >/dev/null

WORK=$(mktemp -d)/report.pdf
printf '%%PDF-1.4 acceptance\n' > "$WORK"

fail=0
check() { # check <說明> <預期> <實際>
  if [ "$2" = "$3" ]; then echo "  ok   $1 ($3)"; else echo "  FAIL $1: got $3, want $2"; fail=1; fi
}
field() { "$BIN" assignment status "$1" --json \
  | python3 -c "import json,sys;print(str(json.load(sys.stdin)['data']['$2']).lower())"; }

echo "== A1 submissiondrafts=0：存檔就等於交出去 =="
"$BIN" assignment submit 1 "$WORK" --dry-run >/dev/null
check "dry run 什麼都沒送" "new" "$(field 1 status)"
"$BIN" assignment submit 1 "$WORK" --yes >/dev/null
check "最終狀態" "submitted" "$(field 1 status)"
check "已交件" "true" "$(field 1 handed_in)"

echo "== A2 submissiondrafts=1：存檔不等於交出去 =="
"$BIN" assignment submit 2 "$WORK" --draft --yes >/dev/null
check "草稿還是草稿" "draft" "$(field 2 status)"
check "不會被說成已交件" "false" "$(field 2 handed_in)"
"$BIN" assignment submit 2 "$WORK" --yes >/dev/null
check "正式交出後" "submitted" "$(field 2 status)"

echo "== A3 requiresubmissionstatement=1 =="
set +e
"$BIN" assignment submit 3 "$WORK" --yes >/dev/null 2>&1; code=$?
set -e
check "沒有 --accept-statement 就拒絕" "2" "$code"
check "而且什麼都沒送" "new" "$(field 3 status)"
"$BIN" assignment submit 3 "$WORK" --yes --accept-statement >/dev/null
check "同意聲明後交件" "submitted" "$(field 3 status)"

echo "== 重複交件是衝突，不是再交一次 =="
set +e
"$BIN" assignment submit 2 "$WORK" --yes >/dev/null 2>&1; code=$?
set -e
check "結束碼" "8" "$code"

[ $fail -eq 0 ] && echo "全部通過（$USER）"
exit $fail
