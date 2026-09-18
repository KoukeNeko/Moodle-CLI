#!/usr/bin/env bash
# 把每一個命令都跑過一遍，記下指令與完整輸出。
#
# 情境是一位四學期的碩士生，後兩學期兼任大學部課程助教——同一個帳號在不同課程
# 有不同角色。單一功能的測試不會碰到那種組合，而那正是會出錯的地方。
#
#   make moodle-up V=v52
#   test/e2e/full-run.sh                 # 標準站
#   test/e2e/full-run.sh --nows          # 也跑沒有 Web Services 的站
#
# QR 登入與 manual 需要 HTTPS 站；偵測到 8443 有站台就會自動多跑一節。
#
# 逐字紀錄寫到 test/e2e/logs/<時間>/transcript.log，每個命令連同結束碼與輸出。
# 這支腳本不判斷對錯，它產生的是可讀的證據；判斷留給讀的人與既有的驗收腳本。
#
# 憑證會經過 redact()，只把 <redacted> 寫進紀錄：這些站台的密碼是公開的，但
# 「紀錄裡不該出現 token」這件事本身不該因為站台是測試站就破例。
set -uo pipefail

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
BIN="$REPO_DIR/bin/moodle"
[ -x "$BIN" ] || { echo "先 make build" >&2; exit 2; }

STD_PORT=${STD_PORT:-8521}
NOWS_PORT=${NOWS_PORT:-8522}
TLS_PORT=${TLS_PORT:-8443}
WITH_NOWS=0
[ "${1:-}" = "--nows" ] && WITH_NOWS=1

CA="$REPO_DIR/test/e2e/tls/ca.crt"
KEYRING_BUS="$REPO_DIR/test/e2e/run/bus"

STAMP=$(date +%Y%m%d-%H%M%S)
LOGDIR="$REPO_DIR/test/e2e/logs/$STAMP"
mkdir -p "$LOGDIR"
TRANSCRIPT="$LOGDIR/transcript.log"
SUMMARY="$LOGDIR/summary.tsv"
printf 'exit\tcommand\n' > "$SUMMARY"

WORKDIR=$(mktemp -d)
export MOODLE_CLI_CONFIG="$WORKDIR/config.yaml"
trap 'rm -rf "$WORKDIR"' EXIT

# 站台回傳的清單存下來重複查，免得同樣的請求打好幾次。
ASSIGNMENTS_JSON="$WORKDIR/assignments.json"

total=0
declare -A exits

# ── 憑證去識別 ────────────────────────────────────────────────────────────
# 紀錄要能被人讀，所以指令要照登，但值不能。SECRETS 收集每一個用過的憑證，
# redact() 把出現在文字裡的那幾個換掉。
SECRETS=()
redact() {
  local text="$1" s
  for s in ${SECRETS+"${SECRETS[@]}"}; do
    [ -n "$s" ] && text="${text//$s/<redacted>}"
  done
  printf '%s' "$text"
}

# ── 紀錄 ──────────────────────────────────────────────────────────────────
# say / note 讓紀錄讀起來像一份敘述，而不是一堆輸出。
say() { printf '\n\n═══ %s ═══\n' "$1" | tee -a "$TRANSCRIPT"; }
note() { printf '  # %s\n' "$1" | tee -a "$TRANSCRIPT"; }

# record 收尾：把輸出縮排寫進紀錄、記下結束碼。
record() { # record <exit> <command-line> <output>
  local status="$1" cmdline="$2" out="$3"
  [ -n "$out" ] && printf '%s\n' "$out" | sed 's/^/    /' | tee -a "$TRANSCRIPT"
  printf '  [exit %d]\n' "$status" | tee -a "$TRANSCRIPT"
  printf '%d\t%s\n' "$status" "$cmdline" >> "$SUMMARY"
  exits[$status]=$(( ${exits[$status]:-0} + 1 ))
  total=$((total + 1))
}

# run 執行一個命令，逐字記下它與輸出。
run() {
  local cmdline
  cmdline=$(redact "moodle $*")
  printf '\n$ %s\n' "$cmdline" | tee -a "$TRANSCRIPT"
  local out status
  out=$("$BIN" "$@" 2>&1); status=$?
  record "$status" "$cmdline" "$(redact "$out")"
  return 0
}

# runenv 一樣，但多帶環境變數（憑證的單次覆寫）。
runenv() {
  local env_desc="$1"; shift
  local cmdline
  cmdline=$(redact "$env_desc moodle $*")
  printf '\n$ %s\n' "$cmdline" | tee -a "$TRANSCRIPT"
  local out status
  out=$(env $env_desc "$BIN" "$@" 2>&1); status=$?
  record "$status" "$cmdline" "$(redact "$out")"
  return 0
}

# runin 從 stdin 餵一個值進去，用於會讀祕密的參數。stdin 的內容不記錄。
runin() {
  local input="$1"; shift
  local cmdline
  cmdline=$(redact "moodle $*")" <password>"
  printf '\n$ %s\n' "$cmdline" | tee -a "$TRANSCRIPT"
  local out status
  out=$(printf '%s\n' "$input" | "$BIN" "$@" 2>&1); status=$?
  record "$status" "$cmdline" "$(redact "$out")"
  return 0
}

# runfrom 從檔案餵 stdin，用於需要一整串輸入的命令（例如 MCP 的 JSON-RPC）。
runfrom() {
  local path="$1" label="$2"; shift 2
  local cmdline
  cmdline=$(redact "moodle $*")" < $label"
  printf '\n$ %s\n' "$cmdline" | tee -a "$TRANSCRIPT"
  local out status
  out=$("$BIN" "$@" < "$path" 2>&1); status=$?
  record "$status" "$cmdline" "$(redact "$out")"
  return 0
}

