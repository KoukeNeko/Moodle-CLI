#!/usr/bin/env bash
# 語意斷言：問「我們最不能說錯的事情，有沒有說錯？」
#
# 跟 compare-run.sh 分工不同。那一支問的是「今天跟昨天長得一樣嗎」，答得出回歸，
# 但答不出「這句話本來就是錯的」——一個從第一天就說謊的指令，每天都一樣。
#
# 這裡的斷言直接讀 JSON 契約。研究的說法是 primary oracle 看語意、
# 人類輸出只當 tripwire：文案改寫時 tripwire 會失效，而語意斷言不會。
#
# 每一條都寫成「情境 → 這個情境下絕不能發生什麼」，而不是「輸出應該長怎樣」。
set -uo pipefail

ASSERT_PASS=0
ASSERT_FAIL=0
ASSERT_LOG=""

# assert_case <名稱> <期望的 kind> <期望的 error code 或 -> <命令...>
#
# kind 是契約的 kind 欄位（error 或某個資料種類）。error code 傳 - 表示不檢查。
assert_case() {
  local name="$1" want_kind="$2" want_code="$3"; shift 3
  local out kind code
  out=$("$BIN" "$@" --json 2>&1)
  kind=$(printf '%s' "$out" | python3 -c 'import json,sys
try: print(json.load(sys.stdin).get("kind",""))
except Exception: print("<not json>")' 2>/dev/null)
  code=$(printf '%s' "$out" | python3 -c 'import json,sys
try: print((json.load(sys.stdin).get("error") or {}).get("code","-"))
except Exception: print("-")' 2>/dev/null)

  if [ "$kind" != "$want_kind" ]; then
    assert_failed "$name" "kind=$kind，期望 $want_kind" "$out"
    return
  fi
  if [ "$want_code" != "-" ] && [ "$code" != "$want_code" ]; then
    assert_failed "$name" "code=$code，期望 $want_code" "$out"
    return
  fi
  assert_passed "$name"
}

# assert_not_claiming <名稱> <不得出現的字串> <命令...>
#
# Tripwire，不是 oracle：它防的是呈現層哪天手滑，而語意由 assert_case 決定。
# 文案改寫會讓這一條失效，那是刻意的——它本來就只是第二道。
assert_not_claiming() {
  local name="$1" forbidden="$2"; shift 2
  local out
  out=$("$BIN" "$@" 2>&1)
  if printf '%s' "$out" | grep -qF "$forbidden"; then
    assert_failed "$name" "輸出宣稱了「$forbidden」" "$out"
    return
  fi
  assert_passed "$name"
}

# assert_names_its_set <名稱> <命令...>
#
# 通則，不是個案：**任何**空的清單都必須講出它問的是哪一個集合。
#
# 前面那些 assert_not_claiming 是一句一句擋的，每發現一種新的說謊方式就要再加
# 一條。這一條問的是不同的問題——「這個答案有沒有主詞」——所以一個還沒有人想過
# 的新命令、或一個被改壞的舊句子，都會在這裡被攔下來，不必先有人受害。
#
# 判準刻意寬鬆：只要出現任何一個「界定範圍」的詞就算。要的是讓「No forums.」
# 這種光禿禿的斷言過不了關，不是規定文案怎麼寫。
assert_names_its_set() {
  local name="$1"; shift
  local out
  out=$("$BIN" "$@" 2>&1)
  # 有資料的就不是這一條要管的
  if printf '%s' "$out" | head -1 | grep -qE '^(ID|WHEN|COURSE|ITEM|METHOD)'; then
    assert_passed "$name（有資料，不適用）"
    return
  fi
  if printf '%s' "$out" | grep -qE 'this account|enrolled on|visible|report covers|period asked for|grade report'; then
    assert_passed "$name"
    return
  fi
  assert_failed "$name" "空的答案沒有說出它問的是哪一個集合" "$out"
}

assert_passed() {
  ASSERT_PASS=$((ASSERT_PASS + 1))
  printf '  ✓ %s\n' "$1" | tee -a "$TRANSCRIPT"
}

assert_failed() {
  ASSERT_FAIL=$((ASSERT_FAIL + 1))
  ASSERT_LOG="${ASSERT_LOG}  ✗ $1：$2"$'\n'
  {
    printf '  ✗ %s：%s\n' "$1" "$2"
    printf '%s\n' "$3" | sed 's/^/      /'
  } | tee -a "$TRANSCRIPT"
}

assert_summary() {
  printf '\n語意斷言：%d 通過' "$ASSERT_PASS"
  [ "$ASSERT_FAIL" -gt 0 ] && printf '，%d 失敗' "$ASSERT_FAIL"
  printf '\n'
  [ "$ASSERT_FAIL" -gt 0 ] && printf '%s' "$ASSERT_LOG"
  return 0
}
