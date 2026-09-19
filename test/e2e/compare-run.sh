#!/usr/bin/env bash
# 比對兩次 full-run.sh 的結果，把「回歸」跟「只是字句變了」分開。
#
#   test/e2e/compare-run.sh                      # 最新的兩次
#   test/e2e/compare-run.sh <新> <基準>          # 指定 logs/ 底下的目錄
#
# 為什麼要分兩層：
#   - summary.tsv 是「每個命令的結束碼」。它變了就是行為變了，這是硬性失敗。
#     上一次有一輪看起來像整片回歸，實際上是 §10 把可交的作業用掉了，
#     結束碼分布從 0:171 掉到 0:160——正是這一層該擋住的東西。
#   - transcript.log 是逐字輸出。改一句訊息的措辭就會讓它整片不同，那不是回歸，
#     但也不該沒人看到。所以這一層只報告，交給讀的人判斷。
set -uo pipefail

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
LOGS="$REPO_DIR/test/e2e/logs"

resolve() { case "$1" in */*) printf '%s' "$1" ;; *) printf '%s' "$LOGS/$1" ;; esac; }

# 哪一座站台跑的。不同版本的站台之間比對沒有意義——題材、id、甚至功能都不一樣，
# 差異會多到把真正的訊號淹掉。第一次寫這支的時候沒想到，於是拿 v45 的結果去跟
# v52 的比，報出 3794 行「差異」。
site_of() { sed -n 's/^# 站台：//p' "$1/transcript.log" 2>/dev/null | head -1; }

if [ $# -ge 2 ]; then
  NEW=$(resolve "$1"); OLD=$(resolve "$2")
  if [ "$(site_of "$NEW")" != "$(site_of "$OLD")" ]; then
    echo "這兩輪跑的不是同一座站台，比對沒有意義：" >&2
    echo "  ${NEW##*/}: $(site_of "$NEW")" >&2
    echo "  ${OLD##*/}: $(site_of "$OLD")" >&2
    exit 2
  fi
else
  mapfile -t runs < <(ls -1d "$LOGS"/*/ 2>/dev/null | sed 's:/$::' | sort)
  [ "${#runs[@]}" -ge 1 ] || { echo "logs/ 裡沒有紀錄" >&2; exit 2; }
  NEW=${runs[-1]}
  NEWSITE=$(site_of "$NEW")
  OLD=""
  for (( i=${#runs[@]} - 2; i >= 0; i-- )); do
    [ "$(site_of "${runs[$i]}")" = "$NEWSITE" ] || continue
    # An aborted run is not a baseline. full-run.sh stops when it finds no
    # assignment left to submit, so its transcript ends part-way and every
    # command after that point looks like it disappeared. Comparing against
    # one reports a regression that is really just a shorter list.
    grep -q '^共執行' "${runs[$i]}/transcript.log" 2>/dev/null || continue
    OLD=${runs[$i]}
    break
  done
  if [ -z "$OLD" ]; then
    # 全新的 volume 跑第一輪就是這樣：沒有可比的基準不是失敗。
    echo "本次：${NEW##*/}（$NEWSITE）"
    echo "沒有同一座站台的前一輪可以比對——如果這是第一輪，那是預期的。"
    exit 0
  fi
fi
for d in "$OLD" "$NEW"; do
  [ -f "$d/summary.tsv" ] || { echo "$d 裡沒有 summary.tsv" >&2; exit 2; }
done
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "基準：${OLD##*/}"
echo "本次：${NEW##*/}"

# 每一輪本來就會不同的東西先抹掉，否則連續兩次一模一樣的跑測也會整片不同：
# 跑測時間、建置資訊、站台回的時間戳、mktemp 的目錄、一次性的 QR token。
# 暫存目錄與 token 會出現在**命令列本身**，所以結束碼那一層也得先過這一關。
#
# 憑證一律換成同一個 <secret>，不管紀錄裡留的是原值還是 <redacted>：
# full-run.sh 的遮蔽範圍改變時，不該讓每一條命令看起來都變了。
normalise() {
  sed -E \
    -e 's/[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})/<time>/g' \
    -e 's/"commit": *"[0-9a-f]+"/"commit": "<commit>"/g' \
    -e 's/commit [0-9a-f]{7,}/commit <commit>/g' \
    -e 's#/tmp/tmp\.[A-Za-z0-9]+#<tmpdir>#g' \
    -e 's/(qrlogin|passport|token|wstoken|sesskey)=[^ &"]+/\1=<secret>/g' \
    -e 's/(\"(userprivateaccesskey|privatetoken|token|wstoken)\"[[:space:]]*:[[:space:]]*\")[^\"]*/\1<secret>/g' \
    -e 's/(MOODLE_WS_TOKEN|MOODLE_SESSION)=[^ ]+/\1=<secret>/g' \
    -e 's#logs/[0-9]{8}-[0-9]{6}#logs/<run>#g' \
    "$1"
}

# ── 第一層：結束碼 ──────────────────────────────────────────────────────
status=0
if diff -u <(normalise "$OLD/summary.tsv") <(normalise "$NEW/summary.tsv") > "$TMP/summary"; then
  echo "結束碼：$(( $(wc -l < "$NEW/summary.tsv") - 1 )) 個命令，與基準相同"
else
  echo "結束碼：有差異 ← 這是回歸，或是題材被用掉了"
  sed -n '3,$p' "$TMP/summary" | grep '^[-+]' | head -40
  status=1
fi

# ── 第二層：逐字輸出 ────────────────────────────────────────────────────
changed=$(diff <(normalise "$OLD/transcript.log") <(normalise "$NEW/transcript.log") \
  | grep -c '^[<>]' || true)
if [ "$changed" -eq 0 ]; then
  echo "逐字輸出：正規化後完全相同"
else
  echo "逐字輸出：$changed 行不同（措辭改動也會算進來，需要人看過）"
  echo "  diff <(sed ... $OLD/transcript.log) ...   或直接："
  echo "  diff '$OLD/transcript.log' '$NEW/transcript.log' | less"
fi

exit $status