# runsum 跑一個輸出太長、只記摘要的命令。
#
# 摘要歸摘要，結束碼仍是命令自己的：早先這裡直接接條管線到 python，
# 結束碼被管線吃掉，命令算進總數卻不在結束碼分布裡——兩個數字對不起來，
# 而對不起來的那幾個正好是沒有人看過的。
runsum() { # runsum <摘要的 python> <命令...>
  local summarise="$1"; shift
  local cmdline
  cmdline=$(redact "moodle $*")" | (摘要)"
  printf '\n$ %s\n' "$cmdline" | tee -a "$TRANSCRIPT"
  local out status summary
  out=$("$BIN" "$@" 2>&1); status=$?
  if [ "$status" -eq 0 ]; then
    summary=$(printf '%s' "$out" | python3 -c "$summarise" 2>&1)
  else
    # 摘要腳本假設輸入是完整的 JSON；失敗時讓命令自己說話，不要蓋上
    # 一層 python 的 traceback。
    summary="$out"
  fi
  record "$status" "$cmdline" "$(redact "$summary")"
  return 0
}

# ── 查 id ─────────────────────────────────────────────────────────────────
# 種子資料每重建一次，課程與作業的 id 就全部換一輪。寫死 id 的腳本會在重建之後
# 安靜地打空氣，所以一律查出來用。
id_of_course() { # id_of_course <shortname>
  "$BIN" course list --json 2>/dev/null | python3 -c '
import json, sys
want = sys.argv[1]
for c in json.load(sys.stdin)["data"]:
    if c["short_name"] == want:
        print(c["id"]); break
' "$1"
}

