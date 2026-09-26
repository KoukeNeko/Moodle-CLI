# 自動化範例

[English](Automation-Recipes) · [首頁](Home-zh-TW)

用鍵盤以外的方式操作 Moodle 的實際範例。每個範例都依賴 [JSON 契約](JSON-Contract-zh-TW)：`--json`
提供穩定的格式，`--fields` 只取會讀的欄位，`--no-input` 讓程式不會停下來等提示，exit code 用來判斷
發生了什麼。只讀取的操作請加上 `--read-only`（或設定 `MOODLE_CLI_READ_ONLY=1`），這樣即使出錯也碰不到
站台。

## 1. 這學期的截止日，依時間排序

```sh
moodle assignment list --current --json --fields name,course_short_name,due_date --no-input --read-only |
  jq -r 'if (.meta.missing | index("due_date")) then error("this route cannot see due dates") else . end
         | .data[] | select(.due_date != null)
         | [.due_date, .course_short_name, .name] | @tsv' |
  sort
```

有兩個細節值得照抄。`--current` 只保留已開始且尚未結束的課程，往年的作業不會混進來。檢查
`meta.missing` 則是讓 `select(.due_date != null)` 安全的關鍵：在讀不到截止日的路線上，`null` 代表
「不知道」，直接濾掉會悄悄漏掉還沒繳的作業。當 `due_date` 不在 `meta.missing` 裡時，`null` 才真的是
沒有截止日的作業。

日期是 UTC 的 RFC 3339 格式；顯示時再轉換，比較時不要轉。

## 2. 依 exit code 分支，不要比對訊息

```sh
if out=$(moodle grade overview --json --no-input --read-only); then
  printf '%s\n' "$out" | jq -r '.data[] | "\(.course_id)\t\(.display)"'
else
  case $? in
    4) echo "signed out: sign in again" >&2 ;;
    5) echo "this account may not read grades" >&2 ;;
    10) echo "network trouble; safe to retry a read" >&2 ;;
    *) printf '%s\n' "$out" | jq -r '.error.message' >&2 ;;
  esac
  exit 1
fi
```

使用 `--json` 時錯誤 envelope 也輸出到 stdout，一個串流就能拿到所有結果。完整對照表見
[JSON 契約](JSON-Contract-zh-TW#exit-code)。對任何會寫入的程式來說最重要的是 `12`：寫入可能已經
發生，要先回讀物件再決定。

## 3. 讓 agent 先查、再演練、最後執行

Agent 不該用猜的判斷一個命令能不能無人看管地執行。直接問 binary：

```sh
moodle schema assignment submit --json --no-input | jq '.data | {safety, idempotency}'
```

```json
{"safety": "write", "idempotency": "non_idempotent"}
```

`read` 命令可以自由執行，`local` 只改變本機，`write` 需要做決定。`non_idempotent` 代表失敗後不能直接
重跑。`moodle commands --json` 一次提供所有命令的這兩個欄位；即使在 `--read-only` 下（會隱藏被擋下的
寫入命令），`schema` 仍然會回答。

繳交前先演練。`--dry-run` 會讀取作業與你目前的繳交狀態，列出實際執行會走的步驟（包括這份作業是否
需要另外按「提交」），但不會送出任何東西。繳交需要 web service token；browser session 可以讀作業，
但不能繳交。

```sh
moodle assignment submit 1436182 hw1.tar.gz --dry-run --json --no-input
```

接著執行；若結果未確認，就回讀狀態：

```sh
if moodle assignment submit 1436182 hw1.tar.gz --yes --json --no-input > result.json; then
  jq '.data' result.json
else
  case $? in
    12) moodle assignment status 1436182 --json --no-input ;;  # 絕不盲目重送
    *)  jq -r '.error.message' result.json >&2; exit 1 ;;
  esac
fi
```

`assignment submit` 回報的是 Moodle 事後的狀態，而不是它送了什麼：仍是草稿的繳交會被如實回報為草稿。

## 4. 給 agent 的規則

把以下內容放進 agent 開始工作前會讀的地方，例如 `AGENTS.md`、`CLAUDE.md` 或 system prompt：

```text
When using the moodle CLI:
- Always pass --json --no-input. Add --read-only unless the task is to change something.
- Use --fields for only the fields you read; an unknown name is refused with the real ones.
- Before running a command you have not used, run `moodle schema <command> --json`.
  Run safety "read" freely; ask before safety "write".
- Before reading a null as "none", check meta.missing: a field listed there was not readable.
- Branch on exit codes and error.code, never on message text. Never retry exit 12:
  read the object back instead.
```

支援 Model Context Protocol 的 agent 也可以改用 `moodle mcp serve`，直接取得結構化結果，不需要經過
shell。
