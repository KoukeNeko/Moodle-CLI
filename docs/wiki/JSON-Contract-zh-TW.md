# JSON contract

[English](JSON-Contract) · [首頁](Home-zh-TW)

輸出給程式使用時請明確加上 `--json`；pipe 不會偷偷改變格式。

```sh
moodle version --json
moodle course list --json
moodle assignment list --current --json --fields name,due_date --no-input
moodle commands --json
moodle schema assignment.submit
moodle schema assignment submit --json
```

無人看管的呼叫者（script、CI、agent）請同時使用 `--json` 與 `--no-input`，並用 `--fields` 只取需要的
欄位。完整範例見[自動化範例](Automation-Recipes-zh-TW)。

## Envelope

成功回應含穩定的 top-level `schema_version`、`kind`、`data` 與 `meta`：

```json
{
  "schema_version": 1,
  "kind": "version",
  "data": {},
  "meta": {"source": "local"}
}
```

錯誤使用 `kind: "error"`，並提供 `code`、`reason`、`outcome`、`retryable` 等結構化欄位。Consumer
必須依這些欄位分支，不可比對英文 `message`。`reason` 刻意設計成 open set，必須容忍未來新增的值。

## 部分回答

`meta.source` 說明由哪條路線回答：`ws`（web service token）、`ajax`（browser session）或 `html`
（站台自己的頁面）。三者看得到的資料不同。路線讀不到某個欄位時，`meta.partial` 為 `true`，
`meta.missing` 列出該欄位名稱，欄位值為 `null`。若某個 `null` 的欄位名稱**不在** `meta.missing`
裡，它就是答案本身，例如沒有截止日的作業、沒有關閉時間的測驗。把 `null` 解讀成「沒有」之前，
先檢查 `meta.missing`。

## 欄位選擇

`--fields` 只保留 `data` 中指定的頂層欄位：清單的每一列，或單一物件。只能與 `--json` 一起使用。
Envelope 與 `meta` 保持完整。回應沒有的欄位名稱會以 exit 2 拒絕，錯誤的 `hint` 會列出可用欄位；
這份清單來自已發布的 schema，所以即使清單是空的也能抓到拼錯的名稱。

```sh
moodle quiz list --current --json --fields name,closes_at
```

縮減後的回應只是投影，不再包含該 kind 的 schema 要求的所有欄位。

## 絕不等待輸入

`--no-input` 保證不會停下來詢問。原本會提示輸入的地方（貼上 callback、隱藏輸入 session cookie、
密碼、繳交前確認）改為以 exit 2 失敗，`hint` 會指出該用哪個旗標提供答案（`--callback`、`--stdin`、
`--password-stdin`、`--yes`）。沒有終端機時這些提示本來就會拒絕；`--no-input` 讓接著終端機時也是如此。

## Exit code

| Exit | Error code | 意義 |
| ---: | --- | --- |
| 0 | — | 成功 |
| 1 | `internal` | 本機 invariant 或非預期內部錯誤 |
| 2 | `usage` | 命令或參數錯誤 |
| 3 | `configuration` | 本機設定缺少或無效 |
| 4 | `authentication` | 憑證失效或被拒絕 |
| 5 | `permission_denied` | 已登入但沒有權限 |
| 6 | `not_found` | 找不到指定資源 |
| 7 | `validation` | 輸入或回傳狀態驗證失敗 |
| 8 | `conflict` | Server state 與操作衝突 |
| 9 | `unavailable` | Capability 或本機設施不可用 |
| 10 | `network` | Transport failure |
| 11 | `upstream` | Moodle 回傳失敗或無效內容 |
| 12 | 任意 code、`outcome: ambiguous` | 寫入可能已經發生 |

Ambiguous outcome 優先於底層錯誤類別。不要自動重試 exit 12；應回讀受影響的 Moodle object，再依目前
狀態決定下一步。

## Schema 與命令探索

`moodle schema` 列出內嵌 response schema；`moodle schema <kind>` 印出單一 schema。
`moodle schema <命令>`（例如 `moodle schema assignment submit --json`）描述單一命令：

```json
{
  "schema_version": 1,
  "kind": "command.schema",
  "data": {
    "command": "assignment submit",
    "kind": "assignment.submit",
    "safety": "write",
    "idempotency": "non_idempotent",
    "input_schema": {"...": "依名稱列出旗標，位置參數為 args"},
    "output_schema": {"...": "assignment.submit 的 schema"}
  },
  "meta": {"source": "local"}
}
```

`safety` 是執行後可能改變什麼：`read` 不改變任何東西；`local` 只改變本機（設定、keychain、下載的
檔案）；`write` 可能改變 Moodle 上的東西。`idempotency` 表示失敗或結果未確認時能否直接重跑；所有
write 都是 `non_idempotent`，因為 Moodle function 沒有 idempotency key。Agent 可以自由執行 `read`，
斟酌 `local`，`write` 則需要做決定。

`moodle commands --json` 一次提供每個命令的 `safety` 與 `idempotency`，以及 `mutates`（`--read-only`
是否會擋下該命令）。Shell 與 agent 直接以 binary 本身作為 discovery source，不必解析 `--help`。

單一個字若是已發布的 kind，會被當成 kind。與 kind 同名的單字命令（`version`、`doctor`、`resolve`、
`commands`）都是 `read`，可在 `moodle commands --json` 中查到。

## 全域旗標

| 旗標 | 用途 |
| --- | --- |
| `--json` | 輸出版本化 JSON contract |
| `--fields` | 只保留這些 data 欄位（需搭配 `--json`） |
| `--no-input` | 絕不提示；需要回答時直接失敗 |
| `--pretty` | JSON 縮排 |
| `--read-only` | 拒絕所有可能改變站台的呼叫；也可用 `MOODLE_CLI_READ_ONLY` |
| `--backend` | 允許回答的路線：`auto` 或 `ws-only` |

改變 contract v1 的含意或形狀必須建立新的 schema version。人類訊息與 diagnostic detail 不是穩定 API。