# pick_submittable 挑一份「自己是學生、還沒交、而且不需要提交聲明」的作業。
#
# 只是「不需要聲明」還不夠：列表第一筆是助教課那份，grad1 在那裡沒有繳交權，
# 整節會全部失敗，而且失敗得很有道理——那正是這個情境要凸顯的東西。
pick_submittable() {
  local id status
  for id in $(python3 -c '
import json, sys
ta = sys.argv[1]
for a in json.load(open(sys.argv[2]))["data"]:
    if a["course_id"] == ta or a["requires_statement"]:
        continue
    print(a["id"])
' "$CSTA" "$ASSIGNMENTS_JSON"); do
    status=$("$BIN" assignment status "$id" --json 2>/dev/null \
      | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["status"])' 2>/dev/null)
    if [ "$status" = "new" ]; then printf '%s' "$id"; return; fi
  done
}

# ── 站台準備 ──────────────────────────────────────────────────────────────
# keychain：headless 主機沒有 Org.freedesktop.secrets，auth login 會停在儲存
# 那一步。test docker 裡的 keyring 容器把 session bus 開在共用目錄上。
HAVE_KEYRING=0
if [ -S "$KEYRING_BUS" ]; then
  export DBUS_SESSION_BUS_ADDRESS="unix:path=$KEYRING_BUS"
  HAVE_KEYRING=1
fi

# HTTPS 站有起來就一起測：QR 登入與 manual 在 http 上會被站台以 httpsrequired 拒絕。
HAVE_TLS=0
if [ -f "$CA" ] && curl -fsS --cacert "$CA" -o /dev/null \
     "https://localhost:$TLS_PORT/login/index.php" 2>/dev/null; then
  HAVE_TLS=1
  export SSL_CERT_FILE="$CA"
fi

{
  printf '# moodle-cli 全功能逐字紀錄\n'
  printf '# 時間：%s\n' "$(date -Is)"
  printf '# 版本：%s\n' "$("$BIN" version --json 2>/dev/null | head -c 200)"
  printf '# 情境：四學期碩士生 grad1，後兩學期兼任 CS1001 助教\n'
  printf '# 站台：http://127.0.0.1:%s' "$STD_PORT"
  [ $HAVE_TLS = 1 ] && printf '、https://localhost:%s' "$TLS_PORT"
  [ $WITH_NOWS = 1 ] && printf '、http://127.0.0.1:%s（無 Web Services）' "$NOWS_PORT"
  printf '\n'
  printf '# keychain：%s\n' "$([ $HAVE_KEYRING = 1 ] && echo "有（test docker 的 keyring 容器）" || echo "沒有，憑證改用環境變數")"
  printf '\n'
} > "$TRANSCRIPT"

# ─────────────────────────────────────────────────────────────────────────
say "1. 不需要站台的命令"
note "先跑不碰網路的：版本、命令自述、schema、shell 補全"
run version
run version --json --pretty
run commands
run commands --json
run schema course.list
run schema assignment.submit
run schema api.call
for shell in bash zsh fish powershell; do
  # 補全腳本本身有上萬字元，只留開頭幾行；命令列照實記成帶管線的那一行，
  # 因為那才是真的被執行的東西。
  cmdline="moodle completion $shell | head -3"
  printf '\n$ %s\n' "$cmdline" | tee -a "$TRANSCRIPT"
  out=$("$BIN" completion "$shell" 2>&1 | head -3); status=$?
  record "$status" "$cmdline" "$out"
done

say "2. 讀 Moodle 連結（純解析，不連線）"
note "學生手上通常是從瀏覽器複製的網址，不是 id"
note "這一節只做解析，所以網址裡的 id 不需要真的存在"
run resolve "http://localhost:$STD_PORT/mod/assign/view.php?id=12"
run resolve "http://localhost:$STD_PORT/course/view.php?id=8"
run resolve "http://localhost:$STD_PORT/mod/forum/discuss.php?d=2"
run resolve "http://localhost:$STD_PORT/grade/report/user/index.php?id=8"
run resolve "http://localhost:$STD_PORT/calendar/view.php?view=month"
run resolve "http://localhost:$STD_PORT/webservice/pluginfile.php/1/mod_assign/introattachment/0/rubric.txt"
run resolve "https://moodle.example.edu/badges/mybadges.php"
note "不是網址也不是 id 的東西應該被擋下"
run resolve "file:///etc/passwd"

say "3. 設定站台"
run site add std "http://127.0.0.1:$STD_PORT"
run site list
run site use std
note "移除站台要明講，不能默默動別人的設定"
run site remove std --yes
run site list
run site add std "http://127.0.0.1:$STD_PORT"
run site use std

say "4. 登入"
note "auth methods 先說這座站台與這台機器各支援什麼"
run auth methods
note "還沒登入"
run auth status

GRAD_TOKEN=$(curl -fsS "http://127.0.0.1:$STD_PORT/login/token.php" \
  -d username=grad1 -d 'password=Student123!' -d service=moodle_mobile_app \
  | python3 -c 'import json,sys;print(json.load(sys.stdin).get("token",""))' 2>/dev/null)
if [ -z "$GRAD_TOKEN" ]; then
  echo "無法取得 grad1 的 token，先跑 seed-masters.sh" >&2
  exit 1
fi
SECRETS+=("$GRAD_TOKEN")

if [ $HAVE_KEYRING = 1 ]; then
  note "憑證只進 OS keychain，設定檔永遠不存憑證——所以需要一台 keychain。"
  note "這台是 test docker 裡的 keyring 容器，以空密碼解鎖的拋棄式金鑰圈。"
  run auth login --method token --token "$GRAD_TOKEN" --username grad1
  run auth status
  run auth status --json
  note "已經在 keychain 裡了，所以後面的命令都不必再帶任何環境變數"
  run course list

  note "登出只清掉本機的憑證，不撤銷 Moodle 那邊的 token"
  run auth logout
  run auth status

  note "改用帳號密碼；密碼從 stdin 進，不經過命令列"
  runin 'Student123!' auth login --method password --username grad1 --password-stdin
  run auth status
  run auth logout

  note "改用瀏覽器已經登入的 session"
  JAR="$WORKDIR/jar.txt"
  LOGINTOKEN=$(curl -fsS -c "$JAR" "http://127.0.0.1:$STD_PORT/login/index.php" \
    | grep -o 'name="logintoken" value="[^"]*"' | sed 's/.*value="\([^"]*\)".*/\1/')
  curl -fsS -b "$JAR" -c "$JAR" -o /dev/null \
    -d "username=grad1&password=Student123!&logintoken=$LOGINTOKEN" \
    "http://127.0.0.1:$STD_PORT/login/index.php" 2>/dev/null
  MOODLESESSION=$(grep -i moodlesession "$JAR" | awk '{print $7}')
  SECRETS+=("$MOODLESESSION")
  run auth login --method browser-session --session-cookie "MoodleSession=$MOODLESESSION"
  run auth status
  run auth logout

  note "壞掉的 session 要被拒絕，不能拿到半個身分"
  run auth login --method browser-session --session-cookie "MoodleSession=deadbeef"

  note "最後用 token 登入一次，後面全部用 keychain 裡的身分"
  run auth login --method token --token "$GRAD_TOKEN" --username grad1
else
  note "這台機器沒有 keychain。憑證只進 keychain 是硬規則，所以 auth login 到此為止——"
  note "它必須說清楚並拒絕，不能退而求其次寫進設定檔。"
  run auth login --method token --token "$GRAD_TOKEN" --username grad1
  note "單次執行用環境變數帶憑證，CI 也是這樣用"
  export MOODLE_WS_TOKEN="$GRAD_TOKEN"
fi

note "單次執行可以覆寫已存放的憑證，不必先登出"
runenv "MOODLE_WS_TOKEN=$GRAD_TOKEN" auth status

say "5. 這個站能做什麼"
run doctor
run doctor --json
run site inspect
note "函式清單很長，只看數量與前幾個"
runsum "
import json,sys
d=json.load(sys.stdin)['data']
print(f\"    函式 {d['function_count']} 個，前 5：{d['functions'][:5]}\")
" site inspect --functions --json

say "6. 四學期的課程"
run course list
run course list --json
note "分頁：每次兩門"
run course list --limit 2
CURSOR=$("$BIN" course list --limit 2 --json 2>/dev/null \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["meta"]["next_cursor"] or "")' 2>/dev/null)
[ -n "$CURSOR" ] && run course list --limit 2 --cursor "$CURSOR"

"$BIN" course list --json > "$WORKDIR/courses.json" 2>/dev/null
CSSEM=$(id_of_course CS5006)   # 進行中的第四學期
CSTA=$(id_of_course CS1001)    # grad1 在這裡是助教，不是學生
note "進行中的學期是 CS5006（id $CSSEM）；grad1 在 CS1001（id $CSTA）是助教"

say "7. 作業：跨四個學期"
run assignment list
run assignment list --json
note "只看進行中那一學期"
run assignment list --course "$CSSEM"
note "助教課那份也在列表裡——他看得到，但那不是他的作業"

"$BIN" assignment list --json > "$ASSIGNMENTS_JSON" 2>/dev/null

say "8. 每一份作業的狀態"
note "涵蓋已評分、已交未評、草稿、未交、逾期；助教課那份不是自己的"
for id in $(python3 -c '
import json, sys
for a in json.load(open(sys.argv[1]))["data"]:
    print(a["id"])
' "$ASSIGNMENTS_JSON"); do
  run assignment status "$id"
done

say "9. 單一作業的細節"
# 助教課那份不能挑：grad1 在 CS1001 不是學生，沒有提交摘要可看，連帶讓下一行
# 的網址少了 cmid。挑一份自己在上面是學生的。
FIRST_ASSIGN=$(python3 -c '
import json, sys
for a in json.load(open(sys.argv[1]))["data"]:
    if a["course_id"] != sys.argv[2]:
        print(a["id"]); break
' "$ASSIGNMENTS_JSON" "$CSTA")
run assignment show "$FIRST_ASSIGN"
run assignment show "$FIRST_ASSIGN" --json
note "也吃從瀏覽器複製的網址"
FIRST_CMID=$("$BIN" assignment show "$FIRST_ASSIGN" --json 2>/dev/null \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["cmid"])' 2>/dev/null)
run assignment show "http://localhost:$STD_PORT/mod/assign/view.php?id=$FIRST_CMID"
note "指向別的東西的網址要說清楚，不能默默接受"
run assignment status "http://localhost:$STD_PORT/mod/forum/view.php?id=1"
run assignment status 999999

say "10. 交作業"
note "先找一份還沒交、而且不需要提交聲明的"
TARGET=$(pick_submittable)
if [ -z "$TARGET" ]; then
  # 這一節會把挑中的作業交出去，所以上一次的執行會把它用掉。沒有把關的話，
  # $TARGET 是空的，後面十幾個命令會變成 `assignment submit  <檔名>`——紀錄看起來
  # 像整片回歸，其實只是待交的作業沒了。跑之前先補回來。
  echo "找不到還沒交的作業，先跑 test/e2e/seed-masters.sh" >&2
  exit 2
fi
note "挑中的是 $TARGET"
WORK="$WORKDIR/thesis-chapter.pdf"
printf '%%PDF-1.4 four semesters of work\n' > "$WORK"

note "dry run：什麼都不送"
run assignment submit "$TARGET" "$WORK" --dry-run
run assignment submit "$TARGET" "$WORK" --dry-run --json
note "沒有 --yes 就不該動手"
run assignment submit "$TARGET" "$WORK"
note "只存草稿"
run assignment submit "$TARGET" "$WORK" --draft --yes
run assignment status "$TARGET"
note "正式交出去"
run assignment submit "$TARGET" "$WORK" --yes
run assignment status "$TARGET"
note "已經交了就不該再交一次"
run assignment submit "$TARGET" "$WORK" --yes

note "需要提交聲明的那一份：不加旗標必須被擋"
STMT=$(python3 -c '
import json, sys
for a in json.load(open(sys.argv[1]))["data"]:
    if a["requires_statement"]:
        print(a["id"]); break
' "$ASSIGNMENTS_JSON")
if [ -n "$STMT" ]; then
  run assignment submit "$STMT" "$WORK" --yes
  run assignment submit "$STMT" "$WORK" --dry-run
  run assignment submit "$STMT" "$WORK" --yes --accept-statement
  run assignment status "$STMT"
fi

note "助教在自己課的作業上沒有繳交權，站台會擋下來"
run assignment submit "$FIRST_ASSIGN" "$WORK" --yes

say "11. 成績"
run grade overview
run grade overview --json
note "每一門課的成績簿"
for cid in $(python3 -c '
import json, sys
for c in json.load(open(sys.argv[1]))["data"]:
    print(c["id"])
' "$WORKDIR/courses.json"); do
  run grade list --course "$cid"
done
run grade list --course "$CSSEM" --json
note "沒給課號要說清楚要怎麼做"
run grade list

say "12. 待辦與截止日"
run calendar upcoming
run calendar upcoming --json
run calendar upcoming --days 7
run calendar upcoming --days 400
run calendar upcoming --limit 3

say "13. 討論區"
run forum list
run forum list --json
run forum list --course "$CSSEM"
for fid in $("$BIN" forum list --json 2>/dev/null \
    | python3 -c 'import json,sys;[print(f["id"]) for f in json.load(sys.stdin)["data"]]' 2>/dev/null | head -4); do
  run forum discussions "$fid"
done
FIRST_DISC=$("$BIN" forum list --json 2>/dev/null | python3 -c '
import json,sys
# 第一個論壇是公告區，沒有討論串；要挑有討論串的才讀得到東西。
for f in json.load(sys.stdin)["data"]:
    if f["discussions"]:
        print(f["id"]); break' 2>/dev/null)
DISC=$("$BIN" forum discussions "$FIRST_DISC" --json 2>/dev/null | python3 -c '
import json,sys
d=json.load(sys.stdin)["data"]
print(d[0]["id"] if d else "")' 2>/dev/null)
if [ -n "$DISC" ]; then
  run forum read "$DISC"
  run forum read "$DISC" --json
  note "討論串的網址也吃"
  run forum read "http://localhost:$STD_PORT/mod/forum/discuss.php?d=$DISC"
else
  note "找不到任何討論串，跳過 forum read"
fi

say "14. 下載檔案"
note "先從作業裡找一個檔案連結"
DOWNLOAD=""
for id in $(python3 -c '
import json, sys
for a in json.load(open(sys.argv[1]))["data"]:
    print(a["id"])
' "$ASSIGNMENTS_JSON"); do
  url=$("$BIN" assignment status "$id" --json 2>/dev/null | python3 -c '
import json,sys
f=json.load(sys.stdin)["data"]["files"]
print(f[0]["url"] if f else "")' 2>/dev/null)
  [ -n "$url" ] && { DOWNLOAD="$url"; break; }
done
if [ -n "$DOWNLOAD" ]; then
  run file download "$DOWNLOAD" --dir "$WORKDIR"
  note "同一個檔案再下載一次，預設不覆蓋"
  run file download "$DOWNLOAD" --dir "$WORKDIR"
  run file download "$DOWNLOAD" --dir "$WORKDIR" --force
  run file download "$DOWNLOAD" --dir "$WORKDIR" --as renamed.pdf
  note "別的主機：憑證在網址裡，所以送錯地方等於把 token 給對方"
  run file download "https://evil.example.com/webservice/pluginfile.php/1/x/y/z.pdf"
  note "不存在的檔案"
  run file download "http://127.0.0.1:$STD_PORT/webservice/pluginfile.php/1/mod_assign/introattachment/0/nope.txt"
  printf '\n  # 下載後目錄內容：\n' | tee -a "$TRANSCRIPT"
  ls -1 "$WORKDIR" | sed 's/^/    /' | tee -a "$TRANSCRIPT"
fi

say "15. 逃生口：直接呼叫 web service"
run api functions --match core_webservice
run api functions --match mod_assign
run api functions --writes --match mod_forum
run api functions --unreviewed --match gradereport
run api call core_webservice_get_site_info
run api call core_enrol_get_users_courses --param userid=0
run api call core_course_get_contents --params-json "{\"courseid\": $CSSEM}"
note "沒審查過的函式一律當成會寫入，要明講才放行"
run api call mod_assign_get_submissions --param "assignmentids[0]=$TARGET"
run api call mod_assign_get_submissions --param "assignmentids[0]=$TARGET" --dry-run
run api call mod_assign_get_submissions --param "assignmentids[0]=$TARGET" --allow-write
note "站台沒有的函式在送出前就擋下"
run api call core_completely_made_up --allow-write

say "16. 唯讀模式"
note "會寫入的命令應該連出現都不出現"
run commands --read-only
runsum "
import json,sys
d=json.load(sys.stdin)['data']
w=[c['path'] for c in d if c['mutates']]
print(f'    共 {len(d)} 個命令，其中會寫入的：{w or \"（無）\"}')" commands --read-only --json
run assignment submit "$TARGET" "$WORK" --yes --read-only
run api call core_webservice_get_site_info --read-only
note "讀取不受影響"
run course list --read-only
note "環境變數也能開啟"
runenv "MOODLE_CLI_READ_ONLY=1" assignment submit "$TARGET" "$WORK" --yes

say "17. 給 AI agent 的 MCP 介面"
note "預設唯讀：會改變東西的工具根本不註冊"
MCPIN="$WORKDIR/mcp.jsonl"
python3 - "$MCPIN" "$STD_PORT" <<'PYEOF'
import json, sys
with open(sys.argv[1], "w") as out:
    def send(obj): out.write(json.dumps(obj, ensure_ascii=False) + "\n")
    send({"jsonrpc":"2.0","id":1,"method":"initialize",
          "params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"full-run","version":"1"}}})
    send({"jsonrpc":"2.0","method":"notifications/initialized"})
    send({"jsonrpc":"2.0","id":2,"method":"tools/list"})
    for i, (tool, args) in enumerate([
        ("course_list", {}),
        ("assignment_list", {}),
        ("calendar_upcoming", {"days": 30}),
        ("grade_overview", {}),
        ("forum_list", {}),
        ("resolve_url", {"url": f"http://localhost:{sys.argv[2]}/mod/assign/view.php?id=12"}),
        ("assignment_submit", {"assignment": "1", "files": ["/tmp/x"]}),
    ], start=3):
        send({"jsonrpc":"2.0","id":i,"method":"tools/call",
              "params":{"name":tool,"arguments":args}})
PYEOF
runfrom "$MCPIN" "(七個 JSON-RPC 訊息)" mcp serve

note "開放寫入時工具才會出現"
WRITEIN="$WORKDIR/mcp-write.jsonl"
printf '%s\n%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' > "$WRITEIN"
runfrom "$WRITEIN" "(tools/list)" mcp serve --allow-write

say "18. 換一個身分：授課教師看到的東西"
note "同一個站、同一批課，但 prof1 是授課教師。看得到的東西不同。"
PROF_TOKEN=$(curl -fsS "http://127.0.0.1:$STD_PORT/login/token.php" \
  -d username=prof1 -d 'password=Student123!' -d service=moodle_mobile_app \
  | python3 -c 'import json,sys;print(json.load(sys.stdin).get("token",""))' 2>/dev/null)
if [ -n "$PROF_TOKEN" ]; then
  SECRETS+=("$PROF_TOKEN")
  runenv "MOODLE_WS_TOKEN=$PROF_TOKEN" auth status
  runenv "MOODLE_WS_TOKEN=$PROF_TOKEN" course list
  runenv "MOODLE_WS_TOKEN=$PROF_TOKEN" assignment list
  note "教師身分讀自己的繳交狀態：他沒有交過任何東西"
  runenv "MOODLE_WS_TOKEN=$PROF_TOKEN" assignment status "$FIRST_ASSIGN"
  note "教師可以用的函式跟學生不同"
  runenv "MOODLE_WS_TOKEN=$PROF_TOKEN" api call mod_assign_get_submissions --param "assignmentids[0]=$TARGET" --allow-write
fi

say "19. 大學部學生：只有一門課"
UG_TOKEN=$(curl -fsS "http://127.0.0.1:$STD_PORT/login/token.php" \
  -d username=ug1 -d 'password=Student123!' -d service=moodle_mobile_app \
  | python3 -c 'import json,sys;print(json.load(sys.stdin).get("token",""))' 2>/dev/null)
if [ -n "$UG_TOKEN" ]; then
  SECRETS+=("$UG_TOKEN")
  runenv "MOODLE_WS_TOKEN=$UG_TOKEN" course list
  runenv "MOODLE_WS_TOKEN=$UG_TOKEN" assignment list
  runenv "MOODLE_WS_TOKEN=$UG_TOKEN" calendar upcoming
  note "別人的課程成績：站台自己會擋"
  runenv "MOODLE_WS_TOKEN=$UG_TOKEN" grade list --course "$CSSEM"
fi

say "19b. 每個身分組都走一遍"
note "Moodle 有八個標準角色。前面跑的是學生、助教與教師，這裡補上剩下的："
note "零選課的帳號（每個新帳號的起點）、站台 manager 與 coursecreator、站台管理員。"
note "guest 不在這裡：那不是能登入的身分，而是未登入的訪客，見第 20 節的無憑證。"
# 每個帳號都拿一次自己的 token：憑證決定看得到什麼，不是站台決定。
for who in nocourse mgr1 cc1 admin; do
  case $who in
    admin) pw='Admin123!' ;;
    *)     pw='Student123!' ;;
  esac
  TK=$(curl -fsS "http://127.0.0.1:$STD_PORT/login/token.php" \
    -d username="$who" -d "password=$pw" -d service=moodle_mobile_app \
    | python3 -c 'import json,sys;print(json.load(sys.stdin).get("token",""))' 2>/dev/null)
  printf '\n  ── %s ──\n' "$who" | tee -a "$TRANSCRIPT"
  if [ -z "$TK" ]; then
    note "$who 拿不到 token（Moodle 不發給這個身分）"
    continue
  fi
  SECRETS+=("$TK")
  # 零選課的帳號：每一項都該是空的，而且是空陣列不是 null。
  # 有站台角色的帳號：課程清單一樣是空的——manager 管站台，不修課。
  runenv "MOODLE_WS_TOKEN=$TK" course list
  runenv "MOODLE_WS_TOKEN=$TK" course list --json
  runenv "MOODLE_WS_TOKEN=$TK" assignment list
  runenv "MOODLE_WS_TOKEN=$TK" calendar upcoming
  runenv "MOODLE_WS_TOKEN=$TK" grade overview
  runenv "MOODLE_WS_TOKEN=$TK" forum list
  # 讀一門他不在的課：不是空，是拒絕。
  runenv "MOODLE_WS_TOKEN=$TK" grade list --course "$CSSEM"
  runenv "MOODLE_WS_TOKEN=$TK" assignment status "$FIRST_ASSIGN"
done
note "站台管理員不能用 app 的登入流程：Moodle 對 admin 關掉它"

say "20. 憑證出問題時"
note "過期或亂填的 token"
runenv "MOODLE_WS_TOKEN=deadbeefdeadbeefdeadbeefdeadbeef" course list
note "沒有憑證"
printf '\n$ (無憑證) moodle course list\n' | tee -a "$TRANSCRIPT"
out=$(env -u MOODLE_WS_TOKEN "$BIN" course list 2>&1); status=$?
record "$status" "(無憑證) moodle course list" "$(redact "$out")"
note "指向不是 Moodle 的網址"
run site add bogus "http://127.0.0.1:9"
runenv "MOODLE_WS_TOKEN=$GRAD_TOKEN" doctor --site bogus
run site remove bogus --yes

say "21. 登出與收尾"
note "登出只清掉本機的憑證，不會撤銷 Moodle 那邊的 token——"
note "那個 token 常常也在手機 App 上用著"
run auth logout
run auth status
run site list

# ─────────────────────────────────────────────────────────────────────────
if [ $HAVE_TLS = 1 ]; then
  say "22. HTTPS 站：QR 登入與 manual"
  note "Moodle 對 http 直接回 httpsrequired，所以這兩條路只在 HTTPS 站驗得到"

  TLS_CONTAINER=$(docker compose --project-name moodle-cli-e2e \
    --file "$REPO_DIR/test/e2e/docker-compose.yml" ps -q v52-tls 2>/dev/null)
  QRGEN="$WORKDIR/qrgen.php"
  cat > "$QRGEN" <<'PHP'
<?php
define('CLI_SCRIPT', true);
require('/var/www/html/config.php');
$u = $DB->get_record('user', ['username' => 'student1'], '*', MUST_EXIST);
\core\session\manager::set_user($u);
$key = \tool_mobile\api::get_qrlogin_key(get_config('tool_mobile'));
echo "moodlemobile://{$CFG->wwwroot}?qrlogin={$key}&userid={$u->id}\n";
PHP
  # docker cp 會保留權限，而 mktemp 產出的是 600；容器裡跑的是 nobody，讀不到。
  chmod 644 "$QRGEN"
  docker cp "$QRGEN" "$TLS_CONTAINER:/tmp/qrgen.php" >/dev/null 2>&1
  qr() { docker exec "$TLS_CONTAINER" php /tmp/qrgen.php 2>/dev/null | tail -1; }

  run site add tls "https://localhost:$TLS_PORT"
  run site use tls
  note "同一組方法，但在 HTTPS 站上 qr 從『不支援』變成『可用』"
  run auth methods

  note "QR 的 key 由站台產生，就像使用者在個人頁面看到 QR 圖那樣"
  QRCODE=$(qr)
  printf '\n$ (站台產生 QR 登入碼，內容含單次使用的 key)\n' | tee -a "$TRANSCRIPT"
  [ -n "$QRCODE" ] && printf '    <redacted>\n' | tee -a "$TRANSCRIPT"

  if [ $HAVE_KEYRING = 1 ] && [ -n "$QRCODE" ]; then
    note "QR 是給 SSO 帳號用的：沒有密碼可以送，只能靠瀏覽器已完成的那次登入"
    run auth login --method qr --qr "$QRCODE"
    run auth status
    note "同一個 key 不能再用一次"
    run auth login --method qr --qr "$QRCODE"
    run auth logout
  else
    note "沒有 keychain，跳過會寫入憑證的那一半"
  fi

  note "manual：使用者自己在瀏覽器登入，把瀏覽器試著開啟的網址貼回來"
  note "這裡用 FIFO 模擬那個貼上的動作，passport 由工具自己產生"
  FIFO="$WORKDIR/manual-in"
  mkfifo "$FIFO"
  "$BIN" auth login --method manual < "$FIFO" > "$WORKDIR/manual-out" 2> "$WORKDIR/manual-err" &
  MANUAL_PID=$!
  exec 9> "$FIFO"
  for _ in $(seq 1 60); do
    grep -q 'launch.php' "$WORKDIR/manual-err" 2>/dev/null && break
    sleep 0.2
  done
  LAUNCH=$(grep -o "https\?://[^ ]*launch\.php?[^ ]*" "$WORKDIR/manual-err" 2>/dev/null | head -1)
  note "工具印出來的登入網址（passport 也在裡面，那是這次登入的識別）"
  printf '    %s\n' "$LAUNCH" | tee -a "$TRANSCRIPT"
  if [ -n "$LAUNCH" ]; then
    note "用瀏覽器 session 走完它，Moodle 會轉址到帶 token 的 callback"
    BROWSER_JAR="$WORKDIR/tls-jar.txt"
    LT=$(curl -fsS --cacert "$CA" -c "$BROWSER_JAR" "https://localhost:$TLS_PORT/login/index.php" \
      | grep -o 'name="logintoken" value="[^"]*"' | sed 's/.*value="\([^"]*\)".*/\1/')
    curl -fsS --cacert "$CA" -b "$BROWSER_JAR" -c "$BROWSER_JAR" -o /dev/null \
      -d "username=student1&password=Student123!&logintoken=$LT" \
      "https://localhost:$TLS_PORT/login/index.php" 2>/dev/null
    CALLBACK=$(curl -fsS --cacert "$CA" -b "$BROWSER_JAR" -o /dev/null \
      -w '%{redirect_url}' "$LAUNCH" 2>/dev/null)
    printf '    <callback URL，內含 token>\n' | tee -a "$TRANSCRIPT"
    printf '%s\n' "$CALLBACK" >&9
  else
    printf '\n' >&9
  fi
  exec 9>&-
  wait "$MANUAL_PID" 2>/dev/null
  MANUAL_STATUS=$?
  note "工具那一邊的輸出（stdout 只有結果，提示走 stderr）："
  sed 's/^/    /' "$WORKDIR/manual-out" | tee -a "$TRANSCRIPT"
  sed 's/^/    /' "$WORKDIR/manual-err" | tee -a "$TRANSCRIPT"
  printf '  [exit %d]\n' "$MANUAL_STATUS" | tee -a "$TRANSCRIPT"
  printf '%d\tmoodle auth login --method manual (互動)\n' "$MANUAL_STATUS" >> "$SUMMARY"
  exits[$MANUAL_STATUS]=$(( ${exits[$MANUAL_STATUS]:-0} + 1 )); total=$((total + 1))
  rm -f "$FIFO"

  note "不是這座站台發的 callback 換不到身分"
  run auth login --method manual --callback "moodlemobile://token=bm90LWEtdG9rZW4="

  run site remove tls --yes
fi

# ─────────────────────────────────────────────────────────────────────────
if [ $WITH_NOWS = 1 ]; then
  say "23. 關閉 Web Services 的站"
  note "這種站發不出 token。唯一的路是瀏覽器已經登入的 session。"
  JAR="$WORKDIR/cookies.txt"
  LOGINTOKEN=$(curl -fsS -c "$JAR" "http://127.0.0.1:$NOWS_PORT/login/index.php" \
    | grep -o 'name="logintoken" value="[^"]*"' | sed 's/.*value="\([^"]*\)".*/\1/')
  curl -fsS -b "$JAR" -c "$JAR" -o /dev/null \
    -d "username=student1&password=Student123!&logintoken=$LOGINTOKEN" \
    "http://127.0.0.1:$NOWS_PORT/login/index.php" 2>/dev/null
  SESSION=$(grep -i moodlesession "$JAR" | awk '{print $7}')
  SECRETS+=("$SESSION")

  printf '\n  # 先確認這個站真的發不出 token：\n' | tee -a "$TRANSCRIPT"
  curl -fsS "http://127.0.0.1:$NOWS_PORT/login/token.php" \
    -d username=student1 -d 'password=Student123!' -d service=moodle_mobile_app \
    | sed 's/^/    /' | tee -a "$TRANSCRIPT"
  printf '\n' | tee -a "$TRANSCRIPT"

  run site add nows "http://127.0.0.1:$NOWS_PORT"
  if [ -n "$SESSION" ]; then
    NOWSENV="MOODLE_SESSION=MoodleSession=$SESSION"
    note "auth methods 在這座站台上誠實地說多數方法不可用"
    runenv "$NOWSENV" auth methods --site nows
    note "課程、行事曆、討論串走 AJAX 端點"
    runenv "$NOWSENV" course list --site nows
    runenv "$NOWSENV" course list --site nows --json
    runenv "$NOWSENV" calendar upcoming --site nows
    note "作業與成績在那個端點上不存在，只能讀頁面"
    runenv "$NOWSENV" assignment list --site nows
    runenv "$NOWSENV" assignment list --site nows --json
    NOWS_ASSIGN=$(env $NOWSENV "$BIN" assignment list --site nows --json 2>/dev/null \
      | python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];print(d[0]["id"] if d else "")' 2>/dev/null)
    NOWS_COURSE=$(env $NOWSENV "$BIN" course list --site nows --json 2>/dev/null \
      | python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];print(d[0]["id"] if d else "")' 2>/dev/null)
    [ -n "$NOWS_ASSIGN" ] && runenv "$NOWSENV" assignment status "$NOWS_ASSIGN" --site nows
    [ -n "$NOWS_COURSE" ] && runenv "$NOWSENV" grade list --course "$NOWS_COURSE" --site nows
    NOWS_FORUM=$(env $NOWSENV "$BIN" forum list --site nows --json 2>/dev/null \
      | python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];print(d[0]["id"] if d else "")' 2>/dev/null)
    [ -n "$NOWS_FORUM" ] && runenv "$NOWSENV" forum read "$NOWS_FORUM" --site nows
    note "讀頁面看不到「存檔是否等於交出去」，所以交作業直接拒絕而不是猜"
    [ -n "$NOWS_ASSIGN" ] && runenv "$NOWSENV" assignment submit "$NOWS_ASSIGN" "$WORK" --yes --site nows
    note "有些東西連頁面都沒有"
    runenv "$NOWSENV" forum list --site nows
    runenv "$NOWSENV" grade overview --site nows
    note "壞掉的 session"
    runenv "MOODLE_SESSION=MoodleSession=deadbeef" course list --site nows
  else
    printf '  # 無法取得 session，略過\n' | tee -a "$TRANSCRIPT"
  fi
fi

# ─────────────────────────────────────────────────────────────────────────
say "結果"
{
  printf '共執行 %d 個命令。\n\n' "$total"
  printf '結束碼分布：\n'
  for code in $(printf '%s\n' "${!exits[@]}" | sort -n); do
    case $code in
      0)  meaning="成功" ;;
      2)  meaning="用法錯誤" ;;
      3)  meaning="設定（例如沒有 keychain）" ;;
      4)  meaning="認證" ;;
      5)  meaning="權限不足" ;;
      6)  meaning="找不到" ;;
      7)  meaning="輸入有問題" ;;
      8)  meaning="衝突" ;;
      9)  meaning="站台不支援" ;;
      10) meaning="網路" ;;
      11) meaning="上游錯誤" ;;
      12) meaning="結果不明" ;;
      *)  meaning="" ;;
    esac
    printf '  %2d  %-28s %d 次\n' "$code" "$meaning" "${exits[$code]}"
  done

  # 涵蓋率：把「這個工具自稱有哪些命令」與「這一輪真的跑了哪些」對起來。
  # 只報數量證明不了涵蓋，這一段才是。沒跑到的命令逐一列出來，而不是安靜地
  # 讓數字看起來很漂亮。
  printf '\n命令涵蓋：\n'
  "$BIN" commands --json > "$WORKDIR/all-commands.json" 2>/dev/null
  printf '  %s\n' "$(python3 - "$WORKDIR/all-commands.json" "$SUMMARY" <<'PY'
