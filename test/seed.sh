#!/usr/bin/env bash
# 對一個已就緒的測試 Moodle 容器佈建內容與設定。
#
#   ./seed.sh <container> [std|nows]
#
# 內容用 moosh 建立（課程、帳號、選課、三個作業），站台設定與作業參數
# 由 seed.php 用 Moodle API 設定（見該檔說明，有幾個 CLI 做不到的地方）。
#
# moosh 會把除錯 backtrace 印到 stderr（它強制開發者除錯模式），那是雜訊不是錯誤，
# 所以這裡把 stderr 導到 log，並以「實際查詢結果」作為成功與否的判準。
set -euo pipefail

CONTAINER="${1:?usage: seed.sh <container> [std|nows]}"
MODE="${2:-std}"
LOG="/tmp/moodle-seed-${CONTAINER}.log"
: > "$LOG"

# 每次 moosh 前先清 session：SQLite 上 moosh 反覆啟動會撞 mdl_sessions.sid 的唯一鍵
# （Moodle 4.5 實測會因此讓 course-enrol / activity-add 整個失敗），清掉就正常。
m() {
  docker exec -w /var/www/html "$CONTAINER" php admin/cli/kill_all_sessions.php >>"$LOG" 2>&1 || true
  docker exec -w /var/www/html "$CONTAINER" moosh "$@" 2>>"$LOG"
}

echo "==> [$CONTAINER] 建立課程與帳號"
# 課程放在預設分類 (id 1)。-r/--reuse 讓重複執行不會建出第二門課。
m course-create --category 1 --fullname "Operating Systems" --reuse CS204 >>"$LOG" || true
m user-create --password 'Teacher123!' --email teacher1@example.com \
   --firstname Tammy --lastname Teacher teacher1 >>"$LOG" || true
m user-create --password 'Student123!' --email student1@example.com \
   --firstname Sam --lastname Student student1 >>"$LOG" || true

echo "==> [$CONTAINER] 選課"
m course-enrol -s -r editingteacher CS204 teacher1 >>"$LOG" || true
m course-enrol -s -r student CS204 student1 >>"$LOG" || true

echo "==> [$CONTAINER] 建立三個作業"
CID=$(docker exec -w /var/www/html "$CONTAINER" \
        php -r 'define("CLI_SCRIPT",true);require("/var/www/html/config.php");
                echo $DB->get_field("course","id",["shortname"=>"CS204"]);' 2>>"$LOG")
for a in "A1 direct submit" "A2 submit button" "A3 statement"; do
  m activity-add -n "$a" -o "--assignsubmission_file_enabled 1 --assignsubmission_file_maxfiles 1" \
     assign "$CID" >>"$LOG" || true
done

echo "==> [$CONTAINER] 站台設定（$MODE）與作業參數"
docker cp "$(dirname "$0")/seed.php" "$CONTAINER:/seed.php" >/dev/null
docker exec "$CONTAINER" php /seed.php "$MODE" 2>>"$LOG" | grep -v '^$'

echo "==> [$CONTAINER] 驗證"
# 以實際查詢結果把關，不信任 moosh 的結束碼：它在 SQLite session 衝突時會無聲失敗，
# 只留下一門空課程（Moodle 4.5 實測）。數量不對就讓整個腳本失敗。
RESULT=$(docker exec -w /var/www/html "$CONTAINER" php -r '
define("CLI_SCRIPT",true);require("/var/www/html/config.php");
$c=$DB->get_record("course",["shortname"=>"CS204"]);
if (!$c) { echo "course=MISSING enrolled=0 assignments=0\n"; exit; }
$n=$DB->count_records_sql("SELECT COUNT(*) FROM {user_enrolments} ue
     JOIN {enrol} e ON e.id=ue.enrolid WHERE e.courseid=?",[$c->id]);
$a=$DB->count_records("assign",["course"=>$c->id]);
echo "course=$c->shortname enrolled=$n assignments=$a\n";' 2>>"$LOG")
echo "    $RESULT"

if [ "$RESULT" != "course=CS204 enrolled=2 assignments=3" ]; then
  echo "ERROR: 佈建不完整（預期 enrolled=2 assignments=3）。詳見 $LOG" >&2
  exit 1
fi

echo "==> 完成（moosh 詳細輸出：$LOG）"
