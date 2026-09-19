#!/usr/bin/env bash
# 找出「Moodle 可能不送、我們卻用值接住」的欄位。
#
# 這個專案已經在同一個陷阱上絆倒過四次：一個 Go 的 bool 或 int，零值剛好是一個
# 有意義的狀態，於是「站台沒說」被印成「站台說沒有」。手動稽核每個欄位不可行，
# 但 Moodle 自己的 returns 宣告就說了哪些欄位可以缺席——VALUE_OPTIONAL。
#
#   scripts/audit-optional-fields.sh            # 預設對 v52-std 容器
#   scripts/audit-optional-fields.sh <容器名>
#
# 列出來的不一定都是缺陷：只有「零值本身是一個宣稱」的才是。日期已經由
# unixTime() 統一轉成 nil，字串與切片的零值不構成宣稱，所以都排除掉。
set -uo pipefail

CONTAINER="${1:-moodle-cli-e2e-v52-std-1}"
SRC=/var/www/html/public
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$REPO_DIR"

docker inspect "$CONTAINER" >/dev/null 2>&1 || {
  echo "容器 $CONTAINER 沒在跑；先 make moodle-up V=v52" >&2; exit 2; }

ours=$(mktemp); optional=$(mktemp)
trap 'rm -f "$ours" "$optional"' EXIT

# 我們讀的每個 json 鍵與它的 Go 型別。
tick=$(printf '\140')
# 型別可能長成 *T、[]T 或 map[K]V，所以字元類直接含 [ 與 ]。
grep -hoE "[][A-Za-z0-9_.*]+ +${tick}json:\"[a-z_]+\"" internal/moodle/*.go \
  | awk -F"${tick}json:\"" '{gsub(/[[:space:]]+$/,"",$1); print substr($2,1,length($2)-1) "\t" $1}' \
  | sort -u > "$ours"

# Moodle 宣告為 VALUE_OPTIONAL 的鍵。
docker exec "$CONTAINER" sh -c "grep -rhoE \"'[a-z_]+' *=> *new external_value\\([^)]*VALUE_OPTIONAL\" \
  $SRC/mod/assign/externallib.php $SRC/enrol/externallib.php \
  $SRC/mod/forum/externallib.php $SRC/course/externallib.php \
  $SRC/calendar/externallib.php \
  $SRC/grade/report/user/classes/external/user.php 2>/dev/null" \
  | sed -E "s/^'([a-z_]+)'.*/\1/" | sort -u > "$optional"

printf '讀 %s 個鍵；Moodle 宣告 %s 個可缺席\n\n' \
  "$(wc -l < "$ours")" "$(wc -l < "$optional")"
echo '可能缺席、而且用「值」接住的：'
found=0
while IFS=$'\t' read -r key gotype; do
  grep -qx "$key" "$optional" || continue
  case "$gotype" in
    # 指標已經表達得出缺席；切片、字串、原始 JSON 的零值不構成宣稱。
    \**|*'[]'*|string|json.RawMessage|any) continue ;;
    # 日期一律走 unixTime()，零會變成 nil。
    int64) case "$key" in *date|*time*|created|modified) continue ;; esac ;;
  esac
  printf '  %-30s %s\n' "$key" "$gotype"
  found=$((found + 1))
done < "$ours"
[ "$found" = 0 ] && echo '  （沒有）'
echo
echo '逐一判斷：零值會不會被印成一句話？會的話就要改成指標，而且要一路到契約。'