import json, sys

declared = [c["path"] for c in json.load(open(sys.argv[1]))["data"]]
# 分組（assignment、auth…）只是容器，本身不是可以單獨跑的命令：判準是
# 「有別的命令以它開頭」。剩下的每一個都得在這一輪裡真的被執行過。
groups = {p for p in declared if any(q.startswith(p + " ") for q in declared)}
required = [p for p in declared if p not in groups]

# transcript 記的是命令列本身，所以從那裡還原路徑：拿掉開頭的環境變數、
# 拿掉 "moodle"，取前兩個字。旗標、參數與管線都不影響路徑。
ran = set()
for line in open(sys.argv[2]):
    parts = line.rstrip("\n").split("\t", 1)
    if len(parts) != 2:
        continue
    words = parts[1].split()
    while words and "=" in words[0]:
        words = words[1:]
    if not words or words[0] != "moodle":
        continue
    words = words[1:]
    for take in (2, 1):
        if len(words) >= take:
            ran.add(" ".join(words[:take]))

missing = [p for p in required if p not in ran]
print("%d/%d 個命令跑過" % (len(required) - len(missing), len(required)))
for p in missing:
    print("  未涵蓋：%s" % p)
PY
)"
  printf '\n逐字紀錄：%s\n' "$TRANSCRIPT"
  printf '逐條清單：%s\n' "$SUMMARY"
} | tee -a "$TRANSCRIPT"

# 非零結束碼在這裡多半是預期的（拒絕、找不到、衝突），所以腳本本身不因此失敗。
# 它產生的是證據，判斷留給讀的人。
exit 0
